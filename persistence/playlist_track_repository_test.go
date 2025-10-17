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

		Describe("sorting", func() {
			var (
				playlist        model.Playlist
				repo            model.PlaylistTrackRepository
				mediaRepo       model.MediaFileRepository
				cleanupTrackIDs []string
				seededTracks    []model.MediaFile
			)

			beforePlaylist := func() {
				mediaRepo = NewMediaFileRepository(ctx, GetDBXBuilder())
				tracks := []model.MediaFile{
					mf(model.MediaFile{
						ID:                   "9901",
						Title:                "Gamma Track",
						OrderTitle:           "gamma track",
						ArtistID:             songComeTogether.ArtistID,
						Artist:               "Artist C",
						OrderArtistName:      "artist c",
						AlbumID:              songComeTogether.AlbumID,
						Album:                "Album C",
						OrderAlbumName:       "album c",
						AlbumArtist:          "Album Artist C",
						OrderAlbumArtistName: "album artist c",
						Duration:             300,
						Genre:                "Rock",
						Comment:              "Zulu",
						TrackNumber:          3,
						Path:                 p("/playlist/gamma.mp3"),
					}),
					mf(model.MediaFile{
						ID:                   "9902",
						Title:                "Alpha Track",
						OrderTitle:           "alpha track",
						ArtistID:             songComeTogether.ArtistID,
						Artist:               "Artist A",
						OrderArtistName:      "artist a",
						AlbumID:              songComeTogether.AlbumID,
						Album:                "Album A",
						OrderAlbumName:       "album a",
						AlbumArtist:          "Album Artist A",
						OrderAlbumArtistName: "album artist a",
						Duration:             120,
						Genre:                "Blues",
						Comment:              "Alpha",
						TrackNumber:          1,
						Path:                 p("/playlist/alpha.mp3"),
					}),
					mf(model.MediaFile{
						ID:                   "9903",
						Title:                "Beta Track",
						OrderTitle:           "beta track",
						ArtistID:             songComeTogether.ArtistID,
						Artist:               "Artist B",
						OrderArtistName:      "artist b",
						AlbumID:              songComeTogether.AlbumID,
						Album:                "Album B",
						OrderAlbumName:       "album b",
						AlbumArtist:          "Album Artist B",
						OrderAlbumArtistName: "album artist b",
						Duration:             210,
						Genre:                "Classical",
						Comment:              "Mid",
						TrackNumber:          2,
						Path:                 p("/playlist/beta.mp3"),
					}),
				}

				seededTracks = append([]model.MediaFile(nil), tracks...)
				cleanupTrackIDs = make([]string, len(tracks))
				for i := range tracks {
					track := tracks[i]
					cleanupTrackIDs[i] = track.ID
					Expect(mediaRepo.Put(&track)).To(Succeed())
				}

				playlist = model.Playlist{Name: "Sort Playlist", OwnerID: "userid", OwnerName: "userid"}
				playlist.AddMediaFilesByID(cleanupTrackIDs)
				Expect(playlistRepo.Put(&playlist)).To(Succeed())
				repo = playlistRepo.Tracks(playlist.ID, true)
			}

			AfterEach(func() {
				if playlist.ID != "" {
					Expect(playlistRepo.Delete(playlist.ID)).To(Succeed())
				}
				for _, id := range cleanupTrackIDs {
					Expect(mediaRepo.Delete(id)).To(Succeed())
				}
				cleanupTrackIDs = nil
				seededTracks = nil
			})

			It("sorts by artist metadata", func() {
				beforePlaylist()
				result, err := repo.ReadAll(rest.QueryOptions{Sort: "artist", Order: "ASC"})
				Expect(err).ToNot(HaveOccurred())

				tracks, ok := result.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(tracks).To(HaveLen(3))
				Expect([]string{tracks[0].Title, tracks[1].Title, tracks[2].Title}).To(Equal([]string{"Alpha Track", "Beta Track", "Gamma Track"}))
			})

			It("sorts by duration", func() {
				beforePlaylist()
				result, err := repo.ReadAll(rest.QueryOptions{Sort: "duration", Order: "ASC"})
				Expect(err).ToNot(HaveOccurred())

				tracks, ok := result.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(tracks).To(HaveLen(3))
				Expect([]string{tracks[0].Title, tracks[1].Title, tracks[2].Title}).To(Equal([]string{"Alpha Track", "Beta Track", "Gamma Track"}))
			})

			It("sorts by genre", func() {
				beforePlaylist()
				result, err := repo.ReadAll(rest.QueryOptions{Sort: "genre", Order: "ASC"})
				Expect(err).ToNot(HaveOccurred())

				tracks, ok := result.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(tracks).To(HaveLen(3))
				Expect([]string{tracks[0].Title, tracks[1].Title, tracks[2].Title}).To(Equal([]string{"Alpha Track", "Beta Track", "Gamma Track"}))
			})

			It("sorts by comment", func() {
				beforePlaylist()
				result, err := repo.ReadAll(rest.QueryOptions{Sort: "comment", Order: "ASC"})
				Expect(err).ToNot(HaveOccurred())

				tracks, ok := result.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(tracks).To(HaveLen(3))
				Expect([]string{tracks[0].Title, tracks[1].Title, tracks[2].Title}).To(Equal([]string{"Alpha Track", "Beta Track", "Gamma Track"}))
			})

			It("sorts by title", func() {
				beforePlaylist()
				result, err := repo.ReadAll(rest.QueryOptions{Sort: "title", Order: "ASC"})
				Expect(err).ToNot(HaveOccurred())

				tracks, ok := result.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(tracks).To(HaveLen(3))
				Expect([]string{tracks[0].Title, tracks[1].Title, tracks[2].Title}).To(Equal([]string{"Alpha Track", "Beta Track", "Gamma Track"}))
			})

			It("sorts synced playlists when missing entries are merged", func() {
				beforePlaylist()

				playlist.Sync = true
				playlist.Path = filepath.Join(GinkgoT().TempDir(), "sorted_sync.m3u")
				contents := strings.Join([]string{
					seededTracks[0].Path,
					"ghost-track.mp3",
					seededTracks[2].Path,
					seededTracks[1].Path,
				}, "\n")
				Expect(os.WriteFile(playlist.Path, []byte(contents), 0o600)).To(Succeed())
				playlist.Tracks = nil
				Expect(playlistRepo.Put(&playlist)).To(Succeed())

				repo = playlistRepo.Tracks(playlist.ID, true)

				result, err := repo.ReadAll(rest.QueryOptions{Sort: "artist", Order: "ASC"})
				Expect(err).ToNot(HaveOccurred())

				tracks, ok := result.(model.PlaylistTracks)
				Expect(ok).To(BeTrue())
				Expect(tracks).To(HaveLen(4))
				Expect([]string{tracks[0].Title, tracks[1].Title, tracks[2].Title}).To(Equal([]string{"Alpha Track", "Beta Track", "Gamma Track"}))
				Expect(tracks[3].Missing).To(BeTrue())
				Expect(tracks[3].Path).To(Equal("ghost-track.mp3"))
			})
		})
	})
})
