package nativeapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/persistence"
)

func statusFor(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, rest.ErrNotFound), errors.Is(err, model.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, rest.ErrPermissionDenied), errors.Is(err, model.ErrNotAuthorized):
		return http.StatusForbidden
	case errors.Is(err, persistence.ErrInvalidRequest):
		return http.StatusBadRequest
	default:
		s := strings.ToLower(err.Error())
		if strings.Contains(s, "unique constraint failed") {
			return http.StatusConflict
		}
		return http.StatusInternalServerError
	}
}
