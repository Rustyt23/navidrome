package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/conf/configtest"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/auth"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retail Player Cover Art Endpoint", func() {
	var (
		router http.Handler
		ds     *tests.MockDataStore
	)

	BeforeEach(func() {
		DeferCleanup(configtest.SetupConfig())
		conf.Server.RetailPlayer.Enabled = true
		conf.Server.EnableMediaFileCoverArt = true

		ds = &tests.MockDataStore{
			MockedMediaFile: tests.CreateMockMediaFileRepo(),
			MockedProperty:  &tests.MockedPropertyRepo{},
		}

		auth.Init(ds)

		nativeRouter := New(ds, nil, nil, nil, core.NewMockLibraryService())
		router = nativeRouter
	})

	It("returns a cover art URL when artworkId is provided", func() {
		recorder := httptest.NewRecorder()
		artID := model.NewArtworkID(model.KindMediaFileArtwork, "song-1", nil)
		query := url.Values{
			"artworkId": []string{artID.String()},
			"size":      []string{"300"},
			"square":    []string{"true"},
		}
		request := httptest.NewRequest(http.MethodGet, "/retailplayer/cover-art?"+query.Encode(), nil)

		router.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))

		var resp retailPlayerCoverArtResponse
		err := json.Unmarshal(recorder.Body.Bytes(), &resp)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.ArtworkID).To(Equal(artID.String()))
		Expect(resp.URL).To(ContainSubstring("/public/img/"))
		Expect(resp.URL).To(ContainSubstring("size=300"))
		Expect(resp.URL).To(ContainSubstring("square=true"))
	})

	It("searches for cover art using metadata", func() {
		recorder := httptest.NewRecorder()

		mediaFile := model.MediaFile{
			ID:          "song-42",
			Title:       "Pigeon",
			Artist:      "Faye Webster",
			Album:       "Atlanta Millionaires Club",
			AlbumID:     "album-1",
			HasCoverArt: true,
			UpdatedAt:   time.Now(),
		}
		ds.MockedMediaFile.(*tests.MockMediaFileRepo).SetData(model.MediaFiles{mediaFile})

		query := url.Values{
			"title":  []string{"Pigeon"},
			"artist": []string{"Faye Webster"},
			"size":   []string{"200"},
		}
		request := httptest.NewRequest(http.MethodGet, "/retailplayer/cover-art?"+query.Encode(), nil)

		router.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))

		var resp retailPlayerCoverArtResponse
		err := json.Unmarshal(recorder.Body.Bytes(), &resp)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.ArtworkID).To(Equal(mediaFile.CoverArtID().String()))
		Expect(resp.MediaFileID).To(Equal(mediaFile.ID))
		Expect(resp.URL).To(ContainSubstring("/public/img/"))
	})

	It("returns 404 when no artwork can be found", func() {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/retailplayer/cover-art?title=Unknown", nil)

		router.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusNotFound))
	})
})
