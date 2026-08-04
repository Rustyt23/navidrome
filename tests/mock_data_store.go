package tests

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/navidrome/navidrome/model"
)

type MockDataStore struct {
	RealDS                          model.DataStore
	MockedLibrary                   model.LibraryRepository
	MockedFolder                    model.FolderRepository
	MockedGenre                     model.GenreRepository
	MockedAlbum                     model.AlbumRepository
	MockedArtist                    model.ArtistRepository
	MockedMediaFile                 model.MediaFileRepository
	MockedTag                       model.TagRepository
	MockedUser                      model.UserRepository
	MockedProperty                  model.PropertyRepository
	MockedPlayer                    model.PlayerRepository
	MockedPlaylist                  model.PlaylistRepository
	MockedDiscovery                 model.DiscoveryRepository
	MockedPlaylistFolder            model.PlaylistFolderRepository
	MockedPlayQueue                 model.PlayQueueRepository
	MockedShare                     model.ShareRepository
	MockedTranscoding               model.TranscodingRepository
	MockedUserProps                 model.UserPropsRepository
	MockedScrobbleBuffer            model.ScrobbleBufferRepository
	MockedScrobble                  model.ScrobbleRepository
	MockedRadio                     model.RadioRepository
	MockedLoudnessAudit             *MockLoudnessAuditRepo
	MockedPlugin                    model.PluginRepository
	MockedRetailPlayerDeviceMapping model.RetailPlayerDeviceMappingRepository
	MockedRetailPlayerFolder        model.RetailPlayerFolderRepository
	scrobbleBufferMu                sync.Mutex
	repoMu                          sync.Mutex

	// GC tracking
	GCCalled bool
	GCError  error
}

func (db *MockDataStore) Library(ctx context.Context) model.LibraryRepository {
	if db.MockedLibrary != nil {
		return db.MockedLibrary
	}
	if db.RealDS != nil {
		return db.RealDS.Library(ctx)
	}
	db.MockedLibrary = &MockLibraryRepo{}
	return db.MockedLibrary
}

func (db *MockDataStore) Folder(ctx context.Context) model.FolderRepository {
	if db.MockedFolder != nil {
		return db.MockedFolder
	}
	if db.RealDS != nil {
		return db.RealDS.Folder(ctx)
	}
	db.MockedFolder = struct{ model.FolderRepository }{}
	return db.MockedFolder
}

func (db *MockDataStore) Tag(ctx context.Context) model.TagRepository {
	if db.MockedTag != nil {
		return db.MockedTag
	}
	if db.RealDS != nil {
		return db.RealDS.Tag(ctx)
	}
	db.MockedTag = struct{ model.TagRepository }{}
	return db.MockedTag
}

func (db *MockDataStore) Album(ctx context.Context) model.AlbumRepository {
	if db.MockedAlbum != nil {
		return db.MockedAlbum
	}
	if db.RealDS != nil {
		return db.RealDS.Album(ctx)
	}
	db.MockedAlbum = CreateMockAlbumRepo()
	return db.MockedAlbum
}

func (db *MockDataStore) Artist(ctx context.Context) model.ArtistRepository {
	if db.MockedArtist != nil {
		return db.MockedArtist
	}
	if db.RealDS != nil {
		return db.RealDS.Artist(ctx)
	}
	db.MockedArtist = CreateMockArtistRepo()
	return db.MockedArtist
}

func (db *MockDataStore) MediaFile(ctx context.Context) model.MediaFileRepository {
	if db.RealDS != nil && db.MockedMediaFile == nil {
		return db.RealDS.MediaFile(ctx)
	}
	db.repoMu.Lock()
	defer db.repoMu.Unlock()
	if db.MockedMediaFile == nil {
		db.MockedMediaFile = CreateMockMediaFileRepo()
	}
	return db.MockedMediaFile
}

func (db *MockDataStore) Genre(ctx context.Context) model.GenreRepository {
	if db.MockedGenre != nil {
		return db.MockedGenre
	}
	if db.RealDS != nil {
		return db.RealDS.Genre(ctx)
	}
	db.MockedGenre = &MockedGenreRepo{}
	return db.MockedGenre
}

