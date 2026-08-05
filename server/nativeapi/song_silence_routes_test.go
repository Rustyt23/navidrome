package nativeapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("addSongSilenceRoute", func() {
	// The silence page reaches these by absolute path, and the native API is
	// mounted at /api - so the UI must ask for "/api/song/silence/...". Asking
	// for a bare "song/silence/..." resolves relative to the current page and
	// 404s, which is exactly what broke the page on first load. This pins the
	// server half of that contract; the UI half is pinned in useSilenceStatus.
	It("registers every endpoint the page calls", func() {
		router := chi.NewRouter()
		(&Router{}).addSongSilenceRoute(router)

		registered := map[string]bool{}
		err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			registered[method+" "+route] = true
			return nil
		})
		Expect(err).ToNot(HaveOccurred())

		for _, want := range []string{
			"GET /song/silence/analyze",
			"POST /song/silence/analyze",
			"POST /song/silence/analyze/stop",
			"DELETE /song/silence/analyze/results",
			"GET /song/silence/trim",
			"POST /song/silence/trim",
			"POST /song/silence/trim/stop",
			"GET /song/silence/summary",
			"GET /song/silence/settings",
		} {
			Expect(registered).To(HaveKey(want), "missing route %s", want)
		}
	})
})
