package persistence

import "testing"

func TestNormalizePlaylistSortField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: ""},
		{name: "id", input: "id", expected: ""},
		{name: "simple order", input: "order_title", expected: "ordertitle"},
		{name: "with prefix", input: "playlist_tracks.created_at", expected: "createdat"},
		{name: "prefer sort tags expression", input: "(coalesce(nullif(f.sort_title,''),f.order_title) collate nocase)", expected: "ordertitle"},
		{name: "multiple columns", input: "order_album_name, order_album_artist_name", expected: "orderalbumname"},
		{name: "desc order", input: "f.order_artist_name desc", expected: "orderartistname"},
		{name: "unknown", input: "playlist_tracks.rowid", expected: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizePlaylistSortField(tc.input); got != tc.expected {
				t.Fatalf("normalizePlaylistSortField(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestSanitizePlaylistSortField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "basic", input: "order_title", expected: "ordertitle"},
		{name: "with spaces and punctuation", input: "(coalesce(nullif(f.sort_title,''),f.order_title) collate nocase)", expected: "coalescenulliffsorttitleffordertitlecollatenocase"},
		{name: "empty", input: "", expected: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizePlaylistSortField(tc.input); got != tc.expected {
				t.Fatalf("sanitizePlaylistSortField(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}
