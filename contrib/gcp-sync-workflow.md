# Syncing normalized audio and embedded cover art

With `GCPSync.Enabled = true`, a successful LUFS optimization or **Save fetched
metadata** operation derives `Artist - Title.mp3` from the embedded artist/title
tags, corrects the library filename and database path, and stages the updated MP3
in `SyncFolder/mp3`. The library directory layout and media IDs are preserved.
External M3U references are updated and their changed playlists are staged too. Fetching a cover alone only caches the image; save
it to embed the artwork in the MP3 first. A later edit replaces the staged copy
with the latest version, including both normalization and embedded artwork.

Uploads wait for the **Sync** button by default. The library keeps its updated
file for playback, and the bucket keeps its previous version until Sync runs.
The button calls `GET /api/sync`; the admin `POST /api/gcpsync/sweep` endpoint
performs the same sweep. Both endpoints acknowledge starting the job; completion
or errors are recorded in the server log.

```toml
SyncFolder = './sync'

[GCPSync]
Enabled = true
Bucket = 'your-existing-bucket'
AutoUpload = false
Schedule = '0'
```

The existing gcloud installation and authentication must allow overwriting objects
in that bucket. Keep the existing credentials configuration. The existing filename is never used to infer artist or title.
An MP3 with missing metadata remains staged and the sync reports which tags are
needed. The legacy `EnforceNaming` option no longer controls uploads. A conflicting
local destination is reported rather than overwritten. `DryRun = true` makes no uploads.

Sync overwrites the bucket object at its root using the NFC-normalized filename,
matching the filenames used in device playlists. After a successful upload, the
staged library copy is removed. The library file is never copied back from an
older staged version. Hidden `.gcpsync` marker files retain this distinction
across restarts and are removed with their successfully uploaded copies.

An interrupted or failed upload leaves the staged MP3 available for the next
Sync. Transfers use a stable snapshot, and a newer edit made during a transfer
remains staged for the next Sync. Manual sweeps and the optional automatic worker
serialize their uploads so an older transfer cannot finish after a newer one.

Automatic publishing is an explicit opt-in: set `AutoUpload = true`, then choose
the timing through `Schedule`:

- `'0'` (the default) keeps files staged for the **Sync** button.
- `''` uploads app-updated MP3s immediately.
- A cron expression or interval publishes the Sync folder on that schedule.
  App edits and playlist renames remain staged until the scheduled sweep; they
  do not upload immediately. The Sync button can still publish them earlier.

For example, daily at 2:00 AM in India:

```toml
[GCPSync]
AutoUpload = true
Schedule = 'CRON_TZ=Asia/Kolkata 0 2 * * *'
```

Set the default directly in `conf/configuration.go` by editing the existing
`viper.SetDefault("gcpsync.schedule", "0")` line. Replace `"0"` with your cron
expression, for example `"CRON_TZ=Asia/Kolkata 0 2 * * *"`. No additional Go
variable is needed. Rebuild and restart after changing a Go default. TOML and
`ND_GCPSYNC_SCHEDULE` override this default, including an explicitly empty TOML
value. Without `CRON_TZ`, cron uses the server's local timezone. Invalid active
schedules or timezone names stop startup with a configuration error.

Use `Schedule = '@every 1m'` for a repeating sweep instead of a specific clock
time. Scheduled sweeps include files placed directly in the Sync folder and
pending files left after a restart. With `AutoUpload = false`, files always wait
for Sync, even if a schedule is configured. Restart after changing configuration.

## Validation

The tests use real FFmpeg normalization and TagLib cover embedding, and a local
stand-in for gcloud that captures the bytes passed to `gcloud storage cp`.
They invoke the Sync button's HTTP endpoint and verify:

- No automatic upload in manual mode; the staged file matches the edited song.
- The overwritten bucket MP3 contains the normalized bytes and measures within
  0.2 LU of the configured target.
- Its embedded cover matches the selected PNG exactly, while its audio packets
  are unchanged by saving cover art.
- Failed uploads preserve the staged file and previous bucket object; retry sends
  the exact updated file.
- Restaging during transfer, overlapping automatic/manual sync, queue overflow,
  cancellation, restart, renaming, Unicode paths and separate library roots.

Run the focused checks with:

```sh
go test -race -shuffle=on -count=3 -tags netgo,sqlite_fts5 ./core/gcpsync
go test -race -shuffle=on -count=3 -tags netgo,sqlite_fts5 ./server/nativeapi -run '^TestGCPSync'
```

These tests do not change the production bucket or verify live GCP permissions.

## Email notifications

Configure the recipient and SMTP details through the `gcpsync.notifications.*`
defaults in `conf/configuration.go`, or override them in TOML:

```toml
[GCPSync.Notifications]
Enabled = true
SMTPServer = "smtp.example.com"
SMTPPort = 587
Username = "sender@example.com"
Password = "your-smtp-app-password"
From = "sender@example.com"
To = "your-new-recipient@example.com"
Subject = "Music sync result"
```

The default is disabled with an empty recipient. Port 587 requires STARTTLS;
port 465 uses implicit TLS. Certificates are verified. Each manual/scheduled
sweep sends one summary listing the uploaded objects and any failures. Automatic
uploads send a result for each completed upload attempt. Notification errors are
logged and do not undo a successful upload. Restart after changing configuration.