func (db *MockDataStore) Playlist(ctx context.Context) model.PlaylistRepository {
	if db.MockedPlaylist != nil {
		return db.MockedPlaylist
	}
	if db.RealDS != nil {
		return db.RealDS.Playlist(ctx)
	}
	db.MockedPlaylist = CreateMockPlaylistRepo()
	return db.MockedPlaylist
}

func (db *MockDataStore) Discovery(ctx context.Context) model.DiscoveryRepository {
	if db.MockedDiscovery == nil {
		if db.RealDS != nil {
			db.MockedDiscovery = db.RealDS.Discovery(ctx)
		} else {
			db.MockedDiscovery = struct{ model.DiscoveryRepository }{}
		}
	}
	return db.MockedDiscovery
}

func (db *MockDataStore) PlaylistFolder(ctx context.Context) model.PlaylistFolderRepository {
	if db.MockedPlaylistFolder == nil {
		if db.RealDS != nil {
			db.MockedPlaylistFolder = db.RealDS.PlaylistFolder(ctx)
		} else {
			db.MockedPlaylistFolder = &MockPlaylistFolderRepo{}
		}
	}
	return db.MockedPlaylistFolder
}

func (db *MockDataStore) PlayQueue(ctx context.Context) model.PlayQueueRepository {
	if db.MockedPlayQueue != nil {
		return db.MockedPlayQueue
	}
	if db.RealDS != nil {
		return db.RealDS.PlayQueue(ctx)
	}
	db.MockedPlayQueue = &MockPlayQueueRepo{}
	return db.MockedPlayQueue
}

func (db *MockDataStore) UserProps(ctx context.Context) model.UserPropsRepository {
	if db.MockedUserProps != nil {
		return db.MockedUserProps
	}
	if db.RealDS != nil {
		return db.RealDS.UserProps(ctx)
	}
	db.MockedUserProps = &MockedUserPropsRepo{}
	return db.MockedUserProps
}

func (db *MockDataStore) Property(ctx context.Context) model.PropertyRepository {
	if db.MockedProperty != nil {
		return db.MockedProperty
	}
	if db.RealDS != nil {
		return db.RealDS.Property(ctx)
	}
	db.MockedProperty = &MockedPropertyRepo{}
	return db.MockedProperty
}

func (db *MockDataStore) Share(ctx context.Context) model.ShareRepository {
	if db.MockedShare != nil {
		return db.MockedShare
	}
	if db.RealDS != nil {
		return db.RealDS.Share(ctx)
	}
	db.MockedShare = &MockShareRepo{}
	return db.MockedShare
}

func (db *MockDataStore) User(ctx context.Context) model.UserRepository {
	if db.MockedUser != nil {
		return db.MockedUser
	}
	if db.RealDS != nil {
		return db.RealDS.User(ctx)
	}
	db.MockedUser = CreateMockUserRepo()
	return db.MockedUser
}

func (db *MockDataStore) RetailPlayerDeviceMapping(ctx context.Context) model.RetailPlayerDeviceMappingRepository {
	if db.MockedRetailPlayerDeviceMapping == nil {
		if db.RealDS != nil {
			db.MockedRetailPlayerDeviceMapping = db.RealDS.RetailPlayerDeviceMapping(ctx)
		} else {
			db.MockedRetailPlayerDeviceMapping = struct {
				model.RetailPlayerDeviceMappingRepository
			}{}
		}
	}
	return db.MockedRetailPlayerDeviceMapping
}

func (db *MockDataStore) RetailPlayerFolder(ctx context.Context) model.RetailPlayerFolderRepository {
	if db.MockedRetailPlayerFolder == nil {
		if db.RealDS != nil {
			db.MockedRetailPlayerFolder = db.RealDS.RetailPlayerFolder(ctx)
		} else {
			db.MockedRetailPlayerFolder = struct {
				model.RetailPlayerFolderRepository
			}{}
		}
	}
	return db.MockedRetailPlayerFolder
}

