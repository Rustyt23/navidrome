package nativeapi

import (
	"path/filepath"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("orphaned backup detection", func() {
	const root = "/backups"

	// A backup written under the identity layout is matched by the song id in
	// its name and nothing else, so a song that has been renamed or moved still
	// claims its own stored original.
	Describe("identity-based backups", func() {
		It("recovers the song id from the name", func() {
			id, ok := backupIDFromName(root, filepath.Join(root, "ab", "ab12cd__Song.mp3"), "ab12cd__Song.mp3")
			Expect(ok).To(BeTrue())
			Expect(id).To(Equal("ab12cd"))
		})

		It("still matches after the song has been renamed", func() {
			// The file on disk keeps the name it had when it was stored.
			id, ok := backupIDFromName(root, filepath.Join(root, "ab", "ab12cd__Old Name.mp3"), "ab12cd__Old Name.mp3")
			Expect(ok).To(BeTrue())
			Expect(id).To(Equal("ab12cd"))
		})

		It("is live only while its song is", func() {
			path := filepath.Join(root, "ab", "ab12cd__Song.mp3")
			live := map[string]struct{}{"ab12cd": {}}
			Expect(backupIsLive(root, path, "ab12cd__Song.mp3", live, nil)).To(BeTrue())
			Expect(backupIsLive(root, path, "ab12cd__Song.mp3", map[string]struct{}{}, nil)).To(BeFalse())
		})
	})

	// Names from before the identity layout can contain anything, including a
	// double underscore. Reading one as an id would invent a song that never
	// existed and mark a live backup orphaned.
	Describe("backups written by an earlier version", func() {
		It("does not mistake a double underscore in a title for an id", func() {
			path := filepath.Join(root, "Artist", "Some__Song.mp3")
			_, ok := backupIDFromName(root, path, "Some__Song.mp3")
			Expect(ok).To(BeFalse())
		})

		It("does not accept a name whose shard directory disagrees", func() {
			path := filepath.Join(root, "zz", "ab12cd__Song.mp3")
			_, ok := backupIDFromName(root, path, "ab12cd__Song.mp3")
			Expect(ok).To(BeFalse())
		})

		It("is matched by the library path it mirrors", func() {
			path := filepath.Join(root, "Artist", "Song.mp3")
			legacy := map[string]struct{}{filepath.Clean(path): {}}
			Expect(backupIsLive(root, path, "Song.mp3", nil, legacy)).To(BeTrue())
			Expect(backupIsLive(root, path, "Song.mp3", nil, map[string]struct{}{})).To(BeFalse())
		})
	})
})

var _ = Describe("loudness file-work guard", func() {
	reset := func() {
		selectionLoudnessRunning.Store(false)
		restoreLoudnessRunning.Store(false)
		libraryLoudness.running.Store(false)
	}
	BeforeEach(reset)
	AfterEach(reset)

	// All three rewrite files in the library. They lock per track, so an overlap
	// cannot corrupt one - but the finishing order is undefined, and a restore
	// that lands before an optimisation reaches the same track is undone
	// without a word.
	It("lets one operation through at a time", func() {
		release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
		Expect(busy).To(BeEmpty())
		Expect(release).ToNot(BeNil())

		_, busy = claimLoudnessFileWork(&selectionLoudnessRunning)
		Expect(busy).To(Equal("a restore"))

		release()
		release2, busy := claimLoudnessFileWork(&selectionLoudnessRunning)
		Expect(busy).To(BeEmpty())
		release2()
	})

	It("holds back a restore while songs are being optimised", func() {
		release, busy := claimLoudnessFileWork(&selectionLoudnessRunning)
		Expect(busy).To(BeEmpty())
		defer release()

		_, busy = claimLoudnessFileWork(&restoreLoudnessRunning)
		Expect(busy).To(Equal("an optimisation of selected songs"))
	})

	It("holds back a restore during a whole-library run", func() {
		libraryLoudness.running.Store(true)
		_, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
		Expect(busy).To(Equal("a whole-library LUFS run"))
	})

	It("reports nothing running when nothing is", func() {
		Expect(loudnessFileWorkBusy()).To(BeEmpty())
	})

	It("releases cleanly so the next operation can proceed", func() {
		for range 3 {
			release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
			Expect(busy).To(BeEmpty())
			release()
		}
		var flag atomic.Bool
		release, busy := claimLoudnessFileWork(&flag)
		Expect(busy).To(BeEmpty())
		release()
	})
})
