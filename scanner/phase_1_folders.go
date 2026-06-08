package scanner

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/squirrel"
	ppl "github.com/google/go-pipeline/pkg/pipeline"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/metadata"
	"github.com/navidrome/navidrome/utils"
	"github.com/navidrome/navidrome/utils/pl"
	"github.com/navidrome/navidrome/utils/slice"
)

func createPhaseFolders(ctx context.Context, state *scanState, ds model.DataStore, cw artwork.CacheWarmer, libs []model.Library) *phaseFolders {
	var jobs []*scanJob
	var updatedLibs []model.Library
	for _, lib := range libs {
		if lib.LastScanStartedAt.IsZero() {
			err := ds.Library(ctx).ScanBegin(lib.ID, state.fullScan)
			if err != nil {
				log.Error(ctx, "Scanner: Error updating last scan started at", "lib", lib.Name, err)
				state.sendWarning(err.Error())
				continue
			}
			// Reload library to get updated state
			l, err := ds.Library(ctx).Get(lib.ID)
			if err != nil {
				log.Error(ctx, "Scanner: Error reloading library", "lib", lib.Name, err)
				state.sendWarning(err.Error())
				continue
			}
			lib = *l
		} else {
			log.Debug(ctx, "Scanner: Resuming previous scan", "lib", lib.Name, "lastScanStartedAt", lib.LastScanStartedAt, "fullScan", lib.FullScanInProgress)
		}
		job, err := newScanJob(ctx, ds, cw, lib, state.fullScan)
		if err != nil {
			log.Error(ctx, "Scanner: Error creating scan context", "lib", lib.Name, err)
			state.sendWarning(err.Error())
			continue
		}
		jobs = append(jobs, job)
		updatedLibs = append(updatedLibs, lib)
	}

	// Update the state with the libraries that have been processed and have their scan timestamps set
	state.libraries = updatedLibs

	return &phaseFolders{
		jobs:            jobs,
		ctx:             ctx,
		ds:              ds,
		state:           state,
		loudnessLimiter: make(chan struct{}, configuredLoudnessParallelism(conf.Server.Scanner.LoudnessNormalization.Parallelism)),
	}
}

type scanJob struct {
	lib         model.Library
	fs          storage.MusicFS
	cw          artwork.CacheWarmer
	lastUpdates map[string]model.FolderUpdateInfo
	lock        sync.Mutex
	numFolders  atomic.Int64
}

func newScanJob(ctx context.Context, ds model.DataStore, cw artwork.CacheWarmer, lib model.Library, fullScan bool) (*scanJob, error) {
	lastUpdates, err := ds.Folder(ctx).GetLastUpdates(lib)
	if err != nil {
		return nil, fmt.Errorf("getting last updates: %w", err)
	}
	fileStore, err := storage.For(lib.Path)
	if err != nil {
		log.Error(ctx, "Error getting storage for library", "library", lib.Name, "path", lib.Path, err)
		return nil, fmt.Errorf("getting storage for library: %w", err)
	}
	fsys, err := fileStore.FS()
	if err != nil {
		log.Error(ctx, "Error getting fs for library", "library", lib.Name, "path", lib.Path, err)
		return nil, fmt.Errorf("getting fs for library: %w", err)
	}
	lib.FullScanInProgress = lib.FullScanInProgress || fullScan
	return &scanJob{
		lib:         lib,
		fs:          fsys,
		cw:          cw,
		lastUpdates: lastUpdates,
	}, nil
}

func (j *scanJob) popLastUpdate(folderID string) model.FolderUpdateInfo {
	j.lock.Lock()
	defer j.lock.Unlock()

	lastUpdate := j.lastUpdates[folderID]
	delete(j.lastUpdates, folderID)
	return lastUpdate
}

// phaseFolders represents the first phase of the scanning process, which is responsible
// for scanning all libraries and importing new or updated files. This phase involves
// traversing the directory tree of each library, identifying new or modified media files,
// and updating the database with the relevant information.
//
// The phaseFolders struct holds the context, data store, and jobs required for the scanning
// process. Each job represents a library being scanned, and contains information about the
// library, file system, and the last updates of the folders.
//
// The phaseFolders struct implements the phase interface, providing methods to produce
// folder entries, process folders, persist changes to the database, and log the results.
type phaseFolders struct {
	jobs             []*scanJob
	ds               model.DataStore
	ctx              context.Context
	state            *scanState
	prevAlbumPIDConf string
	loudnessLimiter  chan struct{}
}

func (p *phaseFolders) description() string {
	return "Scan all libraries and import new/updated files"
}

