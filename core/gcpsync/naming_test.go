package gcpsync

import "testing"

func TestWellFormedName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Artist - Title.mp3", true},
		{"The Beatles - Let It Be.mp3", true},
		// A title containing its own dash splits on the first separator only.
		{"Artist - Some - Title.mp3", true},
		{"ARTIST - TITLE.MP3", true},
		{"Title.mp3", false},
		{"Artist-Title.mp3", false},
		{" - Title.mp3", false},
		{"Artist - .mp3", false},
		{"01 Track.mp3", false},
		// Not a track, so the convention does not apply.
		{"playlist.m3u", true},
		{"cover.jpg", true},
	}
	for _, c := range cases {
		if got := WellFormedName(c.name); got != c.want {
			t.Errorf("WellFormedName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestExpectedNameSanitizesTagValues(t *testing.T) {
	cases := []struct {
		artist, title, want string
	}{
		{"Artist", "Title", "Artist - Title.mp3"},
		// Path separators would create directories, not a filename.
		{"AC/DC", "Back In Black", "AC_DC - Back In Black.mp3"},
		{"Artist", "A: B", "Artist - A_ B.mp3"},
		// Runs of whitespace make two identical-looking names compare unequal.
		{"  The   Band  ", " Song  Name ", "The Band - Song Name.mp3"},
		// Too incomplete to name after.
		{"", "Title", ""},
		{"Artist", "", ""},
		{"   ", "Title", ""},
	}
	for _, c := range cases {
		if got := ExpectedName(c.artist, c.title); got != c.want {
			t.Errorf("ExpectedName(%q, %q) = %q, want %q", c.artist, c.title, got, c.want)
		}
	}
}

func TestConforms(t *testing.T) {
	cases := []struct {
		path, artist, title string
		want                bool
	}{
		{"/music/A/Artist - Title.mp3", "Artist", "Title", true},
		{"/music/A/artist - title.mp3", "Artist", "Title", true},
		{"/music/A/01 Title.mp3", "Artist", "Title", false},
		{"/music/A/Other - Title.mp3", "Artist", "Title", false},
		// A track with no artist tag needs tagging, not renaming, so it is not
		// reported as a naming problem.
		{"/music/A/whatever.mp3", "", "Title", true},
	}
	for _, c := range cases {
		if got := Conforms(c.path, c.artist, c.title); got != c.want {
			t.Errorf("Conforms(%q, %q, %q) = %v, want %v", c.path, c.artist, c.title, got, c.want)
		}
	}
}
