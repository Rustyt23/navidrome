package persistence

import (
	"context"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type silenceBackupRepository struct {
	sqlRepository
}

func NewSilenceBackupRepository(ctx context.Context, db dbx.Builder) model.SilenceBackupRepository {
	r := &silenceBackupRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "media_file_silence_backup"
	return r
}

func (r *silenceBackupRepository) Put(backup *model.SilenceBackup) error {
	if backup == nil || backup.MediaFileID == "" {
		return nil
	}
	if backup.CreatedAt.IsZero() {
		backup.CreatedAt = time.Now()
	}
	values, err := toSQLArgs(backup)
	if err != nil {
		return err
	}
	count, err := r.executeSQL(Update(r.tableName).SetMap(values).Where(Eq{"media_file_id": backup.MediaFileID}))
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = r.executeSQL(Insert(r.tableName).SetMap(values))
	return err
}

func (r *silenceBackupRepository) Get(mediaFileID string) (*model.SilenceBackup, error) {
	backup := model.SilenceBackup{}
	err := r.queryOne(Select("*").From(r.tableName).Where(Eq{"media_file_id": mediaFileID}), &backup)
	if err != nil {
		return nil, err
	}
	return &backup, nil
}

var _ model.SilenceBackupRepository = (*silenceBackupRepository)(nil)