func (p *phaseFolders) producer() ppl.Producer[*folderEntry] {
	return ppl.NewProducer(func(put func(entry *folderEntry)) error {
		var err error
		p.prevAlbumPIDConf, err = p.ds.Property(p.ctx).DefaultGet(consts.PIDAlbumKey, "")
		if err != nil {
			return fmt.Errorf("getting album PID conf: %w", err)
		}

		// TODO Parallelize multiple job when we have multiple libraries
		var total int64
		var totalChanged int64
		for _, job := range p.jobs {
			if utils.IsCtxDone(p.ctx) {
				break
			}
			outputChan, err := walkDirTree(p.ctx, job)
			if err != nil {
				log.Warn(p.ctx, "Scanner: Error scanning library", "lib", job.lib.Name, err)
			}
			for folder := range pl.ReadOrDone(p.ctx, outputChan) {
				job.numFolders.Add(1)
				p.state.sendProgress(&ProgressInfo{
					LibID:     job.lib.ID,
					FileCount: uint32(len(folder.audioFiles)),
					Path:      folder.path,
					Phase:     "1",
				})

				// Log folder info
				log.Trace(p.ctx, "Scanner: Checking folder state", " folder", folder.path, "_updTime", folder.updTime,
					"_modTime", folder.modTime, "_lastScanStartedAt", folder.job.lib.LastScanStartedAt,
					"numAudioFiles", len(folder.audioFiles), "numImageFiles", len(folder.imageFiles),
					"numPlaylists", folder.numPlaylists, "numSubfolders", folder.numSubFolders)

				// Check if folder is outdated
				if folder.isOutdated() {
					if !p.state.fullScan {
						if folder.hasNoFiles() && folder.isNew() {
							log.Trace(p.ctx, "Scanner: Skipping new folder with no files", "folder", folder.path, "lib", job.lib.Name)
							continue
						}
						log.Debug(p.ctx, "Scanner: Detected changes in folder", "folder", folder.path, "lastUpdate", folder.modTime, "lib", job.lib.Name)
					}
					totalChanged++
					folder.elapsed.Stop()
					put(folder)
				} else {
					log.Trace(p.ctx, "Scanner: Skipping up-to-date folder", "folder", folder.path, "lastUpdate", folder.modTime, "lib", job.lib.Name)
				}
			}
			total += job.numFolders.Load()
		}
		log.Debug(p.ctx, "Scanner: Finished loading all folders", "numFolders", total, "numChanged", totalChanged)
		return nil
	}, ppl.Name("traverse filesystem"))
}

func (p *phaseFolders) measure(entry *folderEntry) func() time.Duration {
	entry.elapsed.Start()
	return func() time.Duration { return entry.elapsed.Stop() }
}

func (p *phaseFolders) stages() []ppl.Stage[*folderEntry] {
	return []ppl.Stage[*folderEntry]{
		ppl.NewStage(p.processFolder, ppl.Name("process folder"), ppl.Concurrency(conf.Server.DevScannerThreads)),
		ppl.NewStage(p.persistChanges, ppl.Name("persist changes")),
		ppl.NewStage(p.logFolder, ppl.Name("log results")),
	}
}

func (p *phaseFolders) processFolder(entry *folderEntry) (*folderEntry, error) {
	defer p.measure(entry)()

	// Load children mediafiles from DB
	cursor, err := p.ds.MediaFile(p.ctx).GetCursor(model.QueryOptions{
		Filters: squirrel.And{squirrel.Eq{"folder_id": entry.id}},
	})
	if err != nil {
		log.Error(p.ctx, "Scanner: Error loading mediafiles from DB", "folder", entry.path, err)
		return entry, err
	}
	dbTracks := make(map[string]*model.MediaFile)
	for mf, err := range cursor {
		if err != nil {
			log.Error(p.ctx, "Scanner: Error loading mediafiles from DB", "folder", entry.path, err)
			return entry, err
		}
		dbTracks[mf.Path] = &mf
	}

	// Get list of files to import, based on modtime (or all if fullScan),
	// leave in dbTracks only tracks that are missing (not found in the FS)
	filesToImport := make(map[string]*model.MediaFile, len(entry.audioFiles))
	for afPath, af := range entry.audioFiles {
		fullPath := path.Join(entry.path, afPath)
		dbTrack, foundInDB := dbTracks[fullPath]
		if !foundInDB || p.state.fullScan {
			filesToImport[fullPath] = dbTrack
		} else {
			info, err := af.Info()
			if err != nil {
				log.Warn(p.ctx, "Scanner: Error getting file info", "folder", entry.path, "file", af.Name(), err)
				p.state.sendWarning(fmt.Sprintf("Error getting file info for %s/%s: %v", entry.path, af.Name(), err))
				return entry, nil
			}
			if info.ModTime().After(dbTrack.UpdatedAt) || dbTrack.Missing {
				filesToImport[fullPath] = dbTrack
			}
		}
		delete(dbTracks, fullPath)
	}

	// Remaining dbTracks are tracks that were not found in the FS, so they should be marked as missing
	entry.missingTracks = slices.Collect(maps.Values(dbTracks))

	// Normalize changed audio files before reading metadata so persisted audio properties match the final file.
	if len(filesToImport) > 0 {
		lufsByFile := p.normalizeLoudnessFiles(entry, filesToImport)

		err = p.loadTagsFromFiles(entry, filesToImport, lufsByFile)
		if err != nil {
			log.Warn(p.ctx, "Scanner: Error loading tags from files. Skipping", "folder", entry.path, err)
			p.state.sendWarning(fmt.Sprintf("Error loading tags from files in %s: %v", entry.path, err))
			return entry, nil
		}

		p.createAlbumsFromMediaFiles(entry)
		p.createArtistsFromMediaFiles(entry)
	}

	return entry, nil
}

