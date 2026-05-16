package version

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		// Strict Passthrough
		{"strict standard", "1.2.3", "1.2.3", false},
		{"strict with build", "1.2.3+build.5", "1.2.3+build.5", false},
		{"strict with prerelease", "1.2.3-beta.1", "1.2.3-beta.1", false},
		{"strict with dotted prerelease", "1.2.3-rc.1", "1.2.3-rc.1", false},
		{"strict with both", "1.2.3-beta.1+build.123", "1.2.3-beta.1+build.123", false},

		// Prefixes & Whitespace
		{"trim spaces", "  1.2.3  ", "1.2.3", false},
		{"lowercase v prefix", "v1.2.3", "1.2.3", false},
		{"uppercase V prefix", "V1.2.3", "1.2.3", false},
		{"trim spaces with v", "  v1.2.3  ", "1.2.3", false},
		{"v with prerelease", "v1.2.3-rc.1", "1.2.3-rc.1", false},
		{"trim tabs and newlines", "\t\n1.2.3\r\n", "1.2.3", false},

		// Short Forms
		{"single segment", "1", "1.0.0", false},
		{"two segments", "1.2", "1.2.0", false},
		{"v single segment", "v1", "1.0.0", false},

		// Leading Zeros in Core Segments
		{"leading zero major", "01.0.0", "1.0.0", false},
		{"leading zero minor", "1.02.0", "1.2.0", false},
		{"leading zero patch", "1.2.03", "1.2.3", false},
		{"leading zeros in all", "01.02.03", "1.2.3", false},
		{"leading zero with suffix", "01.0.01-beta", "1.0.1-beta", false},

		// Alphabetic Suffixes
		{"minor+a", "3.0a", "3.0.0-a", false},
		{"minor+rc", "2.1rc1", "2.1.0-rc1", false},
		{"patch+alpha", "1.0.0beta", "1.0.0-beta", false},
		{"short form alpha", "1beta", "1.0.0-beta", false},
		{"two segments alpha", "1.2beta", "1.2.0-beta", false},
		{"patch+alpha+num", "1.0.0beta1", "1.0.0-beta1", false},
		{"alpha with dots inside", "1.2beta.3", "1.2.0-beta.3", false}, // Ensures regex catches dots
		{"trailing alpha with leading-zero patch", "1.0.00beta", "1.0.0-beta", false},

		// 4+ Dotted Segments Collapsing
		{"four zeros", "1.0.0.0", "1.0.0-0", false},
		{"five segments", "1.2.3.4.5", "1.2.3-4.5", false},
		{"many segments", "1.2.3.4.5.6.7.8.9", "1.2.3-4.5.6.7.8.9", false},
		{"prerelease leading zero stripped", "1.0.0.01", "1.0.0-1", false},
		{"prerelease all zeros becomes zero", "1.0.0.00", "1.0.0-0", false},
		{"four segments where 4th is alpha", "1.2.3.alpha", "1.2.3-alpha", false},
		{"4 segments where 4th is alphanumeric", "1.2.3.4beta", "1.2.3-4beta", false},
		{"prerelease alphanumeric 0beta untouched", "1.0.0.0beta", "1.0.0-0beta", false},
		{"prerelease multiple leading zeros stripped", "1.0.0.005.006", "1.0.0-5.6", false},
		{"prerelease mixed alpha+zero untouched", "1.0.0.alpha.01", "1.0.0-alpha.1", false},
		{"0beta in mid prerelease position", "1.0.0.alpha.0beta", "1.0.0-alpha.0beta", false},

		// Core vs Metadata Split logic
		{"4 segments + minimal build", "1.2.3.4+b", "1.2.3-4+b", false},
		{"hyphen in prerelease kept", "1.2.3-rc-1", "1.2.3-rc-1", false},
		{"4 segments + prerelease only", "1.2.3.4-rc", "1.2.3-4.rc", false},
		{"plus comes before hyphen", "1.0.0+build-1", "1.0.0+build-1", false},
		{"4+ segments with build only", "1.2.3.4+build", "1.2.3-4+build", false},
		{"patch+alpha with build", "1.0.0beta+build.1", "1.0.0-beta+build.1", false},
		{"5 segments + dotted prerelease", "1.2.3.4.5-rc.1", "1.2.3-4.5.rc.1", false},
		{"plus before hyphen in suffix", "1.2.3+build-rc.1", "1.2.3+build-rc.1", false},
		{"4+ segments with existing prerelease", "1.2.3.4-alpha", "1.2.3-4.alpha", false},
		{"4 segments + prerelease + build minimal", "1.2.3.4-rc+b", "1.2.3-4.rc+b", false},
		{"hyphen inside build metadata kept", "1.2.3+build-123", "1.2.3+build-123", false},
		{"v prefix with prerelease and build", "v1.2.3-rc.1+build.1", "1.2.3-rc.1+build.1", false},
		{"4+ segments with prerelease and build", "1.2.3.4-alpha+bld", "1.2.3-4.alpha+bld", false},

		// Complex / Mixed Normalizations
		{"short form alpha+build", "1.2beta+bld", "1.2.0-beta+bld", false},
		{"V with full complex form", "V1.2.3.4-rc.1", "1.2.3-4.rc.1", false},
		{"v prefix, 4 segments, alpha+build", "v01.02.03.04beta+build", "1.2.3-04beta+build", false},
		{"all the weirdness combined", "  v01.02.03.04.05beta+build  ", "1.2.3-4.05beta+build", false},

		// Length Constraints
		{
			name:    "max length exact",
			in:      "1.2.3-alpha.x123456789012345678901234567890123456789012345678901",
			want:    "1.2.3-alpha.x123456789012345678901234567890123456789012345678901",
			wantErr: false,
		},
		{
			name:    "max length exceeded",
			in:      "1.2.3-alpha.01234567890123456789012345678901234567890123456789012",
			want:    "",
			wantErr: true,
		},
		{
			name:    "reducible long string passes",
			in:      "v000000000000000000000000000000000000000000000000000000000001.2.3",
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name:    "expansion causes length failure",
			in:      "1-alpha.0123456789012345678901234567890123456789012345678901",
			want:    "",
			wantErr: true,
		},
		{
			name:    "expansion stays under cap",
			in:      "1-alpha.x12345678901234567890123456789012345678901234567890",
			want:    "1.0.0-alpha.x12345678901234567890123456789012345678901234567890",
			wantErr: false,
		},

		// Errors
		{"only v", "v", "", true},
		{"empty string", "", "", true},
		{"only whitespace", "   ", "", true},
		{"consecutive dots", "1..0", "", true},
		{"empty prerelease", "1.2.3-", "", true},
		{"empty build metadata", "1.2.3+", "", true},
		{"garbage string", "not-a-version", "", true},
		{"wildcards (unsupported)", "1.2.x", "", true},
		{"invalid internal spaces", "1.2. 3", "", true},
		{"numeric prerelease 3-digit leading zero", "1.0.0-001", "", true},
		{"dotted numeric prerelease with leading zero", "1.0.0-1.02", "", true},
		{"multiple build tags", "1.2.3+build1+build2", "", true}, // Semver only allows one '+'
		{
			name:    "explicit invalid prerelease leading zero",
			in:      "1.2.3-01",
			want:    "",
			wantErr: true,
		},

		// Non-ASCII characters
		{"unicode digits rejected", "١.٢.٣", "", true},
		{"non-ASCII in prerelease", "1.2.3-α", "", true},
		{"zero-width space inside", "1.2.3\u200B", "", true},

		// Adversarial / Edge Cases
		{"only v's", "vvv", "", true},
		{"triple v", "vvv1.2.3", "1.2.3", false},
		{"v with no numerics", "vbeta", "", true},
		{"NUL byte rejected", "1.2.3\x00", "", true},
		{"mixed multiple v", "Vv1.2.3", "1.2.3", false},
		{"invalid UTF-8 rejected", "1.2.3\xff", "", true},
		{"double lowercase v", "vv1.2.3", "1.2.3", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Normalize(%q) error = %v, wantErr = %v", tc.in, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
