package scanner

import "testing"

func TestNormalizeMusicBrainzTerm(t *testing.T) {
	got := normalizeMusicBrainzTerm("  Song Title (Live) feat. Artist! ")
	want := "song title"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestChooseRelease(t *testing.T) {
	a := musicBrainzRelease{}
	a.ReleaseGroup.FirstReleaseDate = "2001-01-01"
	b := musicBrainzRelease{}
	b.ReleaseGroup.FirstReleaseDate = "1998-01-01"

	if !chooseRelease(a, b, 2000) {
		t.Fatal("expected 2001 release to be chosen as closest")
	}
	if chooseRelease(a, b, 0) {
		t.Fatal("expected earliest release when no known year")
	}
}