const filesBatchSize = 200

// loadTagsFromFiles reads metadata from the files in the given list and populates
// the entry's tracks and tags with the results.
func (p *phaseFolders) loadTagsFromFiles(entry *folderEntry, toImport map[string]*model.MediaFile, lufsByFile map[string]float64) error {
	tracks := make([]model.MediaFile, 0, len(toImport))
	uniqueTags := make(map[string]model.Tag, len(toImport))
	for chunk := range slice.CollectChunks(maps.Keys(toImport), filesBatchSize) {
		allInfo, err := entry.job.fs.ReadTags(chunk...)
		if err != nil {
			log.Warn(p.ctx, "Scanner: Error extracting metadata from files. Skipping", "folder", entry.path, err)
			return err
		}
		for filePath, info := range allInfo {
			md := metadata.New(filePath, info)
			track := md.ToMediaFile(entry.job.lib.ID, entry.id)
			if lufs, ok := lufsByFile[filePath]; ok {
				if track.Tags == nil {
					track.Tags = model.Tags{}
				}
				track.Tags[model.TagName("loudnorm_final_lufs")] = []string{strconv.FormatFloat(lufs, 'f', 2, 64)}
			}
			tracks = append(tracks, track)
			for _, t := range track.Tags.FlattenAll() {
				uniqueTags[t.ID] = t
			}

			// Keep track of any album ID changes, to reassign annotations later
			prevAlbumID := ""
			if prev := toImport[filePath]; prev != nil {
				prevAlbumID = prev.AlbumID
			} else {
				prevAlbumID = md.AlbumID(track, p.prevAlbumPIDConf)
			}
			_, ok := entry.albumIDMap[track.AlbumID]
			if prevAlbumID != track.AlbumID && !ok {
				entry.albumIDMap[track.AlbumID] = prevAlbumID
			}
		}
	}
	entry.tracks = tracks
	entry.tags = slices.Collect(maps.Values(uniqueTags))
	return nil
}

// createAlbumsFromMediaFiles groups the entry's tracks by album ID and creates albums
func (p *phaseFolders) createAlbumsFromMediaFiles(entry *folderEntry) {
	grouped := slice.Group(entry.tracks, func(mf model.MediaFile) string { return mf.AlbumID })
	albums := make(model.Albums, 0, len(grouped))
	for _, group := range grouped {
		songs := model.MediaFiles(group)
		album := songs.ToAlbum()
		albums = append(albums, album)
	}
	entry.albums = albums
}

// createArtistsFromMediaFiles creates artists from the entry's tracks
func (p *phaseFolders) createArtistsFromMediaFiles(entry *folderEntry) {
	participants := make(model.Participants, len(entry.tracks)*3) // preallocate ~3 artists per track
	for _, track := range entry.tracks {
		participants.Merge(track.Participants)
	}
	entry.artists = participants.AllArtists()
}

