package persistence

import (
	"context"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type silenceAuditRepository struct {
	sqlRepository
}

func NewSilenceAuditRepository(ctx context.Context, db dbx.Builder) model.SilenceAuditRepository {
	r := &silenceAuditRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "media_file_silence"
	return r
}

// Put upserts the silence record for a media file. media_file_id is the primary
// key, so re-analysing a track replaces its previous record.
func (r *silenceAuditRepository) Put(audit *model.SilenceAudit) error {
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

	// trimmed_at is left alone when this write is not itself a trim.
	//
	// Re-analysing an already-trimmed track measures what is on disk now, which
	// is the trimmed file - correct, and worth recording. But the analysis
	// carries no trimmed_at, and writing that nil would erase the fact that the
	// file was cut. The run filter reads exactly that column to decide what to
	// leave alone, so erasing it puts an already-trimmed track back in the queue,
	// where the next run would cut the margin off it as well.
	updateValues := make(map[string]any, len(values))
	for k, v := range values {
		if k == "trimmed_at" && audit.TrimmedAt == nil {
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

func (r *silenceAuditRepository) Get(mediaFileID string) (*model.SilenceAudit, error) {
	sel := Select("*").From(r.tableName).Where(Eq{"media_file_id": mediaFileID})
	res := model.SilenceAudit{}
	if err := r.queryOne(sel, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Clear removes only the derived silence-analysis records. It never touches
// media files or anything the loudness feature owns.
func (r *silenceAuditRepository) Clear() (int64, error) {
	return r.executeSQL(Delete(r.tableName))
}

func (r *silenceAuditRepository) CountByVerdict() (map[string]int64, error) {
	sel := Select("verdict", "count(*) as count").From(r.tableName).GroupBy("verdict")
	var rows []struct {
		Verdict string `structs:"verdict"`
		Count   int64  `structs:"count"`
	}
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Verdict] = row.Count
	}
	return counts, nil
}

// PendingTrimSeconds totals the time waiting to be removed: every trimmable
// track that has not been cut yet. Tracks already trimmed are excluded, so the
// figure counts down to zero as a run progresses rather than staying put.
func (r *silenceAuditRepository) PendingTrimSeconds() (float64, error) {
	sel := Select("coalesce(sum(lead_trim + trail_trim), 0) as total").From(r.tableName).
		Where(And{
			Eq{"verdict": model.SilenceVerdictTrimmable},
			Eq{"trimmed_at": nil},
		})
	var row struct {
		Total float64 `structs:"total"`
	}
	if err := r.queryOne(sel, &row); err != nil {
		return 0, err
	}
	return row.Total, nil
}

var _ model.SilenceAuditRepository = (*silenceAuditRepository)(nil)
