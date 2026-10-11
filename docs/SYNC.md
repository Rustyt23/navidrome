# GCPSync — The Sync Service

A map of the sync subsystem: what puts files in the Sync folder, what the
service does with them, what reaches the Google Cloud Storage bucket, and where
everything lands locally. Written from the code as it stands on branch
`master_vikash3_rustyt_issues_fixed+download`.

- **Goal:** keep a GCS bucket and the local library in agreement, so playback
  devices pulling from the bucket get the same audio and playlists the server
  has.
- **Origin:** replaces an external `sync-to-GCS` bash script that ran on cron on
  the server. The NFC renaming and CRLF cleaning below are that script's
  `convmv` / `sed` / `uconv` steps, reimplemented in Go.
- **Credentials:** uploads shell out to the `gcloud` CLI, so the server needs no
  credential handling of its own beyond pointing at a service-account file.

---

## 1. Component map

| Layer | Path | Responsibility |
|---|---|---|
| Service | `core/gcpsync/gcpsync.go` | Everything below: queue, worker, sweep, uploads, moves |
| Config | `conf/configuration.go` | `GCPSync.*` options, `SyncFolder`, `MusicFolder`, `PlaylistsPath` |
| Startup | `cmd/root.go` → `startGCPSync` | Starts the worker, registers the cron sweep |
| HTTP | `server/nativeapi/gcpsync.go` | `POST /api/gcpsync/sweep` |
| | `server/nativeapi/native_api.go` → `addSyncRoute` | `GET /api/sync` (the UI button) |
| Producers | `server/nativeapi/song_loudness_run.go` | Stages LUFS-normalized tracks |
| | `server/nativeapi/musicbrainz_metadata.go` | Stages tracks after cover art / tag writes |
| | `core/playlists/fork_features.go` | Mirrors saved playlists into the Sync folder |
| | `core/discoveries.go` → `Publish` | Moves a whole discovery folder into the Sync folder |
| | `server/nativeapi/retail_player.go` | Rewrites a synced playlist after a dislike |
| UI | `ui/src/layout/ActivityPanel.jsx` | The "Sync" button |
| Naming | `core/gcpsync/naming.go` | The `Artist - Title.mp3` rules |
| | `core/gcpsync/settings.go` | The UI-controlled enforcement switch |
| | `server/nativeapi/song_naming.go` | List / rename / settings endpoints |
| | `ui/src/layout/NamingIssuesPanel.jsx` | The notification panel and one-click rename |
| | `ui/src/personal/NamingEnforcementToggle.jsx` | The Personal-settings switch |
| Tests | `core/gcpsync/gcpsync_test.go` | Drives real sweeps against a stub `gcloud` |
| | `core/gcpsync/naming_test.go` | The naming rules |

HTTP routes are mounted **admin-only** (`native_api.go` → `adminOnlyMiddleware`).

---

## 2. The core idea: the Sync folder is an inbox

`SyncFolder` is **not storage**. It is a staging area that should be empty
between runs. Every file that lands in it is claimed by exactly one rule and
moved out — to the bucket and the music folder, to the playlist folder, or to
the trash. Files still sitting there after a sweep mean something failed, and
the log line for that file says what.

Inside it, one subfolder carries special meaning:

```
<SyncFolder>/
├── mp3/          <- STAGING: "this file is processed, overwrite everything"
│   └── Artist/Album/Artist - Song.mp3
├── trash/        <- files the service refuses to sync
├── SomePlaylist.m3u
└── drop/         <- anything else: new material arriving from outside
    └── Artist - New.mp3
```

**`SyncFolder/mp3` is the marker for "the app processed this file."** It is the
whole basis of the overwrite rules in §4. It is a marker *on disk*, which is why
it survives a restart that wipes the in-memory upload queue.

---

## 3. Two mechanisms

### 3a. The event-driven queue (immediate)

When the app improves a file, the code path that made the change calls
`StageMP3` (`gcpsync.go:135`):

1. **Copy** the file into `SyncFolder/mp3/<path relative to MusicFolder>`.
   Copy, not move: the file stays at its library path because the library row,
   the loudness audit, and any in-flight playback all still point at it.

   > To be exact about *which* file that is: LUFS and the tag writers rewrite
   > the song **in place**, so by the time staging runs, the file sitting in the
   > music folder is already the **processed** one. Staging duplicates that
   > processed file; it never resurrects a pre-processing version. The
   > untouched original only exists in the LUFS backup folder, and only an
   > explicit restore puts it back.