func (p *phaseFolders) persistChanges(entry *folderEntry) (*folderEntry, error) {
	defer p.measure(entry)()
	p.state.changesDetected.Store(true)

	err := p.ds.WithTx(func(tx model.DataStore) error {
		// Instantiate all repositories just once per folder
		folderRepo := tx.Folder(p.ctx)
		tagRepo := tx.Tag(p.ctx)
		artistRepo := tx.Artist(p.ctx)
		libraryRepo := tx.Library(p.ctx)
		albumRepo := tx.Album(p.ctx)
		mfRepo := tx.MediaFile(p.ctx)

		// Save folder to DB
		folder := entry.toFolder()
		err := folderRepo.Put(folder)
		if err != nil {
			log.Error(p.ctx, "Scanner: Error persisting folder to DB", "folder", entry.path, err)
			return err
		}

		// Save all tags to DB
		err = tagRepo.Add(entry.job.lib.ID, entry.tags...)
		if err != nil {
			log.Error(p.ctx, "Scanner: Error persisting tags to DB", "folder", entry.path, err)
			return err
		}

		// Save all new/modified artists to DB. Their information will be incomplete, but they will be refreshed later
		for i := range entry.artists {
			err = artistRepo.Put(&entry.artists[i], "name",
				"mbz_artist_id", "sort_artist_name", "order_artist_name", "full_text", "updated_at")
			if err != nil {
				log.Error(p.ctx, "Scanner: Error persisting artist to DB", "folder", entry.path, "artist", entry.artists[i].Name, err)
				return err
			}
			err = libraryRepo.AddArtist(entry.job.lib.ID, entry.artists[i].ID)
			if err != nil {
				log.Error(p.ctx, "Scanner: Error adding artist to library", "lib", entry.job.lib.ID, "artist", entry.artists[i].Name, err)
				return err
			}
			if entry.artists[i].Name != consts.UnknownArtist && entry.artists[i].Name != consts.VariousArtists {
				entry.job.cw.PreCache(entry.artists[i].CoverArtID())
			}
		}

		// Save all new/modified albums to DB. Their information will be incomplete, but they will be refreshed later
		for i := range entry.albums {
			err = p.persistAlbum(albumRepo, &entry.albums[i], entry.albumIDMap)
			if err != nil {
				log.Error(p.ctx, "Scanner: Error persisting album to DB", "folder", entry.path, "album", entry.albums[i], err)
				return err
			}
			if entry.albums[i].Name != consts.UnknownAlbum {
				entry.job.cw.PreCache(entry.albums[i].CoverArtID())
			}
		}

		// Save all tracks to DB
		for i := range entry.tracks {
			err = mfRepo.Put(&entry.tracks[i])
			if err != nil {
				log.Error(p.ctx, "Scanner: Error persisting mediafile to DB", "folder", entry.path, "track", entry.tracks[i], err)
				return err
			}
		}

		// Mark all missing tracks as not available
		if len(entry.missingTracks) > 0 {
			err = mfRepo.MarkMissing(true, entry.missingTracks...)
			if err != nil {
				log.Error(p.ctx, "Scanner: Error marking missing tracks", "folder", entry.path, err)
				return err
			}

			// Touch all albums that have missing tracks, so they get refreshed in later phases
			groupedMissingTracks := slice.ToMap(entry.missingTracks, func(mf *model.MediaFile) (string, struct{}) {
				return mf.AlbumID, struct{}{}
			})
			albumsToUpdate := slices.Collect(maps.Keys(groupedMissingTracks))
			err = albumRepo.Touch(albumsToUpdate...)
			if err != nil {
				log.Error(p.ctx, "Scanner: Error touching album", "folder", entry.path, "albums", albumsToUpdate, err)
				return err
			}
		}
		return nil
	}, "scanner: persist changes")
	if err != nil {
		log.Error(p.ctx, "Scanner: Error persisting changes to DB", "folder", entry.path, err)
	}
	return entry, err
}

// persistAlbum persists the given album to the database, and reassigns annotations from the previous album ID
func (p *phaseFolders) persistAlbum(repo model.AlbumRepository, a *model.Album, idMap map[string]string) error {
	prevID := idMap[a.ID]
	log.Trace(p.ctx, "Persisting album", "album", a.Name, "albumArtist", a.AlbumArtist, "id", a.ID, "prevID", cmp.Or(prevID, "nil"))
	if err := repo.Put(a); err != nil {
		return fmt.Errorf("persisting album %s: %w", a.ID, err)
	}
	if prevID == "" {
		return nil
	}

	// Reassign annotation from previous album to new album
	log.Trace(p.ctx, "Reassigning album annotations", "from", prevID, "to", a.ID, "album", a.Name)
	if err := repo.ReassignAnnotation(prevID, a.ID); err != nil {
		log.Warn(p.ctx, "Scanner: Could not reassign annotations", "from", prevID, "to", a.ID, "album", a.Name, err)
		p.state.sendWarning(fmt.Sprintf("Could not reassign annotations from %s to %s ('%s'): %v", prevID, a.ID, a.Name, err))
	}

	// Keep created_at field from previous instance of the album
	if err := repo.CopyAttributes(prevID, a.ID, "created_at"); err != nil {
		// Silently ignore when the previous album is not found
		if !errors.Is(err, model.ErrNotFound) {
			log.Warn(p.ctx, "Scanner: Could not copy fields", "from", prevID, "to", a.ID, "album", a.Name, err)
			p.state.sendWarning(fmt.Sprintf("Could not copy fields from %s to %s ('%s'): %v", prevID, a.ID, a.Name, err))
		}
	}
	// Don't keep track of this mapping anymore
	delete(idMap, a.ID)
	return nil
}

