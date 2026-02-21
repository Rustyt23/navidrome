package metadataenrichment

import "testing"

func TestNormalizeMetadata(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    string
		expected string
	}{
		"lowercases and trims": {
			input:    "  The Song Title  ",
			expected: "the song title",
		},
		"removes feat and punctuation": {
			input:    "Artist feat. Guest, Vol. 2!",
			expected: "artist guest vol 2",
		},
		"collapses spaces": {
			input:    "A   B\t\tC",
			expected: "a b c",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeMetadata(tc.input); got != tc.expected {
				t.Fatalf("normalizeMetadata(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}
