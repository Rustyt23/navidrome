package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upRepairSilenceTrimBackupSchema, downRepairSilenceTrimBackupSchema)
}

func upRepairSilenceTrimBackupSchema(ctx context.Context, tx *sql.Tx) error {
	columns := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(media_file_silence_backup)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !columns["original_mode"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE media_file_silence_backup ADD COLUMN original_mode INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add silence backup original_mode: %w", err)
		}
	}
	if !columns["original_mod_time"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE media_file_silence_backup ADD COLUMN original_mod_time DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00'`); err != nil {
			return fmt.Errorf("add silence backup original_mod_time: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE media_file_silence_backup SET original_mod_time = created_at WHERE original_mod_time = '1970-01-01 00:00:00'`); err != nil {
			return fmt.Errorf("backfill silence backup original_mod_time: %w", err)
		}
	}
	if !columns["prepared_at"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE media_file_silence_backup ADD COLUMN prepared_at DATETIME`); err != nil {
			return fmt.Errorf("add silence backup prepared_at: %w", err)
		}
	}
	return nil
}

func downRepairSilenceTrimBackupSchema(context.Context, *sql.Tx) error {
	// SQLite cannot safely drop individual columns on all supported versions;
	// this repair is intentionally forward-only.
	return nil
}