func (p *phaseFolders) logFolder(entry *folderEntry) (*folderEntry, error) {
	logCall := log.Info
	if entry.isEmpty() {
		logCall = log.Trace
	}
	logCall(p.ctx, "Scanner: Completed processing folder",
		"audioCount", len(entry.audioFiles), "imageCount", len(entry.imageFiles), "plsCount", entry.numPlaylists,
		"elapsed", entry.elapsed.Elapsed(), "tracksMissing", len(entry.missingTracks),
		"tracksImported", len(entry.tracks), "library", entry.job.lib.Name, consts.Zwsp+"folder", entry.path)
	return entry, nil
}

const (
	maxLoudnessNormalizeAttempts = 3
	closeLoudnessMissLUFS        = 1.0
	minLoudnessImprovementLUFS   = 0.02
)

type loudnessFileResult struct {
	filePath string
	lufs     float64
	ok       bool
}

func (p *phaseFolders) normalizeLoudnessFiles(entry *folderEntry, filesToImport map[string]*model.MediaFile) map[string]float64 {
	options := conf.Server.Scanner.LoudnessNormalization
	if !options.Enabled || len(filesToImport) == 0 {
		return nil
	}

	lufsByFile := map[string]float64{}

	target := ffmpeg.LoudnessTarget{
		IntegratedLUFS: options.TargetLUFS,
		TruePeak:       options.TruePeak,
		LRA:            options.LRA,
	}
	tolerance := effectiveLoudnessTolerance(options.Tolerance)
	libraryPath := filepath.Clean(entry.job.lib.Path)
	minLUFS := options.TargetLUFS - tolerance
	maxLUFS := options.TargetLUFS + tolerance
	parallelism := effectiveLoudnessParallelism(options.Parallelism, len(filesToImport))
	log.Info(p.ctx, "Scanner: checking track loudness", "tracks", len(filesToImport), "parallelism", parallelism, "targetLUFS", options.TargetLUFS, "tolerance", tolerance, "minLUFS", minLUFS, "maxLUFS", maxLUFS, "library", entry.job.lib.Name, consts.Zwsp+"folder", entry.path)

	files := make(chan string)
	results := make(chan loudnessFileResult)
	var wg sync.WaitGroup
	wg.Add(parallelism)
	for range parallelism {
		go func() {
			defer wg.Done()
			normalizer := ffmpeg.NewLoudnessNormalizer()
			for filePath := range files {
				trackPath := absoluteMediaPath(libraryPath, filePath)
				p.loudnessLimiter <- struct{}{}
				result := p.normalizeLoudnessFile(normalizer, libraryPath, filePath, trackPath, target, tolerance, minLUFS, maxLUFS, options.TargetLUFS, options.Backup, options.BackupSuffix)
				<-p.loudnessLimiter
				results <- result
			}
		}()
	}

	go func() {
		for filePath := range filesToImport {
			files <- filePath
		}
		close(files)
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.ok {
			lufsByFile[result.filePath] = result.lufs
		}
	}

	return lufsByFile
}

