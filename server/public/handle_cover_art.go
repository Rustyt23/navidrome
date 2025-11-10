package public

import (
	"context"
	"net/http"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
)

func (pub *Router) handleGetCoverArt() http.HandlerFunc {
	constructor := func(ctx context.Context) rest.Repository {
		return pub.ds.Resource(ctx, model.MediaFile{})
	}

	return rest.GetAll(constructor)
}
