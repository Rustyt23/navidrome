package persistence

import (
	"context"
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type retailPlayerDeviceMappingRepository struct {
	sqlRepository
}

func NewRetailPlayerDeviceMappingRepository(ctx context.Context, db dbx.Builder) model.RetailPlayerDeviceMappingRepository {
	r := &retailPlayerDeviceMappingRepository{}
	r.ctx = ctx
	r.db = db
	r.registerModel(&model.RetailPlayerDeviceMapping{}, nil)
	return r
}

func (r *retailPlayerDeviceMappingRepository) FindBySlugKey(slugKey string) (*model.RetailPlayerDeviceMapping, error) {
	normalized := strings.TrimSpace(strings.ToLower(slugKey))
	if normalized == "" {
		return nil, model.ErrNotFound
	}

	sel := r.newSelect().Columns("*").Where(Eq{"slug_key": normalized}).Limit(1)
	mapping := model.RetailPlayerDeviceMapping{}
	if err := r.queryOne(sel, &mapping); err != nil {
		return nil, err
	}
	return &mapping, nil
}

var _ model.RetailPlayerDeviceMappingRepository = (*retailPlayerDeviceMappingRepository)(nil)