func (p *phaseFolders) normalizeLoudnessFile(normalizer ffmpeg.LoudnessNormalizer, libraryPath, filePath, trackPath string, target ffmpeg.LoudnessTarget, tolerance, minLUFS, maxLUFS, targetLUFS float64, backup bool, backupSuffix string) loudnessFileResult {
	analysis, err := normalizer.AnalyzeLoudness(p.ctx, trackPath, target)
	if err != nil {
		log.Warn(p.ctx, "Scanner: could not analyze track loudness", "path", trackPath, err)
		p.state.sendWarning(fmt.Sprintf("Could not analyze track loudness for %s: %v", trackPath, err))
		return loudnessFileResult{filePath: filePath}
	}
	if !shouldNormalizeLoudness(analysis.InputIntegrated, targetLUFS, tolerance) {
		log.Debug(p.ctx, "Scanner: track loudness already in target range", "path", trackPath, "lufs", analysis.InputIntegrated, "minLUFS", minLUFS, "maxLUFS", maxLUFS)
		return loudnessFileResult{filePath: filePath, lufs: analysis.InputIntegrated, ok: true}
	}

	if finalLUFS, ok := p.normalizeTrackLoudnessToRange(normalizer, trackPath, target, *analysis, tolerance, minLUFS, maxLUFS, backup, backupSuffix); ok {
		if err := copyLoudnessUpdatedTrackToSyncFolder(libraryPath, filePath, trackPath); err != nil {
			log.Warn(p.ctx, "Scanner: could not copy LUFS-updated track to sync folder", "path", trackPath, "syncFolder", conf.Server.SyncFolder, err)
			p.state.sendWarning(fmt.Sprintf("Could not copy LUFS-updated track to sync folder for %s: %v", trackPath, err))
		}
		return loudnessFileResult{filePath: filePath, lufs: finalLUFS, ok: true}
	}
	return loudnessFileResult{filePath: filePath}
}

func (p *phaseFolders) normalizeTrackLoudnessToRange(normalizer ffmpeg.LoudnessNormalizer, trackPath string, target ffmpeg.LoudnessTarget, analysis ffmpeg.LoudnessAnalysis, tolerance, minLUFS, maxLUFS float64, backup bool, backupSuffix string) (float64, bool) {
	fromLUFS := analysis.InputIntegrated
	attemptAnalysis := analysis
	attemptTarget := target
	previousDistance := math.Abs(analysis.InputIntegrated - target.IntegratedLUFS)
	if math.Abs(target.IntegratedLUFS-analysis.InputIntegrated) <= closeLoudnessMissLUFS {
		var err error
		attemptTarget = adjustedLoudnessTarget(target, analysis.InputIntegrated, minLUFS, maxLUFS)
		log.Debug(p.ctx, "Scanner: using adjusted loudness normalization target", "path", trackPath, "fromLUFS", analysis.InputIntegrated, "fromTruePeak", analysis.InputTruePeak, "targetLUFS", target.IntegratedLUFS, "targetTruePeak", target.TruePeak, "attemptTargetLUFS", attemptTarget.IntegratedLUFS)
		attemptAnalysis, err = analyzeLoudnessForTarget(p.ctx, normalizer, trackPath, target, attemptTarget, 1)
		if err != nil {
			p.state.sendWarning(fmt.Sprintf("Could not analyze track loudness for adjusted target for %s: %v", trackPath, err))
			return 0, false
		}
	}
	for attempt := 1; attempt <= maxLoudnessNormalizeAttempts; attempt++ {
		if err := normalizeTrackLoudness(p.ctx, normalizer, trackPath, attemptTarget, attemptAnalysis, backup, backupSuffix); err != nil {
			log.Warn(p.ctx, "Scanner: could not normalize track loudness", "path", trackPath, "lufs", attemptAnalysis.InputIntegrated, "target", target.IntegratedLUFS, "attemptTarget", attemptTarget.IntegratedLUFS, "attempt", attempt, err)
			p.state.sendWarning(fmt.Sprintf("Could not normalize track loudness for %s: %v", trackPath, err))
			return 0, false
		}

		finalAnalysis, err := normalizer.AnalyzeLoudness(p.ctx, trackPath, target)
		if err != nil {
			log.Warn(p.ctx, "Scanner: normalized track loudness but could not verify final LUFS", "path", trackPath, "fromLUFS", fromLUFS, "targetLUFS", target.IntegratedLUFS, "attempt", attempt, err)
			p.state.sendWarning(fmt.Sprintf("Could not verify normalized track loudness for %s: %v", trackPath, err))
			return 0, false
		}
		log.Info(p.ctx, "Scanner: normalized track loudness", "path", trackPath, "fromLUFS", fromLUFS, "finalLUFS", finalAnalysis.InputIntegrated, "finalTruePeak", finalAnalysis.InputTruePeak, "targetLUFS", target.IntegratedLUFS, "targetTruePeak", target.TruePeak, "minLUFS", minLUFS, "maxLUFS", maxLUFS, "attempt", attempt)
		if !shouldNormalizeLoudness(finalAnalysis.InputIntegrated, target.IntegratedLUFS, tolerance) {
			return finalAnalysis.InputIntegrated, true
		}
		currentDistance := math.Abs(finalAnalysis.InputIntegrated - target.IntegratedLUFS)
		if isAdjustedLoudnessTarget(target, attemptTarget) && currentDistance > previousDistance-minLoudnessImprovementLUFS {
			log.Warn(p.ctx, "Scanner: normalized track loudness did not improve enough", "path", trackPath, "fromLUFS", fromLUFS, "finalLUFS", finalAnalysis.InputIntegrated, "finalTruePeak", finalAnalysis.InputTruePeak, "targetLUFS", target.IntegratedLUFS, "targetTruePeak", target.TruePeak, "minLUFS", minLUFS, "maxLUFS", maxLUFS, "attempt", attempt)
			p.state.sendWarning(fmt.Sprintf("Normalized track loudness did not improve enough for %s: %.2f LUFS (wanted %.2f to %.2f)", trackPath, finalAnalysis.InputIntegrated, minLUFS, maxLUFS))
			return finalAnalysis.InputIntegrated, true
		}
		if attempt == maxLoudnessNormalizeAttempts {
			attemptAnalysis = *finalAnalysis
			break
		}
		previousDistance = currentDistance
		attemptTarget = adjustedLoudnessTarget(target, finalAnalysis.InputIntegrated, minLUFS, maxLUFS)
		log.Debug(p.ctx, "Scanner: adjusting loudness normalization target", "path", trackPath, "finalLUFS", finalAnalysis.InputIntegrated, "finalTruePeak", finalAnalysis.InputTruePeak, "targetLUFS", target.IntegratedLUFS, "targetTruePeak", target.TruePeak, "attemptTargetLUFS", attemptTarget.IntegratedLUFS, "attempt", attempt+1)
		attemptAnalysis, err = analyzeLoudnessForTarget(p.ctx, normalizer, trackPath, target, attemptTarget, attempt+1)
		if err != nil {
			p.state.sendWarning(fmt.Sprintf("Could not analyze track loudness for adjusted target for %s: %v", trackPath, err))
			return 0, false
		}
	}

	log.Warn(p.ctx, "Scanner: normalized track loudness outside target range", "path", trackPath, "finalLUFS", attemptAnalysis.InputIntegrated, "minLUFS", minLUFS, "maxLUFS", maxLUFS, "attempts", maxLoudnessNormalizeAttempts)
	p.state.sendWarning(fmt.Sprintf("Normalized track loudness outside target range for %s: %.2f LUFS (wanted %.2f to %.2f)", trackPath, attemptAnalysis.InputIntegrated, minLUFS, maxLUFS))
	return attemptAnalysis.InputIntegrated, true
}

