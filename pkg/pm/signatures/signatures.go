package signatures

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/pkg/errors"
)

const signingAlgorithm = "ECDSA_SHA_256"

type sig struct {
	R, S *big.Int
}

type keyJson struct {
	Expires string `json:"expires"`
	Type    string `json:"type"`
	KeyID   string `json:"keyid"`
	PubKey  string `json:"pubkey"`
}

type KeysJson []keyJson

// Verify verifies a Base64 encoded ASN.1 DER signature against a message using a PEM encoded Public Key.
func Verify(keys KeysJson, keyId string, signatureBase64 string, originalMessage []byte) error {
	var rawPublicKeyBase64, keyType string
	for _, key := range keys {
		if key.KeyID == keyId {
			keyType = key.Type
			rawPublicKeyBase64 = key.PubKey
			break
		}
	}

	if rawPublicKeyBase64 == "" {
		return fmt.Errorf("public key with KeyID %s not found", keyId)
	}

	if keyType != signingAlgorithm {
		return fmt.Errorf("unsupported signing algorithm: %s", keyType)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(rawPublicKeyBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 public key: %v", err)
	}

	genericPublicKey, err := x509.ParsePKIXPublicKey(keyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse PKIX public key: %v", err)
	}

	publicKey, ok := genericPublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("public key is not of type ECDSA")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 signature: %v", err)
	}

	var sig sig
	if _, err := asn1.Unmarshal(sigBytes, &sig); err != nil {
		return fmt.Errorf("failed to unmarshal ASN.1 signature: %v", err)
	}

	hash := sha256.Sum256(originalMessage)
	valid := ecdsa.Verify(publicKey, hash[:], sig.R, sig.S)
	if !valid {
		return errors.New("signature verification failed: invalid signature")
	}

	return nil
}
