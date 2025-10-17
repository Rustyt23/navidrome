package persistence

import (
	"context"
	"os"
	"path/filepath"

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
		var (
			playlist      model.Playlist
			mediaRepo     model.MediaFileRepository
			createdTracks []model.MediaFile
			repo          model.PlaylistTrackRepository
		)

		BeforeEach(func() {
			mediaRepo = NewMediaFileRepository(ctx, GetDBXBuilder())
			createdTracks = []model.MediaFile{
				mf(model.MediaFile{
					ID:              "9101",
					Title:           "Sort Song Gamma",
					ArtistID:        "3",
					Artist:          "Gamma Artist",
					OrderArtistName: "gamma artist",
					AlbumID:         "101",
					Album:           "Sgt Peppers",
					TrackNumber:     3,
					Genre:           "Jazz",
					Comment:         "Charlie",
					Duration:        240,
					Path:            p("/sort/gamma.mp3"),
				}),
				mf(model.MediaFile{
					ID:              "9102",
					Title:           "Sort Song Alpha",
					ArtistID:        "3",
					Artist:          "Alpha Artist",
					OrderArtistName: "alpha artist",
					AlbumID:         "101",
					Album:           "Sgt Peppers",
					TrackNumber:     1,
					Genre:           "Blues",
					Comment:         "Alpha",
					Duration:        120,
					Path:            p("/sort/alpha.mp3"),
				}),
				mf(model.MediaFile{
					ID:              "9103",
					Title:           "Sort Song Beta",
					ArtistID:        "3",
					Artist:          "Beta Artist",
					OrderArtistName: "beta artist",
					AlbumID:         "101",
					Album:           "Sgt Peppers",
					TrackNumber:     2,
					Genre:           "Rock",
					Comment:         "Bravo",
					Duration:        180,
					Path:            p("/sort/beta.mp3"),
				}),
			}

			for i := range createdTracks {
				track := createdTracks[i]
				Expect(mediaRepo.Put(&track)).To(Succeed())
				createdTracks[i] = track
			}

			playlist = model.Playlist{Name: "Sorting", OwnerID: "userid", OwnerName: "userid"}
			playlist.AddMediaFilesByID([]string{createdTracks[0].ID, createdTracks[1].ID, createdTracks[2].ID})
			Expect(playlistRepo.Put(&playlist)).To(Succeed())

			repo = playlistRepo.Tracks(playlist.ID, true)
		})

		AfterEach(func() {
			if playlist.ID != "" {
				Expect(playlistRepo.Delete(playlist.ID)).To(Succeed())
			}
			for _, track := range createdTracks {
				Expect(mediaRepo.Delete(track.ID)).To(Succeed())
			}
		})

		It("sorts playlist tracks by key metadata fields", func() {
			cases := []struct {
				field   string
				asc     []string
				message string
			}{
				{
					field:   "artist",
					asc:     []string{"9102", "9103", "9101"},
					message: "artist",
				},
				{
					field:   "trackNumber",
					asc:     []string{"9102", "9103", "9101"},
					message: "track number",
				},
				{
					field:   "genre",
					asc:     []string{"9102", "9101", "9103"},
					message: "genre",
				},
				{
					field:   "comment",
					asc:     []string{"9102", "9103", "9101"},
					message: "comment",
				},
				{
					field:   "duration",
					asc:     []string{"9102", "9103", "9101"},
					message: "duration",
				},
			}

			for _, tc := range cases {
				tc := tc
				resultAsc, err := repo.ReadAll(rest.QueryOptions{Sort: tc.field, Order: "ASC"})
				Expect(err).ToNot(HaveOccurred())
				ascTracks, ok := resultAsc.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(extractMediaFileIDs(ascTracks)).To(Equal(tc.asc), "ascending sort by %s", tc.message)

				resultDesc, err := repo.ReadAll(rest.QueryOptions{Sort: tc.field, Order: "DESC"})
				Expect(err).ToNot(HaveOccurred())
				descTracks, ok := resultDesc.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(extractMediaFileIDs(descTracks)).To(Equal(reverseStrings(tc.asc)), "descending sort by %s", tc.message)
			}
		})
	})
})

func extractMediaFileIDs(tracks model.PlaylistTracks) []string {
	ids := make([]string, len(tracks))
	for i, track := range tracks {
		ids[i] = track.MediaFileID
	}
	return ids
}

func reverseStrings(values []string) []string {
	reversed := make([]string, len(values))
	for i := range values {
		reversed[i] = values[len(values)-1-i]
	}
	return reversed
}
