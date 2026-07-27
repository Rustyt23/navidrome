package persistence

import (
	"context"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type loudnessAuditRepository struct {
	sqlRepository
}

func NewLoudnessAuditRepository(ctx context.Context, db dbx.Builder) model.LoudnessAuditRepository {
	r := &loudnessAuditRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "media_file_loudness"
	return r
}

// Put upserts the audit record for a media file. media_file_id is the primary
// key, so re-analyzing a track replaces its previous record.
func (r *loudnessAuditRepository) Put(audit *model.LoudnessAudit) error {
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
	// The client's decision is theirs: re-analysing a track refreshes its
	// measurements but must never silently discard a choice already made.
	updateValues := make(map[string]any, len(values))
	for k, v := range values {
		if k == "decision" {
			continue
		}
		updateValues[k] = v
	}
	update := Update(r.tableName).SetMap(updateValues).Where(Eq{"media_file_id": audit.MediaFileID})
	count, err := r.executeSQL(update)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	insert := Insert(r.tableName).SetMap(values)
	_, err = r.executeSQL(insert)
	return err
}

// SetDecision records the client's choice for a phase 2 track, creating the
// row if the track has not been analysed yet.
func (r *loudnessAuditRepository) SetDecision(mediaFileID, decision string) error {
	update := Update(r.tableName).Set("decision", decision).Where(Eq{"media_file_id": mediaFileID})
	count, err := r.executeSQL(update)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	insert := Insert(r.tableName).Columns("media_file_id", "decision").Values(mediaFileID, decision)
	_, err = r.executeSQL(insert)
	return err
}

func (r *loudnessAuditRepository) Get(mediaFileID string) (*model.LoudnessAudit, error) {
	sel := Select("*").From(r.tableName).Where(Eq{"media_file_id": mediaFileID})
	res := model.LoudnessAudit{}
	err := r.queryOne(sel, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Clear removes only derived loudness-analysis records. It never touches media
// files, backups, or the loudness tags already stored on media_file rows.
func (r *loudnessAuditRepository) Clear() (int64, error) {
	return r.executeSQL(Delete(r.tableName))
}

var _ model.LoudnessAuditRepository = (*loudnessAuditRepository)(nil)
