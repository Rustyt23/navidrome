package persistence

import (
	"context"
	"database/sql"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type loudnessAuditRepository struct {
	sqlRepository
	// conn is the raw pool, needed only for snapshots: ATTACH belongs to a
	// single connection, so those take one of their own rather than borrowing
	// whichever the query builder happens to use. Nil where snapshots are not
	// available, which the snapshot calls report rather than panic on.
	conn *sql.DB
}

func NewLoudnessAuditRepository(ctx context.Context, db dbx.Builder, conn *sql.DB) model.LoudnessAuditRepository {
	r := &loudnessAuditRepository{conn: conn}
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

	// A record that describes an exception raises the latch. Doing it here
	// rather than at each of the places that build an audit means no write path
	// can forget to, and the rule stays in one place.
	if audit.IsException() {
		audit.WasException = true
	}

	values, err := toSQLArgs(audit)
	if err != nil {
		return err
	}
	// The client's decision is theirs: re-analysing a track refreshes its
	// measurements but must never silently discard a choice already made.
	//
	// was_exception is skipped for the same reason whenever this record is not
	// itself an exception: a fresh analysis of a track that has since been fixed
	// carries false, and writing that would erase the history the column exists
	// to keep. Omitting the column leaves whatever is already stored.
	// A record with no measurement in it must not erase the one that is stored.
	//
	// Every failure path builds a fresh audit and leaves the measurement fields
	// at their zero values, so a single transient error - a decode timeout, a
	// drive unmounted for a moment - used to write lufs_before = NULL,
	// codec_before = '', size_before = 0 straight over a good row. The before
	// snapshot is the only record of what the song originally was, and once the
	// backup is deleted it cannot be taken again.
	//
	// Expressed as what a failure may write rather than what it must not.
	// Listing the columns to skip meant every column added later was silently
	// opted in to being destroyed, and the first version of this missed the
	// after-snapshot and the phase for exactly that reason.
	//
	// Keyed on the record calling itself a failure and carrying no measurement.
	// Keying on the missing measurement alone was wrong: plenty of legitimate
	// records - a re-analysis that found nothing wrong, a decision being
	// recorded - carry no before-snapshot either, and blocking those froze the
	// phase and action of any track they touched.
	//
	// Inserts are unaffected: a brand new row has nothing to protect and writes
	// everything it has, which is how a never-measured track still gets its
	// failure recorded.
	keepStoredMeasurements := audit.Status == model.LoudnessStatusFailed && audit.LufsBefore == nil

	// What a failed attempt legitimately knows: that it happened, when, and why
	// it did not work. Everything else describes the song, which it did not
	// manage to look at.
	failureColumns := map[string]bool{
		"status": true, "verdict": true, "error": true,
		"analyzed_at": true, "has_backup": true,
	}

	updateValues := make(map[string]any, len(values))
	for k, v := range values {
		if k == "decision" || (k == "was_exception" && !audit.WasException) {
			continue
		}
		if keepStoredMeasurements && !failureColumns[k] {
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
// Making a choice about a track is what marks it as one that needed a human, so
// recording a decision also raises the exception latch. An empty decision is the
// "undecide this" case and must not: it is the removal of a choice, not one.
func (r *loudnessAuditRepository) SetDecision(mediaFileID, decision string) error {
	update := Update(r.tableName).Set("decision", decision)
	if decision != "" {
		update = update.Set("was_exception", true)
	}
	update = update.Where(Eq{"media_file_id": mediaFileID})
	count, err := r.executeSQL(update)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	// analyzed_at is set even though nothing has been analysed. The column has no
	// default, and LoudnessAudit.AnalyzedAt is a time.Time rather than a pointer,
	// so a NULL here makes every later Get of this row fail to scan - which the
	// callers treat as "no record", quietly discarding the decision just made.
	insert := Insert(r.tableName).
		Columns("media_file_id", "decision", "was_exception", "analyzed_at").
		Values(mediaFileID, decision, decision != "", time.Now())
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