func (db *MockDataStore) Transcoding(ctx context.Context) model.TranscodingRepository {
	if db.MockedTranscoding != nil {
		return db.MockedTranscoding
	}
	if db.RealDS != nil {
		return db.RealDS.Transcoding(ctx)
	}
	db.MockedTranscoding = struct{ model.TranscodingRepository }{}
	return db.MockedTranscoding
}

func (db *MockDataStore) Player(ctx context.Context) model.PlayerRepository {
	if db.MockedPlayer != nil {
		return db.MockedPlayer
	}
	if db.RealDS != nil {
		return db.RealDS.Player(ctx)
	}
	db.MockedPlayer = struct{ model.PlayerRepository }{}
	return db.MockedPlayer
}

func (db *MockDataStore) ScrobbleBuffer(ctx context.Context) model.ScrobbleBufferRepository {
	if db.RealDS != nil && db.MockedScrobbleBuffer == nil {
		return db.RealDS.ScrobbleBuffer(ctx)
	}
	db.scrobbleBufferMu.Lock()
	defer db.scrobbleBufferMu.Unlock()
	if db.MockedScrobbleBuffer == nil {
		db.MockedScrobbleBuffer = &MockedScrobbleBufferRepo{}
	}
	return db.MockedScrobbleBuffer
}

func (db *MockDataStore) Scrobble(ctx context.Context) model.ScrobbleRepository {
	if db.MockedScrobble != nil {
		return db.MockedScrobble
	}
	if db.RealDS != nil {
		return db.RealDS.Scrobble(ctx)
	}
	db.MockedScrobble = &MockScrobbleRepo{ctx: ctx}
	return db.MockedScrobble
}

func (db *MockDataStore) LoudnessAudit(ctx context.Context) model.LoudnessAuditRepository {
	if db.MockedLoudnessAudit != nil {
		return db.MockedLoudnessAudit
	}
	if db.RealDS != nil {
		return db.RealDS.LoudnessAudit(ctx)
	}
	db.MockedLoudnessAudit = &MockLoudnessAuditRepo{data: map[string]*model.LoudnessAudit{}}
	return db.MockedLoudnessAudit
}

// MockLoudnessAuditRepo is an in-memory LoudnessAuditRepository for tests.
type MockLoudnessAuditRepo struct {
	data map[string]*model.LoudnessAudit
	// snaps holds the copies Snapshot took, newest first, so the handlers can be
	// exercised end to end without a real database file.
	snaps []model.LoudnessSnapshot
	saved map[string]map[string]*model.LoudnessAudit
	// SnapshotErr, when set, makes every snapshot call fail.
	SnapshotErr error
}

func (m *MockLoudnessAuditRepo) Put(audit *model.LoudnessAudit) error {
	if m.data == nil {
		m.data = map[string]*model.LoudnessAudit{}
	}
	m.data[audit.MediaFileID] = audit
	return nil
}

func (m *MockLoudnessAuditRepo) SetDecision(mediaFileID, decision string) error {
	if m.data == nil {
		m.data = map[string]*model.LoudnessAudit{}
	}
	if audit, ok := m.data[mediaFileID]; ok {
		audit.Decision = decision
		return nil
	}
	m.data[mediaFileID] = &model.LoudnessAudit{MediaFileID: mediaFileID, Decision: decision}
	return nil
}

func (m *MockLoudnessAuditRepo) Get(mediaFileID string) (*model.LoudnessAudit, error) {
	if audit, ok := m.data[mediaFileID]; ok {
		return audit, nil
	}
	return nil, model.ErrNotFound
}

func (m *MockLoudnessAuditRepo) Clear() (int64, error) {
	count := int64(len(m.data))
	m.data = map[string]*model.LoudnessAudit{}
	return count, nil
}

