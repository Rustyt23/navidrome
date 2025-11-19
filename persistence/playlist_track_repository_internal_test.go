package persistence

import "testing"

func TestQualifyPlaylistTrackSort(t *testing.T) {
	cases := map[string]string{
		"genre":        "f.genre",
		"comment desc": "f.comment desc",
		"created_at":   "f.created_at",
	}

	for input, expected := range cases {
		if got := qualifyPlaylistTrackSort(input); got != expected {
			t.Fatalf("qualifyPlaylistTrackSort(%q) = %q, want %q", input, got, expected)
		}
	}
}
