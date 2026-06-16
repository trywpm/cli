package signatures

import "testing"

func TestCanonicalDependencies(t *testing.T) {
	cases := []struct {
		name string
		deps map[string]string
		want string
	}{
		{"empty", map[string]string{}, "{}"},
		{"single", map[string]string{"elementor": "4.1.0"}, `{"elementor":"4.1.0"}`},
		{"sorted_keys", map[string]string{"elementor": "4.1.0", "akismet": "5.0.0"}, `{"akismet":"5.0.0","elementor":"4.1.0"}`},
		{"ascii_case_order", map[string]string{"b": "1.0.0", "a": "1.0.0", "B": "1.0.0"}, `{"B":"1.0.0","a":"1.0.0","b":"1.0.0"}`},
		// Out of the package-name domain, but pins the escaping/ordering contract:
		{"html_chars_unescaped", map[string]string{"x": "<3&>"}, `{"x":"<3&>"}`},
		{"non_ascii_key_order", map[string]string{"é": "1.0.0", "z": "1.0.0"}, `{"z":"1.0.0","é":"1.0.0"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(canonicalDependencies(tc.deps)); got != tc.want {
				t.Fatalf("canonicalDependencies = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestPayload(t *testing.T) {
	const digest = "sha256:hmBCVLbYU0UkrrgEE3xDhAd9jmVq57tMgICXB0XZrGA="
	const want3 = "elementor:4.1.0:" + digest

	t.Run("nil_deps_is_three_fields", func(t *testing.T) {
		got, err := payload("elementor", "4.1.0", digest, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want3 {
			t.Fatalf("Payload = %q, want %q", got, want3)
		}
	})

	t.Run("empty_deps_is_three_fields", func(t *testing.T) {
		got, err := payload("elementor", "4.1.0", digest, map[string]string{})
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want3 {
			t.Fatalf("Payload = %q, want %q", got, want3)
		}
	})

	t.Run("deps_append_base64_sha256", func(t *testing.T) {
		got, err := payload("elementor", "4.1.0", digest, map[string]string{
			"elementor": "4.1.0", "akismet": "5.0.0",
		})
		if err != nil {
			t.Fatal(err)
		}
		// deps_digest golden value from the registry JS.
		want := want3 + ":F334q11N6Ds7005RwbHApqUBXjUrfjpI4NSo9hOPznQ="
		if string(got) != want {
			t.Fatalf("Payload = %q, want %q", got, want)
		}
	})
}
