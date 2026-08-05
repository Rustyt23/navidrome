package silence

import (
	"github.com/navidrome/navidrome/model"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TrackPath", func() {
	// The regression: media_file.path is library-relative, and on a flat
	// library it is only the filename. Passing it to ffmpeg unresolved made
	// every single song fail with "no such file or directory".
	It("resolves a library-relative path against the library", func() {
		mf := &model.MediaFile{
			LibraryPath: "/Users/me/music",
			Path:        "2022 - Elysian Fields.mp3",
		}
		Expect(TrackPath(mf)).To(Equal("/Users/me/music/2022 - Elysian Fields.mp3"))
	})

	It("resolves a nested relative path", func() {
		mf := &model.MediaFile{
			LibraryPath: "/Users/me/music",
			Path:        "Artist/Album/01 - Song.mp3",
		}
		Expect(TrackPath(mf)).To(Equal("/Users/me/music/Artist/Album/01 - Song.mp3"))
	})

	// Joining an already-absolute entry would produce
	// "/music/Users/me/.../song.mp3", which exists nowhere.
	It("leaves an absolute path alone", func() {
		mf := &model.MediaFile{
			LibraryPath: "/Users/me/music",
			Path:        "/elsewhere/song.mp3",
		}
		Expect(TrackPath(mf)).To(Equal("/elsewhere/song.mp3"))
	})

	It("returns nothing when there is no path to resolve", func() {
		Expect(TrackPath(&model.MediaFile{LibraryPath: "/Users/me/music"})).To(BeEmpty())
		Expect(TrackPath(nil)).To(BeEmpty())
	})
})
