package nativeapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

type userPreferencesResponse struct {
	Theme     string `json:"theme"`
	IsDefault bool   `json:"isDefault"`
}

type updateUserPreferencesRequest struct {
	Theme string `json:"theme"`
}

func (n *Router) addPreferencesRoute(r chi.Router) {
	r.Route("/preferences", func(r chi.Router) {
		r.Get("/", getUserPreferences(n.ds))
		r.Put("/", updateUserPreferences(n.ds))
	})
}

func getUserPreferences(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, ok := request.UserFrom(ctx)
		if !ok {
			http.Error(w, "user not found in context", http.StatusUnauthorized)
			return
		}

		repo := ds.UserProps(ctx)
		theme, err := repo.Get(user.ID, consts.UserPreferenceThemeKey)
		isDefault := false
		if errors.Is(err, model.ErrNotFound) {
			theme = conf.Server.DefaultTheme
			isDefault = true
		} else if err != nil {
			log.Error(ctx, "Error retrieving user theme preference", err)
			http.Error(w, "failed to load preferences", http.StatusInternalServerError)
			return
		}

		resp := userPreferencesResponse{
			Theme:     theme,
			IsDefault: isDefault,
		}
		if err := rest.RespondWithJSON(w, http.StatusOK, resp); err != nil {
			log.Error(ctx, "Error sending user preferences response", err)
		}
	}
}

func updateUserPreferences(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, ok := request.UserFrom(ctx)
		if !ok {
			http.Error(w, "user not found in context", http.StatusUnauthorized)
			return
		}

		var payload updateUserPreferencesRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		theme := strings.TrimSpace(payload.Theme)
		if theme == "" {
			http.Error(w, "theme is required", http.StatusBadRequest)
			return
		}

		if err := ds.UserProps(ctx).Put(user.ID, consts.UserPreferenceThemeKey, theme); err != nil {
			log.Error(ctx, "Error saving user theme preference", err)
			http.Error(w, "failed to save preferences", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
