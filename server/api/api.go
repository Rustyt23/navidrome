package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func NewRouter() http.Handler {
	s := New()
	ng := chi.NewRouter()

	ng.MethodFunc(http.MethodGet, "/triggers", s.RetailPlayerGetTriggers)
	ng.MethodFunc(http.MethodPost, "/play", s.RetailPlayerPlayCue)
	ng.MethodFunc(http.MethodPost, "/stop", s.RetailPlayerStopCue)

	return ng
}