func (m *MockLoudnessAuditRepo) Snapshot(dir string, keep int) (*model.LoudnessSnapshot, error) {
	if m.SnapshotErr != nil {
		return nil, m.SnapshotErr
	}
	if m.saved == nil {
		m.saved = map[string]map[string]*model.LoudnessAudit{}
	}
	copied := map[string]*model.LoudnessAudit{}
	for id, audit := range m.data {
		clone := *audit
		copied[id] = &clone
	}
	now := time.Now()
	snap := model.LoudnessSnapshot{
		File:      fmt.Sprintf("lufs-audit-%s-%d.db", now.Format("20060102-150405"), len(m.snaps)),
		Path:      dir,
		Rows:      int64(len(copied)),
		CreatedAt: now,
	}
	m.saved[snap.File] = copied
	m.snaps = append([]model.LoudnessSnapshot{snap}, m.snaps...)
	if keep > 0 && len(m.snaps) > keep {
		for _, dropped := range m.snaps[keep:] {
			delete(m.saved, dropped.File)
		}
		m.snaps = m.snaps[:keep]
	}
	return &snap, nil
}

func (m *MockLoudnessAuditRepo) Snapshots(string) ([]model.LoudnessSnapshot, error) {
	if m.SnapshotErr != nil {
		return nil, m.SnapshotErr
	}
	return m.snaps, nil
}

func (m *MockLoudnessAuditRepo) RestoreSnapshot(_, file string) (*model.LoudnessRestoreReport, error) {
	if m.SnapshotErr != nil {
		return nil, m.SnapshotErr
	}
	saved, ok := m.saved[file]
	if !ok {
		return nil, model.ErrNotFound
	}
	report := &model.LoudnessRestoreReport{
		File:     file,
		Rows:     int64(len(saved)),
		Restored: int64(len(saved)),
	}
	for id := range m.data {
		if _, kept := saved[id]; !kept {
			report.Removed++
		}
	}
	m.data = saved
	return report, nil
}

func (db *MockDataStore) Radio(ctx context.Context) model.RadioRepository {
	if db.MockedRadio != nil {
		return db.MockedRadio
	}
	if db.RealDS != nil {
		return db.RealDS.Radio(ctx)
	}
	db.MockedRadio = CreateMockedRadioRepo()
	return db.MockedRadio
}

func (db *MockDataStore) Plugin(ctx context.Context) model.PluginRepository {
	if db.MockedPlugin != nil {
		return db.MockedPlugin
	}
	if db.RealDS != nil {
		return db.RealDS.Plugin(ctx)
	}
	db.MockedPlugin = CreateMockPluginRepo()
	return db.MockedPlugin
}

func (db *MockDataStore) WithTx(block func(tx model.DataStore) error, label ...string) error {
	return block(db)
}

func (db *MockDataStore) WithTxImmediate(block func(tx model.DataStore) error, label ...string) error {
	return block(db)
}

func (db *MockDataStore) Resource(ctx context.Context, m any) model.ResourceRepository {
	switch m.(type) {
	case model.MediaFile, *model.MediaFile:
		return db.MediaFile(ctx).(model.ResourceRepository)
	case model.Album, *model.Album:
		return db.Album(ctx).(model.ResourceRepository)
	case model.Artist, *model.Artist:
		return db.Artist(ctx).(model.ResourceRepository)
	case model.User, *model.User:
		return db.User(ctx).(model.ResourceRepository)
	case model.Playlist, *model.Playlist:
		return db.Playlist(ctx).(model.ResourceRepository)
	case model.Radio, *model.Radio:
		return db.Radio(ctx).(model.ResourceRepository)
	case model.Share, *model.Share:
		return db.Share(ctx).(model.ResourceRepository)
	case model.Genre, *model.Genre:
		return db.Genre(ctx).(model.ResourceRepository)
	case model.Tag, *model.Tag:
		return db.Tag(ctx).(model.ResourceRepository)
	case model.Transcoding, *model.Transcoding:
		return db.Transcoding(ctx).(model.ResourceRepository)
	case model.Player, *model.Player:
		return db.Player(ctx).(model.ResourceRepository)
	case model.RetailPlayerFolder, *model.RetailPlayerFolder:
		return db.RetailPlayerFolder(ctx).(model.ResourceRepository)
	case model.Plugin, *model.Plugin:
		return db.Plugin(ctx).(model.ResourceRepository)
	default:
		return struct{ model.ResourceRepository }{}
	}
}

func (db *MockDataStore) GC(context.Context, ...int) error {
	db.GCCalled = true
	if db.GCError != nil {
		return db.GCError
	}
	return nil
}