func effectiveLoudnessTolerance(tolerance float64) float64 {
	if tolerance <= 0 {
		return conf.DefaultLoudnessNormalizationTolerance
	}
	return tolerance
}

func effectiveLoudnessParallelism(parallelism, fileCount int) int {
	parallelism = configuredLoudnessParallelism(parallelism)
	if fileCount <= 1 {
		return 1
	}
	return min(parallelism, fileCount)
}

func configuredLoudnessParallelism(parallelism int) int {
	if parallelism <= 0 {
		return 1
	}
	return parallelism
}

func adjustedLoudnessTarget(target ffmpeg.LoudnessTarget, measuredLUFS, minLUFS, maxLUFS float64) ffmpeg.LoudnessTarget {
	target.IntegratedLUFS += target.IntegratedLUFS - measuredLUFS
	target.IntegratedLUFS = min(max(target.IntegratedLUFS, minLUFS), maxLUFS)
	return target
}

func isAdjustedLoudnessTarget(target, attemptTarget ffmpeg.LoudnessTarget) bool {
	return attemptTarget.IntegratedLUFS != target.IntegratedLUFS
}

func analyzeLoudnessForTarget(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, trackPath string, target, attemptTarget ffmpeg.LoudnessTarget, attempt int) (ffmpeg.LoudnessAnalysis, error) {
	analysis, err := normalizer.AnalyzeLoudness(ctx, trackPath, attemptTarget)
	if err != nil {
		log.Warn(ctx, "Scanner: could not analyze track loudness for adjusted target", "path", trackPath, "targetLUFS", target.IntegratedLUFS, "attemptTargetLUFS", attemptTarget.IntegratedLUFS, "attempt", attempt, err)
		return ffmpeg.LoudnessAnalysis{}, err
	}
	return *analysis, nil
}

func shouldNormalizeLoudness(lufs, targetLUFS, tolerance float64) bool {
	return math.Abs(lufs-targetLUFS) > tolerance
}

