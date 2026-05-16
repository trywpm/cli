package version

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		// Already strict.
		{"strict passthrough", "1.2.3", "1.2.3", false},
		{"strict with build", "1.2.3+build.5", "1.2.3+build.5", false},
		{"strict with prerelease", "1.2.3-beta.1", "1.2.3-beta.1", false},

		// v/V prefix.
		{"lowercase v prefix", "v1.2.3", "1.2.3", false},
		{"uppercase V prefix", "V1.2.3", "1.2.3", false},
		{"v prefix with 4 segments", "v1.0.0.01", "1.0.0-1", false},

		// Leading zeros in core.
		{"leading zero major", "01.0.0", "1.0.0", false},
		{"leading zero minor", "1.01.0", "1.1.0", false},
		{"leading zero patch", "1.0.01", "1.0.1", false},
		{"leading zero with suffix", "1.0.01-beta", "1.0.1-beta", false},

		// Short forms.
		{"single segment", "1", "1.0.0", false},
		{"two segments", "1.2", "1.2.0", false},

		// Alphabetic suffix without separator.
		{"patch+alpha", "1.0.0beta", "1.0.0-beta", false},
		{"patch+alpha+num", "1.0.0beta1", "1.0.0-beta1", false},
		{"minor+rc", "2.1rc1", "2.1.0-rc1", false},
		{"minor+a", "3.0a", "3.0.0-a", false},

		// 4+ segments collapsed.
		{"four zeros", "1.0.0.0", "1.0.0-0", false},
		{"five segments", "1.2.3.4.5", "1.2.3-4.5", false},
		{"four segments alpha", "1.0.0.alpha.1", "1.0.0-alpha.1", false},

		// The fix: leading zeros in prerelease.
		{"prerelease leading zero", "1.0.0.01", "1.0.0-1", false},
		{"prerelease all zeros", "1.0.0.00", "1.0.0-0", false},
		{"prerelease multi leading zero", "1.0.0.01.02", "1.0.0-1.2", false},
		{"prerelease mixed alpha+zero", "1.0.0.alpha.01", "1.0.0-alpha.1", false},
		{"prerelease 0beta untouched", "1.0.0.0beta", "1.0.0-0beta", false},

		// Whitespace.
		{"trim spaces", "  1.2.3  ", "1.2.3", false},

		// Errors.
		{"empty", "", "", true},
		{"only v", "v", "", true},
		{"only whitespace", "   ", "", true},
		{"garbage", "not-a-version", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Normalize(%q) error = %v, wantErr = %v", tc.in, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