2. `EnqueueMP3` (`gcpsync.go:106`) pushes the staged copy onto a buffered
   channel, de-duplicated by path through a `pending` set.
3. A background worker started at boot (`Start`) drains the channel one file at
   a time: upload (overwrite), then move the staged copy into the music folder,
   on top of the file it was made from.

   For an app-staged file this last move writes identical bytes over identical
   bytes — the processed file replacing itself — and its real job is clearing
   the staging folder. The move matters for files that arrive in `Sync/mp3`
   from anywhere else, which have no copy in the library yet. Either way, what
   ends up in the music folder is the processed audio.

The queue is in-memory (capacity 4096) and **does not survive a restart**. That
is fine — the staged copy is still on disk, so the next sweep finishes the job.

Two producers use it:

| Producer | Trigger |
|---|---|
| `song_loudness_run.go:130` | A track was LUFS-normalized (`res.Changed`) |
| `musicbrainz_metadata.go:1865` | Cover art embedded, or fetched tags written |

**Two passes over the same song.** A track is commonly improved twice in a row —
LUFS normalizes it, then cover art is embedded — and both passes stage it. Three
rules keep that safe:

- **Different songs co-exist.** Each staged copy keeps its path relative to the
  library root under `Sync/mp3`, so staging one never touches another. The root
  is passed in by the caller (`StageMP3(libraryRoot, path, reason)`) because a
  library need not sit under `MusicFolder`, and `relUnder` retries with symlinks
  resolved — a music folder reached through a symlink is configured as one
  string and stored in the library as another. When neither matches, staging
  falls back to the bare filename and logs a warning: that fallback flattens the
  folder, and two songs sharing a filename in different albums then overwrite
  each other there, one pass destroying the other's work before it is uploaded.
- **The same song overwrites.** `copyFile` writes to a temp file beside the
  destination and renames it into place. The replacement is atomic, so the
  upload worker reading that path sees either the previous complete file or the
  new one — never a half-written copy — and a copy that fails partway leaves the
  previous staged copy intact.
- **A superseded upload is retried, not dropped.** If the second pass lands
  while the first is still queued or mid-upload, the queue records it in
  `restaged` and runs it again once the in-flight upload finishes — re-staging
  from the *library* file, which by then carries both passes' changes. A plain
  de-duplicating set dropped that second request, leaving the bucket holding the
  older version with nothing left to correct it.

The temp files are dotfiles, and the sweep skips dotfiles entirely — sweeping
one would move a half-written file out from under the copy still writing it.
The staging folder itself is created at startup and is never removed when it
drains, so it stays visible as the marker it is.

### 3b. The sweep (batch)

`Sweep` (`gcpsync.go:446`) walks whatever is actually on disk in the Sync
folder. It doesn't care how a file got there, which covers the two cases the
queue can't:

- Files that arrived **from outside the app** — a published discovery folder, a
  mirrored playlist, anything copied in by hand.
- Files whose **queue entry died in a restart**.

A mutex means two sweeps never overlap, and any file currently held by the
upload worker is skipped (`isPending`) rather than raced for.

---

## 4. The rules

### Per file type

| File | Bucket | Local destination |
|---|---|---|
| `.mp3` **inside** `Sync/mp3` | **Always upload** (overwrite) | `MusicFolder/<rel to Sync/mp3>` |
| `.mp3` **outside** `Sync/mp3` | Upload **only if absent** from the bucket | `MusicFolder/<rel to SyncFolder>` |
| `.mp3` **not named `Artist - Title.mp3`** | **Never** | Held in the Sync folder (§4b) |
| `.m3u` | Always upload (overwrite) | `PlaylistsPath/<rel>`, previous version archived |
| anything else | **Never** | `DeletedPlaylistsPath/<rel to SyncFolder>` |

The two MP3 rows are the whole point. A file in staging was processed by the
app, so its bucket copy is stale by definition and must be replaced. A file
anywhere else is new material arriving from outside, so it is uploaded when the
bucket lacks it and left alone when the bucket already has it.

### Telling new from existing

`bucketObjects` (`gcpsync.go:343`) runs `gcloud storage ls gs://<bucket>/**`
**once per sweep** into a lookup set, so classifying N files costs one gcloud
round trip rather than N. It is skipped entirely when the batch holds no
unstaged MP3s to classify.

If the listing fails, new and existing cannot be told apart, so **no unstaged
MP3 is uploaded that sweep** and a warning is logged. Re-uploading the whole
library is the worse of the two mistakes; the next sweep retries. Staged files
still overwrite, since they need no classification.