func absoluteMediaPath(libraryPath, mediaPath string) string {
	trackPath := filepath.FromSlash(mediaPath)
	if !filepath.IsAbs(trackPath) {
		trackPath = filepath.Join(libraryPath, trackPath)
	}
	return filepath.Clean(trackPath)
}

func normalizeTrackLoudness(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, trackPath string, target ffmpeg.LoudnessTarget, analysis ffmpeg.LoudnessAnalysis, backup bool, backupSuffix string) error {
	stat, err := os.Stat(trackPath)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(trackPath), "."+trimExt(filepath.Base(trackPath))+".loudnorm-*.tmp"+filepath.Ext(trackPath))
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Remove(tmpPath); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	if err := normalizer.NormalizeLoudness(ctx, trackPath, tmpPath, target, analysis); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, stat.Mode()); err != nil {
		return err
	}
	if backup {
		if backupSuffix == "" {
			backupSuffix = ".before_loudnorm"
		}
		backupPath := trackPath + backupSuffix
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := copyFile(trackPath, backupPath, stat); err != nil {
				return fmt.Errorf("creating loudness backup: %w", err)
			}
		} else if err != nil {
			return err
		}
	}
	return os.Rename(tmpPath, trackPath)
}

func copyFile(srcPath, dstPath string, stat os.FileInfo) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, stat.Mode())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(dstPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dstPath)
		return closeErr
	}
	return os.Chtimes(dstPath, stat.ModTime(), stat.ModTime())
}

func copyLoudnessUpdatedTrackToSyncFolder(libraryPath, filePath, trackPath string) error {
	if conf.Server.SyncFolder == "" {
		return nil
	}
	stat, err := os.Stat(trackPath)
	if err != nil {
		return err
	}
	dstPath := loudnessSyncPath(libraryPath, filePath, trackPath)
	srcAbs, srcErr := filepath.Abs(trackPath)
	dstAbs, dstErr := filepath.Abs(dstPath)
	if srcErr == nil && dstErr == nil && filepath.Clean(srcAbs) == filepath.Clean(dstAbs) {
		return nil
	}
	return copyFileReplace(trackPath, dstPath, stat)
}

func loudnessSyncPath(libraryPath, filePath, trackPath string) string {
	rel := filepath.FromSlash(filePath)
	if filepath.IsAbs(rel) {
		rel = filepath.Base(rel)
		if libraryPath != "" {
			if r, err := filepath.Rel(libraryPath, trackPath); err == nil && r != "." && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
	}
	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(trackPath)
	}
	return filepath.Join(conf.Server.SyncFolder, rel)
}

func copyFileReplace(srcPath, dstPath string, stat os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dstPath), "."+filepath.Base(dstPath)+".sync-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	src, err := os.Open(srcPath)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	_, copyErr := io.Copy(tmp, src)
	closeSrcErr := src.Close()
	closeTmpErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeSrcErr != nil {
		return closeSrcErr
	}
	if closeTmpErr != nil {
		return closeTmpErr
	}
	if err := os.Chmod(tmpPath, stat.Mode()); err != nil {
		return err
	}
	if err := os.Chtimes(tmpPath, stat.ModTime(), stat.ModTime()); err != nil {
		return err
	}
	return os.Rename(tmpPath, dstPath)
}

func trimExt(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}

func (p *phaseFolders) finalize(err error) error {
	errF := p.ds.WithTx(func(tx model.DataStore) error {
		for _, job := range p.jobs {
			// Mark all folders that were not updated as missing
			if len(job.lastUpdates) == 0 {
				continue
			}
			folderIDs := slices.Collect(maps.Keys(job.lastUpdates))
			err := tx.Folder(p.ctx).MarkMissing(true, folderIDs...)
			if err != nil {
				log.Error(p.ctx, "Scanner: Error marking missing folders", "lib", job.lib.Name, err)
				return err
			}
			err = tx.MediaFile(p.ctx).MarkMissingByFolder(true, folderIDs...)
			if err != nil {
				log.Error(p.ctx, "Scanner: Error marking tracks in missing folders", "lib", job.lib.Name, err)
				return err
			}
			// Touch all albums that have missing folders, so they get refreshed in later phases
			_, err = tx.Album(p.ctx).TouchByMissingFolder()
			if err != nil {
				log.Error(p.ctx, "Scanner: Error touching albums with missing folders", "lib", job.lib.Name, err)
				return err
			}
		}
		return nil
	}, "scanner: finalize phaseFolders")
	return errors.Join(err, errF)
}

var _ phase[*folderEntry] = (*phaseFolders)(nil)
