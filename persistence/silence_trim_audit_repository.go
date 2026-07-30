package persistence

import (
	"context"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type silenceTrimAuditRepository struct {
	sqlRepository
}

func NewSilenceTrimAuditRepository(ctx context.Context, db dbx.Builder) model.SilenceTrimAuditRepository {
	r := &silenceTrimAuditRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "media_file_silence_trim"
	return r
}

// Put stores the proposal and its decision together. A human approval belongs
// to exact source bytes and trim boundaries, so callers deliberately carry it
// forward only when that proposal is unchanged.
func (r *silenceTrimAuditRepository) Put(audit *model.SilenceTrimAudit) error {
	if audit == nil || audit.MediaFileID == "" {
		return nil
	}
	if audit.AnalyzedAt.IsZero() {
		audit.AnalyzedAt = time.Now()
	}

	values, err := toSQLArgs(audit)
	if err != nil {
		return err
	}
	count, err := r.executeSQL(
		Update(r.tableName).
			SetMap(values).
			Where(Eq{"media_file_id": audit.MediaFileID}),
	)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = r.executeSQL(Insert(r.tableName).SetMap(values))
	return err
}

func (r *silenceTrimAuditRepository) Get(mediaFileID string) (*model.SilenceTrimAudit, error) {
	res := model.SilenceTrimAudit{}
	err := r.queryOne(Select("*").From(r.tableName).Where(Eq{"media_file_id": mediaFileID}), &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *silenceTrimAuditRepository) SetDecision(mediaFileID, decision string) error {
	count, err := r.executeSQL(
		Update(r.tableName).
			Set("decision", decision).
			Where(Eq{"media_file_id": mediaFileID}),
	)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = r.executeSQL(
		Insert(r.tableName).
			Columns("media_file_id", "decision").
			Values(mediaFileID, decision),
	)
	return err
}

func (r *silenceTrimAuditRepository) Clear() (int64, error) {
	// Applied/restored rows are the provenance required to prove that a backup
	// belongs to this song generation. "Clear analysis" may remove only
	// disposable dry-run rows.
	return r.executeSQL(
		Delete(r.tableName).Where(Eq{"has_backup": false}),
	)
}

var _ model.SilenceTrimAuditRepository = (*silenceTrimAuditRepository)(nil)
