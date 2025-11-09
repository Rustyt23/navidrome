package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
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

		It("returns only duplicate playlist tracks", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"duplicatesOnly": true},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())

			Expect(tracks).To(HaveLen(3))
			Expect(tracks[0].ID).To(Equal("2"))
			Expect(tracks[1].ID).To(Equal("4"))
			Expect(tracks[2].ID).To(Equal("5"))
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

			Expect(tracks).To(HaveLen(4))
			Expect(tracks[0].ID).To(Equal("2"))
			Expect(tracks[1].ID).To(Equal("4"))
			Expect(tracks[2].ID).To(Equal("5"))
			Expect(tracks[3].Missing).To(BeTrue())
			Expect(tracks[3].Path).To(Equal("ghost-track.mp3"))
			for _, track := range tracks {
				Expect(track.Path).ToNot(Equal("phantom.mp3"))
			}
		})
	})

	Describe("string column sorting", func() {
		var (
			playlist         model.Playlist
			mediaRepo        model.MediaFileRepository
			insertedTracks   []model.MediaFile
			insertedTrackIDs []string
		)

		BeforeEach(func() {
			mediaRepo = NewMediaFileRepository(ctx, GetDBXBuilder())
			insertedTracks = []model.MediaFile{
				mf(model.MediaFile{
					ID:       id.NewRandom(),
					Title:    "Playlist Alpha",
					ArtistID: songAntenna.ArtistID,
					Artist:   songAntenna.Artist,
					AlbumID:  songAntenna.AlbumID,
					Album:    songAntenna.Album,
					Path:     p(fmt.Sprintf("/playlist/string-alpha-%s.mp3", id.NewRandom())),
					Genre:    "rock",
					Comment:  "The Avayas",
				}),
				mf(model.MediaFile{
					ID:       id.NewRandom(),
					Title:    "Playlist Beta",
					ArtistID: songAntenna.ArtistID,
					Artist:   songAntenna.Artist,
					AlbumID:  songAntenna.AlbumID,
					Album:    songAntenna.Album,
					Path:     p(fmt.Sprintf("/playlist/string-beta-%s.mp3", id.NewRandom())),
					Genre:    "Blues",
					Comment:  "beta words",
				}),
				mf(model.MediaFile{
					ID:       id.NewRandom(),
					Title:    "Playlist Gamma",
					ArtistID: songAntenna.ArtistID,
					Artist:   songAntenna.Artist,
					AlbumID:  songAntenna.AlbumID,
					Album:    songAntenna.Album,
					Path:     p(fmt.Sprintf("/playlist/string-gamma-%s.mp3", id.NewRandom())),
					Genre:    "theatre",
					Comment:  "alpha notes",
				}),
			}

			insertedTrackIDs = make([]string, len(insertedTracks))
			for i := range insertedTracks {
				Expect(mediaRepo.Put(&insertedTracks[i])).To(Succeed())
				insertedTrackIDs[i] = insertedTracks[i].ID
				mfID := insertedTracks[i].ID
				DeferCleanup(func() { _ = mediaRepo.Delete(mfID) })
			}

			playlist = model.Playlist{Name: "Sorting Playlist", OwnerID: "userid", OwnerName: "userid"}
			playlist.AddMediaFilesByID(insertedTrackIDs)
			Expect(playlistRepo.Put(&playlist)).To(Succeed())
			DeferCleanup(func() {
				if playlist.ID != "" {
					_ = playlistRepo.Delete(playlist.ID)
				}
			})
		})

		It("sorts playlist genres case-insensitively", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Sort:  "genre",
				Order: "asc",
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(len(insertedTracks)))
			expected := append([]model.MediaFile(nil), insertedTracks...)
			slices.SortFunc(expected, func(a, b model.MediaFile) int {
				return strings.Compare(strings.ToLower(a.Genre), strings.ToLower(b.Genre))
			})

			for i, mf := range expected {
				Expect(tracks[i].MediaFileID).To(Equal(mf.ID))
			}
		})

		It("sorts playlist comments without stripping articles", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Sort:  "comment",
				Order: "asc",
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())
			Expect(tracks).To(HaveLen(len(insertedTracks)))
			expected := append([]model.MediaFile(nil), insertedTracks...)
			slices.SortFunc(expected, func(a, b model.MediaFile) int {
				return strings.Compare(strings.ToLower(a.Comment), strings.ToLower(b.Comment))
			})

			for i, mf := range expected {
				Expect(tracks[i].MediaFileID).To(Equal(mf.ID))
			}
			Expect(tracks[len(tracks)-1].Comment).To(Equal("The Avayas"))
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

		It("finds tracks when substring matches metadata even if full_text is stale", func() {
			originalID := playlist.ID
			if originalID != "" {
				Expect(playlistRepo.Delete(originalID)).To(Succeed())
			}

			mediaRepo := NewMediaFileRepository(ctx, GetDBXBuilder())
			track := mf(model.MediaFile{
				ID:          "2001",
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
				id    string
				title string
			}{
				{songRadioactivity.ID, songRadioactivity.Title},
				{songAntenna.ID, songAntenna.Title},
				{songDayInALife.ID, songDayInALife.Title},
			}
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
	})
})
