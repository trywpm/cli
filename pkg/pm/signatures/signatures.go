package signatures

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"unicode/utf8"

	"go.wpm.so/cli/pkg/pm/wpmjson/manifest"
)

const (
	maxPayloadBytes  = 4096
	signingAlgorithm = "ECDSA_SHA_256"
)

type ecdsaSignature struct {
	R, S *big.Int
}

type key struct {
	Expires string `json:"expires"`
	Type    string `json:"type"`
	KeyID   string `json:"keyid"`
	PubKey  string `json:"pubkey"`
}

// Keys is the set of trusted public keys from keys.json.
type Keys []key

type Verifier struct {
	keys map[string]*ecdsa.PublicKey
}

// New returns a Verifier backed by keys.
func New(keys Keys) *Verifier {
	parsed := make(map[string]*ecdsa.PublicKey, len(keys))
	for _, k := range keys {
		if k.Type != signingAlgorithm {
			continue
		}
		pub, err := parseECDSAKey(k.PubKey)
		if err != nil {
			continue
		}
		parsed[k.KeyID] = pub
	}
	return &Verifier{keys: parsed}
}

// Verify checks the manifest's signature against the trusted keys.
func (v *Verifier) Verify(m *manifest.Package) error {
	sigs := m.Dist.Signatures
	if len(sigs) == 0 {
		return errors.New("no signatures found")
	}

	pub, ok := v.keys[sigs[0].KeyID]
	if !ok {
		return fmt.Errorf("no trusted key for KeyID %s", sigs[0].KeyID)
	}

	var deps map[string]string
	if m.Dependencies != nil {
		deps = *m.Dependencies
	}

	msg, err := payload(m.Name, m.Version, m.Dist.Digest, deps)
	if err != nil {
		return err
	}

	return verifyECDSA(pub, sigs[0].Sig, msg)
}

func parseECDSAKey(pubKeyBase64 string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return nil, err
	}

	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}

	ec, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ECDSA")
	}
	return ec, nil
}

func verifyECDSA(pub *ecdsa.PublicKey, signatureBase64 string, msg []byte) error {
	sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 signature: %w", err)
	}

	var s ecdsaSignature
	if _, err := asn1.Unmarshal(sigBytes, &s); err != nil {
		return fmt.Errorf("failed to unmarshal ASN.1 signature: %w", err)
	}

	hash := sha256.Sum256(msg)
	if !ecdsa.Verify(pub, hash[:], s.R, s.S) {
		return errors.New("signature verification failed: invalid signature")
	}
	return nil
}

// payload builds the message the registry signs:
//
//	name:version:digest                 (no dependencies)
//	name:version:digest:deps_digest     (with dependencies)
//
// deps_digest is base64(sha256(canonicalDependencies(deps))). digest is used
// verbatim, including its "sha256:" prefix.
func payload(name, version, digest string, deps map[string]string) ([]byte, error) {
	msg := name + ":" + version + ":" + digest
	if len(deps) > 0 {
		sum := sha256.Sum256(canonicalDependencies(deps))
		msg += ":" + base64.StdEncoding.EncodeToString(sum[:])
	}

	if len(msg) >= maxPayloadBytes {
		return nil, fmt.Errorf("signature payload exceeds %d bytes", maxPayloadBytes)
	}

	return []byte(msg), nil
}

// canonicalDependencies serializes deps into the canonical form used in the
// signing payload: sorted keys, no spaces, JSON-escaped. For the package-name
// and semver charset this equals RFC 8785 (JCS).
func canonicalDependencies(deps map[string]string) []byte {
	keys := make([]string, 0, len(deps))
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b := make([]byte, 0, 2+len(keys)*16)
	b = append(b, '{')
	for i, k := range keys {
		if i > 0 {
			b = append(b, ',')
		}
		b = appendJSONString(b, k)
		b = append(b, ':')
		b = appendJSONString(b, deps[k])
	}
	return append(b, '}')
}

// appendJSONString writes s as a JSON string with the minimal escaping RFC 8785
// requires. Go's encoding/json also escapes <, >, &, and U+2028/U+2029; those
// must stay literal here to keep the output canonical.
func appendJSONString(dst []byte, s string) []byte {
	const hex = "0123456789abcdef"
	dst = append(dst, '"')
	for _, r := range s {
		switch r {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			if r < 0x20 {
				dst = append(dst, '\\', 'u', '0', '0', hex[r>>4], hex[r&0xf])
			} else {
				dst = utf8.AppendRune(dst, r)
			}
		}
	}
	return append(dst, '"')
}
