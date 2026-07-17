package server

import (
	"net/http"
	"net/http/httptest"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("frontend assets", func() {
	It("serves the standalone device player without authentication", func() {
		assets := fstest.MapFS{
			"index.html":                    {Data: []byte("main app")},
			"player/index.html":             {Data: []byte("standalone player")},
			"player-assets/player.js":       {Data: []byte("player module")},
			"player-assets/player-utils.js": {Data: []byte("player utilities")},
		}
		server := &Server{appRoot: "/app"}
		router := chi.NewRouter()
		router.Mount("/app", server.frontendAssetsHandlerWithFS(assets))
		request := httptest.NewRequest(http.MethodGet, "/app/player/AlilaMarea_Pool", nil)
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		Expect(response.Code).To(Equal(http.StatusOK))
		Expect(response.Header().Get("Content-Type")).To(Equal("text/html; charset=utf-8"))
		Expect(response.Body.String()).To(Equal("standalone player"))

		assetRequest := httptest.NewRequest(http.MethodGet, "/app/player-assets/player.js", nil)
		assetResponse := httptest.NewRecorder()
		router.ServeHTTP(assetResponse, assetRequest)

		Expect(assetResponse.Code).To(Equal(http.StatusOK))
		Expect(assetResponse.Body.String()).To(Equal("player module"))
	})
})
