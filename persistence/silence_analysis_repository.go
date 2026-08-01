package persistence

import (
	"context"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type silenceAnalysisRepository struct {
	sqlRepository
}

func NewSilenceAnalysisRepository(ctx context.Context, db dbx.Builder) model.SilenceAnalysisRepository {
	r := &silenceAnalysisRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "media_file_silence"
	return r
}

func (r *silenceAnalysisRepository) Put(analysis *model.SilenceAnalysis) error {
	if analysis == nil || analysis.MediaFileID == "" {
		return nil
	}
	if analysis.AnalyzedAt.IsZero() {
		analysis.AnalyzedAt = time.Now()
	}
	values, err := toSQLArgs(analysis)
	if err != nil {
		return err
	}
	count, err := r.executeSQL(Update(r.tableName).SetMap(values).Where(Eq{"media_file_id": analysis.MediaFileID}))
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = r.executeSQL(Insert(r.tableName).SetMap(values))
	return err
}

func (r *silenceAnalysisRepository) Get(mediaFileID string) (*model.SilenceAnalysis, error) {
	analysis := model.SilenceAnalysis{}
	err := r.queryOne(Select("*").From(r.tableName).Where(Eq{"media_file_id": mediaFileID}), &analysis)
	if err != nil {
		return nil, err
	}
	return &analysis, nil
}

var _ model.SilenceAnalysisRepository = (*silenceAnalysisRepository)(nil)
