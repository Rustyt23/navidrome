package nativeapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/conf/configtest"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User preferences API", func() {
	var (
		ds      *tests.MockDataStore
		user    model.User
		repo    *tests.MockedUserPropsRepo
		handler http.Handler
	)

	BeforeEach(func() {
		DeferCleanup(configtest.SetupConfig())
		repo = &tests.MockedUserPropsRepo{}
		ds = &tests.MockDataStore{MockedUserProps: repo, MockedUser: tests.CreateMockUserRepo()}
		user = model.User{ID: "user-1", UserName: "tester"}
		Expect(ds.User(nil).Put(&user)).To(Succeed())
	})

	Describe("GET /api/preferences", func() {
		It("returns default theme when preference is not stored", func() {
			req := httptest.NewRequest("GET", "/preferences", nil)
			req = req.WithContext(request.WithUser(req.Context(), user))
			w := httptest.NewRecorder()

			handler = getUserPreferences(ds)
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp userPreferencesResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Theme).To(Equal(conf.Server.DefaultTheme))
			Expect(resp.IsDefault).To(BeTrue())
		})

		It("returns stored theme when preference exists", func() {
			Expect(repo.Put(user.ID, consts.UserPreferenceThemeKey, "DarkTheme")).To(Succeed())

			req := httptest.NewRequest("GET", "/preferences", nil)
			req = req.WithContext(request.WithUser(req.Context(), user))
			w := httptest.NewRecorder()

			handler = getUserPreferences(ds)
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp userPreferencesResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp.Theme).To(Equal("DarkTheme"))
			Expect(resp.IsDefault).To(BeFalse())
		})

		It("returns unauthorized when no user context is present", func() {
			req := httptest.NewRequest("GET", "/preferences", nil)
			w := httptest.NewRecorder()

			handler = getUserPreferences(ds)
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("PUT /api/preferences", func() {
		It("stores the provided theme", func() {
			body := bytes.NewBufferString(`{"theme":"SpotifyTheme"}`)
			req := httptest.NewRequest("PUT", "/preferences", body)
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(request.WithUser(req.Context(), user))
			w := httptest.NewRecorder()

			handler = updateUserPreferences(ds)
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
			stored, err := repo.Get(user.ID, consts.UserPreferenceThemeKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(stored).To(Equal("SpotifyTheme"))
		})

		It("validates that theme is provided", func() {
			body := bytes.NewBufferString(`{"theme":""}`)
			req := httptest.NewRequest("PUT", "/preferences", body)
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(request.WithUser(req.Context(), user))
			w := httptest.NewRecorder()

			handler = updateUserPreferences(ds)
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns unauthorized when no user context is present", func() {
			body := bytes.NewBufferString(`{"theme":"DarkTheme"}`)
			req := httptest.NewRequest("PUT", "/preferences", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler = updateUserPreferences(ds)
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})
})
