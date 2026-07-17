package persistence

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pocketbase/dbx"
)

var _ = Describe("PlaylistTrackRepository", func() {
	var (
		playlistRepo model.PlaylistRepository
		ctx          context.Context
	)

	BeforeEach(func() {
		ctx = log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid", UserName: "userid", IsAdmin: true})
		playlistRepo = NewPlaylistRepository(ctx, GetDBXBuilder())
	})

	Describe("duplicatesOnly filter", func() {
		var playlist model.Playlist

		BeforeEach(func() {
			playlist = model.Playlist{Name: "Duplicates Filter", OwnerID: "userid", OwnerName: "userid"}
			playlist.AddMediaFilesByID([]string{"1001", "1001", "1003", "1003", "1003"})
			Expect(playlistRepo.Put(&playlist)).To(Succeed())
		})

		AfterEach(func() {
			if playlist.ID != "" {
				Expect(playlistRepo.Delete(playlist.ID)).To(Succeed())
			}
		})

		It("returns one representative for each duplicated track", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"duplicatesOnly": true},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())

			Expect(tracks).To(HaveLen(2))
			Expect(tracks[0].ID).To(Equal("1"))
			Expect(tracks[1].ID).To(Equal("3"))
		})

		It("includes only duplicate missing playlist entries when filtering duplicates", func() {
			playlist.Sync = true
			playlist.Path = filepath.Join(GinkgoT().TempDir(), "duplicates_missing.m3u")
			Expect(os.WriteFile(playlist.Path, []byte("ghost-track.mp3\n"+
				"ghost-track.mp3\n"+
				"phantom.mp3\n"), 0o600)).To(Succeed())
			Expect(playlistRepo.Put(&playlist)).To(Succeed())

			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"duplicatesOnly": true},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())

			Expect(tracks).To(HaveLen(3))
			Expect(tracks).To(ContainElement(HaveField("MediaFileID", "1001")))
			Expect(tracks).To(ContainElement(HaveField("MediaFileID", "1003")))
			Expect(tracks).To(ContainElement(And(
				HaveField("Missing", true),
				HaveField("Path", "ghost-track.mp3"),
			)))
			for _, track := range tracks {
				Expect(track.Path).ToNot(Equal("phantom.mp3"))
			}
		})

		It("excludes duplicated missing entries when missing files are disabled", func() {
			playlist.Sync = true
			playlist.Path = filepath.Join(GinkgoT().TempDir(), "duplicates_without_missing.m3u")
			Expect(os.WriteFile(playlist.Path, []byte("ghost-track.mp3\nghost-track.mp3\n"), 0o600)).To(Succeed())
			Expect(playlistRepo.Put(&playlist)).To(Succeed())

			repo := playlistRepo.Tracks(playlist.ID, true)
			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{
					"duplicatesOnly": true,
					"includeMissing": false,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(2))
			Expect(tracks).To(ConsistOf(
				HaveField("MediaFileID", "1001"),
				HaveField("MediaFileID", "1003"),
			))
		})
	})

	Describe("includeMissing filter", func() {
		var playlist model.Playlist

		BeforeEach(func() {
			playlist = model.Playlist{
				Name:      "Missing Filter",
				OwnerID:   "userid",
				OwnerName: "userid",
				Sync:      true,
				Path:      filepath.Join(GinkgoT().TempDir(), "missing_filter.m3u"),
			}
			playlist.AddMediaFilesByID([]string{songDayInALife.ID})
			contents := songDayInALife.Path + "\nghost-track.mp3\n"
			Expect(os.WriteFile(playlist.Path, []byte(contents), 0o600)).To(Succeed())
			Expect(playlistRepo.Put(&playlist)).To(Succeed())
		})

		AfterEach(func() {
			if playlist.ID != "" {
				Expect(playlistRepo.Delete(playlist.ID)).To(Succeed())
			}
		})

		It("includes missing entries by default", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)
			result, err := repo.ReadAll(rest.QueryOptions{Sort: "title", Order: "ASC"})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(2))
			Expect(tracks[1].Missing).To(BeTrue())
			Expect(tracks[1].Path).To(Equal("ghost-track.mp3"))
		})

		It("hides missing entries when disabled", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)
			result, err := repo.ReadAll(rest.QueryOptions{
				Sort:    "title",
				Order:   "ASC",
				Filters: map[string]interface{}{"includeMissing": false},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(1))
			Expect(tracks[0].Missing).To(BeFalse())
		})
	})

	Describe("search filter", func() {
		var playlist model.Playlist

		BeforeEach(func() {
			playlist = model.Playlist{
				Name:      "Search Filter",
				OwnerID:   "userid",
				OwnerName: "userid",
				Sync:      true,
			}
			playlist.AddMediaFilesByID([]string{songDayInALife.ID})
			playlist.Path = filepath.Join(GinkgoT().TempDir(), "search_filter.m3u")
			Expect(os.WriteFile(playlist.Path, []byte(songDayInALife.Path+"\n"), 0o600)).To(Succeed())
			Expect(playlistRepo.Put(&playlist)).To(Succeed())
		})

		AfterEach(func() {
			if playlist.ID != "" {
				Expect(playlistRepo.Delete(playlist.ID)).To(Succeed())
			}
		})

		It("does not mark synced tracks as missing when filtering by q", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"q": "ru"},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			for _, track := range tracks {
				Expect(track.Missing).To(BeFalse())
			}
		})

		It("finds missing playlist entries by path", func() {
			contents := songDayInALife.Path + "\nghost-track.mp3\n"
			Expect(os.WriteFile(playlist.Path, []byte(contents), 0o600)).To(Succeed())

			repo := playlistRepo.Tracks(playlist.ID, true)
			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"q": "ghost"},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(1))
			Expect(tracks[0].Missing).To(BeTrue())
			Expect(tracks[0].Path).To(Equal("ghost-track.mp3"))
		})

		It("finds tracks when substring matches metadata even if full_text is stale", func() {
			originalID := playlist.ID
			if originalID != "" {
				Expect(playlistRepo.Delete(originalID)).To(Succeed())
			}

			mediaRepo := NewMediaFileRepository(ctx, GetDBXBuilder())
			track := mf(model.MediaFile{
				ID:          "playlist-substring-track",
				Title:       "Bonita Applebum (Sir Piers and Si Ashton's Curious House Mix)",
				ArtistID:    songDayInALife.ArtistID,
				Artist:      "A Tribe Called Quest, Sir Piers",
				AlbumID:     songDayInALife.AlbumID,
				Album:       songDayInALife.Album,
				Path:        p("/playlist/bonita.mp3"),
				AlbumArtist: songDayInALife.AlbumArtist,
			})
			Expect(mediaRepo.Put(&track)).To(Succeed())
			DeferCleanup(func() {
				_ = mediaRepo.Delete(track.ID)
			})

			playlist = model.Playlist{Name: "Substring Fallback", OwnerID: "userid", OwnerName: "userid"}
			playlist.AddMediaFilesByID([]string{track.ID})
			Expect(playlistRepo.Put(&playlist)).To(Succeed())

			_, err := GetDBXBuilder().Update("media_file", dbx.Params{
				"full_text": "a and applebum bonita curious house mix piers quest si sir tribe",
			}, dbx.HashExp{"id": track.ID}).Execute()
			Expect(err).ToNot(HaveOccurred())

			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"q": "to"},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(1))
			Expect(tracks[0].MediaFileID).To(Equal(track.ID))
			Expect(tracks[0].Missing).To(BeFalse())
		})

		It("matches synced entries that only specify filenames", func() {
			originalID := playlist.ID
			if originalID != "" {
				Expect(playlistRepo.Delete(originalID)).To(Succeed())
			}

			playlist = model.Playlist{
				Name:      "Filename Entries",
				OwnerID:   "userid",
				OwnerName: "userid",
				Sync:      true,
			}
			playlist.AddMediaFilesByID([]string{songDayInALife.ID})
			playlist.Path = filepath.Join(GinkgoT().TempDir(), "filename_entries.m3u")

			Expect(os.WriteFile(playlist.Path, []byte(filepath.Base(songDayInALife.Path)+"\n"), 0o600)).To(Succeed())
			Expect(playlistRepo.Put(&playlist)).To(Succeed())

			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(1))
			Expect(tracks[0].MediaFileID).To(Equal(songDayInALife.ID))
			Expect(tracks[0].Missing).To(BeFalse())
		})
	})

	Describe("sorting", func() {
		var playlist model.Playlist

		BeforeEach(func() {
			playlist = model.Playlist{
				Name:      "Sorting Respect",
				OwnerID:   "userid",
				OwnerName: "userid",
				Sync:      true,
			}
			playlist.AddMediaFilesByID([]string{songRadioactivity.ID, songAntenna.ID, songDayInALife.ID})
			playlist.Path = filepath.Join(GinkgoT().TempDir(), "sorting_respect.m3u")
			contents := strings.Join([]string{
				songRadioactivity.Path,
				songAntenna.Path,
				songDayInALife.Path,
			}, "\n")
			Expect(os.WriteFile(playlist.Path, []byte(contents), 0o600)).To(Succeed())
			Expect(playlistRepo.Put(&playlist)).To(Succeed())

			updates := []struct {
				id            string
				title         string
				originalOrder string
			}{
				{songRadioactivity.ID, songRadioactivity.Title, songRadioactivity.OrderTitle},
				{songAntenna.ID, songAntenna.Title, songAntenna.OrderTitle},
				{songDayInALife.ID, songDayInALife.Title, songDayInALife.OrderTitle},
			}
			DeferCleanup(func() {
				for _, upd := range updates {
					_, err := GetDBXBuilder().Update("media_file", dbx.Params{
						"order_title": upd.originalOrder,
					}, dbx.HashExp{"id": upd.id}).Execute()
					Expect(err).ToNot(HaveOccurred())
				}
			})
			for _, upd := range updates {
				_, err := GetDBXBuilder().Update("media_file", dbx.Params{
					"order_title": strings.ToLower(upd.title),
				}, dbx.HashExp{"id": upd.id}).Execute()
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			if playlist.ID != "" {
				Expect(playlistRepo.Delete(playlist.ID)).To(Succeed())
			}
		})

		It("respects explicit sort order for synced playlists", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{Sort: "title", Order: "ASC"})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(3))

			Expect(tracks[0].Title).To(Equal("A Day In A Life"))
			Expect(tracks[1].Title).To(Equal("Antenna"))
			Expect(tracks[2].Title).To(Equal("Radioactivity"))
		})

		It("sorts by createdAt using media file timestamps", func() {
			updates := []struct {
				id        string
				createdAt string
			}{
				{songRadioactivity.ID, "2024-02-01 10:00:00"},
				{songAntenna.ID, "2024-01-01 10:00:00"},
				{songDayInALife.ID, "2024-03-01 10:00:00"},
			}
			for _, upd := range updates {
				_, err := GetDBXBuilder().Update("media_file", dbx.Params{
					"created_at": upd.createdAt,
				}, dbx.HashExp{"id": upd.id}).Execute()
				Expect(err).ToNot(HaveOccurred())
			}

			repo := playlistRepo.Tracks(playlist.ID, true)
			result, err := repo.ReadAll(rest.QueryOptions{Sort: "createdAt", Order: "ASC"})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(3))
			Expect(tracks[0].ID).To(Equal("2"))
			Expect(tracks[1].ID).To(Equal("1"))
			Expect(tracks[2].ID).To(Equal("3"))
		})

		It("sorts by LUFS using media file loudness tags", func() {
			updates := []struct {
				id   string
				lufs string
			}{
				{songRadioactivity.ID, "-16.20"},
				{songAntenna.ID, "-9.80"},
				{songDayInALife.ID, "-12.40"},
			}
			DeferCleanup(func() {
				for _, upd := range updates {
					_, err := GetDBXBuilder().Update("media_file", dbx.Params{
						"tags": "{}",
					}, dbx.HashExp{"id": upd.id}).Execute()
					Expect(err).ToNot(HaveOccurred())
				}
			})

			for _, upd := range updates {
				_, err := GetDBXBuilder().Update("media_file", dbx.Params{
					"tags": `{"loudnorm_final_lufs":[{"value":"` + upd.lufs + `"}]}`,
				}, dbx.HashExp{"id": upd.id}).Execute()
				Expect(err).ToNot(HaveOccurred())
			}

			repo := playlistRepo.Tracks(playlist.ID, true)
			result, err := repo.ReadAll(rest.QueryOptions{Sort: "lufs", Order: "ASC"})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(3))
			Expect(tracks[0].ID).To(Equal("1"))
			Expect(tracks[1].ID).To(Equal("3"))
			Expect(tracks[2].ID).To(Equal("2"))
		})
	})
})