### 4b. The naming gate

A track only reaches the devices if its **filename** is `Artist - Title.mp3`.
Playlists list their tracks as bare filenames and the bucket is flat, so an
oddly-named object is one no playlist entry can ever resolve. Uploading it does
not just waste a round trip — it puts a permanently unreachable object in the
bucket and hides the fact that the track is unplayable.

So a badly-named MP3 is **neither uploaded nor filed into the library**. It is
left in the Sync folder, where it stays visible and gets retried once the name
is fixed (`blockedByNaming`, `gcpsync.go`). This applies to both paths — the
event-driven queue and the sweep — and only to `.mp3`; playlists have their own
naming.

Two checks implement it, for the two callers that know different amounts
(`core/gcpsync/naming.go`):

| Function | Knows | Checks |
|---|---|---|
| `WellFormedName(path)` | just a filename | the *shape*: `something - something.mp3` |
| `Conforms(path, artist, title)` | the library row too | the name against the track's actual tags |

The sync service has no database handle, so it uses the shape check. The UI has
the row, so it reports the exact expected name and offers the rename.
`ExpectedName` sanitizes tag values first — `/`, `\` and `:` become `_`, control
characters become spaces, whitespace runs collapse — so `AC/DC` yields
`AC_DC - Back In Black.mp3` rather than a directory. A track missing an artist
or title tag is **not** reported: the fix there is tagging, not renaming.

**The switch:** Settings → Personal → *Require "Artist - Title.mp3" filenames*.
Unlike the icon toggles beside it this is not a display preference — it is
persisted server-side in the property table (`GCPSyncNamingEnforcement`), because
the server is what blocks the upload. It falls back to `GCPSync.EnforceNaming`
(default `true`) until the toggle is first used. Turning it off also hides the
notification icon: one switch, whole feature.

**The UI:** a notification icon in the top bar, beside Missing Tracks, badged
with the number of offending tracks (`ui/src/layout/NamingIssuesPanel.jsx`).
Each row carries the current filename, and:

- **left** — a play-style button that renames the track to `Artist - Title.mp3`
  in one click
- **right** — an info icon showing `current name ↓ new name`

A rename moves the file, updates the library row's path (`UpdatePath`) so the
scanner does not see a missing track plus a duplicate, and renames any staged
copy waiting in `Sync/mp3` so the track unblocks. It refuses to rename over an
existing file — two different songs sharing tags must not overwrite each other —
and rolls the file back if the database write fails, so disk and database never
disagree.

**Endpoints** (admin-only, `server/nativeapi/song_naming.go`):

| Route | Purpose |
|---|---|
| `GET /api/notifications/naming-issues` | List offending tracks (`limit`/`offset`, `X-Total-Count`) |
| `POST /api/song/naming/rename` | `{"ids":[…]}` → rename to the convention |
| `GET`/`PUT /api/song/naming/settings` | Read / set `{"enforced": bool}` |

The listing compares every track's filename against its tags, which is not a SQL
predicate, so the scan runs in Go and is cached for 30s; a rename drops the
cache.

### Naming: flat in the bucket, nested locally

These two pull in opposite directions and both matter:

- **Bucket object names are always the bare filename** (`objectName`,
  `gcpsync.go:324`), NFC-normalized. Playlists list their tracks as bare
  `Artist - Title.mp3` strings (`model/mediafile.go` → `ToM3U8`), so devices
  resolve every object at the bucket root. Nesting object names would point
  every playlist entry at nothing.
- **Local moves preserve the relative path** (`destRel`, `gcpsync.go:298`).
  Flattening here would drop a copy of every synced track at the library root,
  which the next scan indexes as a second track.

`libraryRel` (`gcpsync.go:310`) is the inverse used when staging, so a file
round-trips: `MusicFolder/A/B/x.mp3` → `Sync/mp3/A/B/x.mp3` → back to
`MusicFolder/A/B/x.mp3`.

> All internal paths are absolutized (`syncRoot`, `stagingRoot`, `musicRoot`)
> before comparison. `filepath.Rel` refuses to mix absolute and relative paths,
> and a mismatch there silently misclassifies every staged file.

---

## 5. What one sweep does, in order

1. **Bail** if `GCPSync.Enabled` is false or `Bucket` is empty.
2. **Take the sweep mutex** — no two sweeps overlap.
3. **NFC-normalize filenames** (`normalizeFilenamesNFC`, `gcpsync.go:580`),
   bottom-up so children are renamed before their parents. macOS and Linux
   disagree about how to encode accented characters; the bucket needs one
   spelling.
4. **Walk** the Sync folder into a sorted file list, skipping the trash folder.
5. **List the bucket** once, if there is anything unstaged to classify.
6. **Dispatch** each file per the table in §4, skipping files the upload worker
   currently holds.
7. **Remove empty directories** left behind.
8. **Log a tally**: `mp3sToMusicFolder`, `playlists`, `uploaded`,
   `alreadyInBucket`, `trashed`, `skippedPending`, `elapsed`.

---

## 6. Playlists

`processPlaylist` (`gcpsync.go:612`) does more than upload:

1. **Clean** — CR → space, content normalized to NFC (`cleanPlaylistFile`).
   Devices choke on CRLF.
2. **Upload** the cleaned file (overwrite).
3. **Version and place** it under `playlistDestRoot()` — the first entry of
   `PlaylistsPath` with any `**` glob suffix stripped:

| Destination state | What happens |
|---|---|
| Identical to incoming | Incoming copy archived to `LastVersionFolder/<rel>/<name>_<timestamp>.m3u` |
| Different | Old destination archived, incoming moved into its place |
| Does not exist | Incoming moved straight in |

If `PlaylistsPath` or `GCPSync.LastVersionFolder` is unset, the playlist is
uploaded but **left in the Sync folder** with a warning — versioning has nowhere
to go.

---

## 7. Trash

The library holds nothing but `.mp3` and `.m3u`, so anything else in the Sync
folder is junk. It is moved to **`DeletedPlaylistsPath`** — the folder the app
already empties deleted playlists into (`persistence/playlist_repository.go`) —
keeping its relative path, so the trash shows where each file came from. Only
when that option is unset does it fall back to `<SyncFolder>/trash`, so
trashing always has somewhere to go.

A name already taken in the trash gets a nanosecond-timestamp suffix rather
than being overwritten, matching how deleted playlists handle collisions. The
trash is where files go to be looked at later; silently replacing one defeats
the point.

This branch used to upload every non-MP3, non-M3U file unconditionally, which
let a stray file overwrite whatever bucket object shared its basename.

The sweep never descends into the trash — not in the walk, not in NFC
normalization, not in empty-dir cleanup. Without that, each run would nest the
previous run's trash one level deeper.

---

## 8. Configuration

| Option | Default | Meaning |
|---|---|---|
| `GCPSync.Enabled` | `false` | Master switch. Off means no worker, no sweep. |
| `GCPSync.Bucket` | `""` | Target bucket. Empty is treated as disabled. |
| `GCPSync.CredentialsFile` | `""` | Sets `GOOGLE_APPLICATION_CREDENTIALS` for `gcloud` |
| `GCPSync.GcloudPath` | `gcloud` | Path to the CLI |
| `GCPSync.Schedule` | `""` | Cron expression for the sweep. **Empty disables the schedule** — event-driven uploads and manual sweeps still work. |
| `GCPSync.DryRun` | `false` | Log every upload/move/rename without performing it |
| `GCPSync.LastVersionFolder` | `""` | Where superseded playlists are archived |
| `GCPSync.EnforceNaming` | `true` | Hold back MP3s not named `Artist - Title.mp3`. Overridden by the Personal-settings toggle once used. |

Plus four options the service reads but does not own: `SyncFolder` (the inbox),
`MusicFolder` (where MP3s land), `PlaylistsPath` (where M3Us land), and
`DeletedPlaylistsPath` (the shared trash).

Environment form: `ND_GCPSYNC_ENABLED`, `ND_GCPSYNC_BUCKET`, and so on.

---

## 9. Triggers

| Trigger | Path | Notes |
|---|---|---|
| Cron | `GCPSync.Schedule` → `cmd/root.go:261` | Registered at boot; empty schedule = no cron |
| UI "Sync" button | `GET /api/sync` | Returns `{"message": ...}`, which the activity panel shows as a toast |
| API | `POST /api/gcpsync/sweep` | Returns `202` + `{"status":"sweep started"}` |

**The two endpoints do the same thing.** Both call
`gcpsync.GetInstance().Sweep()` in a goroutine and return immediately. They
differ only in shape and audience:

- `GET /api/sync` is the **legacy path**, kept because the UI button already
  called it — it used to fire an external webhook that ran the bash script. It
  returns a human-readable `message` field because `ActivityPanel.jsx` pipes
  that string straight into a notification.
- `POST /api/gcpsync/sweep` is the **REST-correct one** for scripts and curl:
  POST for a side effect, `202 Accepted` for work that continues after the
  response.

Both are admin-only, and both return `503` when `GCPSync.Enabled` is false.

---

## 10. Safety properties

- **Idempotent.** Re-running a sweep on an empty Sync folder does nothing.
- **Serialized.** The sweep mutex and the `pending` set stop the batch pass and
  the upload worker from fighting over the same file.
- **Fail-soft.** A file that fails to upload is left where it is and picked up
  next sweep. One bad file never aborts the run.
- **Conservative under uncertainty.** A failed bucket listing suppresses
  unstaged uploads rather than guessing.
- **Bounded.** Every `gcloud` call is capped at `uploadTimeout` (5 minutes).
- **Cross-device safe.** `move` falls back to copy + remove when `os.Rename`
  crosses a filesystem boundary.
- **Dry-run.** `GCPSync.DryRun` logs every mutation without performing it.

---

## 11. Tests

`core/gcpsync/gcpsync_test.go` drives real sweeps against a stub `gcloud` shell
script that records invocations and answers `storage ls` with a canned listing:

| Test | Asserts |
|---|---|
| `TestSweepStagedMP3OverwritesAndKeepsItsPath` | Staged file overwrites even when present in the bucket, and lands nested, not flattened |
| `TestSweepExistingMP3IsNotUploadedAgain` | Unstaged + present in bucket → moved, never uploaded |
| `TestSweepNewMP3IsUploaded` | Unstaged + absent from bucket → uploaded and moved |
| `TestSweepTrashesUnsupportedFilesInsteadOfUploading` | A `.jpg` reaches the trash, never the bucket |
| `TestSweepDoesNotReprocessTrash` | Trash is not nested inside itself |
| `TestSweepTrashUsesDeletedPlaylistsPath` | Trash goes to the shared folder, not a second one |
| `TestSweepTrashDoesNotOverwriteExistingFile` | A name collision in the trash keeps both files |
| `TestSweepHoldsBadlyNamedMP3` | A misnamed track is neither uploaded nor filed into the library |
| `TestSweepUploadsBadlyNamedMP3WhenEnforcementDisabled` | The toggle actually lifts the gate |
| `TestSweepDoesNotHoldPlaylists` | The rule applies to tracks, not playlists |
| `TestStagedSongsCoexist` | A LUFS-staged and a cover-art-staged song both survive |
| `TestRestagingSameSongOverwrites` | Re-staging replaces the copy and leaves no temp files |
| `TestSupersededUploadIsRetriedNotDropped` | A second pass mid-upload is retried, not lost |
| `TestSweepIgnoresStagingTempFiles` | An in-flight temp copy is left alone |
| `TestSweepKeepsStagingFolder` | The staging folder survives a sweep that drains it |
| `TestStagedSongsWithSameFilenameCoexist` | Same filename in two albums does not collide in staging |
| `TestStagingResolvesSymlinkedMusicFolder` | A symlinked music folder does not flatten staging |
| `TestWellFormedName` / `TestExpectedNameSanitizesTagValues` / `TestConforms` | The naming rules themselves |
| `TestSweepSkipsUnstagedUploadsWhenBucketListingFails` | Listing failure suppresses unstaged uploads only |
| `TestStageMP3PreservesLibraryLayout` | Staged copy keeps its layout; original stays put |
| `TestObjectNameStaysFlat` | Bucket names stay bare filenames |

---

## 12. Known gaps and caveats

- **Device propagation is out of scope.** Nothing here pushes to or notifies
  playback devices. The service guarantees the bucket object is replaced;
  devices pick it up on their own poll cycle.
- **Staged copies can go stale.** If LUFS stages a track and the cover-art pass
  then writes to the library file before the sweep runs, the staged copy
  overwrites the newer file and the art is lost. This is the price of the
  unconditional overwrite that `Sync/mp3` is defined to mean. An mtime check in
  `moveToMusicFolder` would close it.
- **Basename collisions in the bucket.** Because bucket object names are flat,
  two tracks with the same filename in different folders still map to one
  object — a live and a studio cut of the same song, say. Staging now keeps them
  apart locally, but the bucket can hold only one, and the flat naming is
  required by the playlists. There is no fix short of changing how playlists
  reference tracks.
- **Case sensitivity.** `SyncFolder/MP3` (uppercase) is not recognized as the
  staging folder on a case-sensitive filesystem.
- **Config keys renamed.** These options were `gcsync.*` before; existing
  `gcsync.*` settings and `ND_GCSYNC_*` variables are silently ignored.
