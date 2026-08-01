package nativeapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifySilenceBackupFileRecoversPreparedTrim(t *testing.T) {
	backup := &model.SilenceBackup{
		Status:         model.SilenceBackupStatusPrepared,
		OriginalSHA256: "original",
		TrimmedSHA256:  "trimmed",
	}
	assert.Equal(t, silenceFileIsOriginal, classifySilenceBackupFile(backup, "original"))
	assert.Equal(t, silenceFileIsTrimmed, classifySilenceBackupFile(backup, "trimmed"))
	assert.Equal(t, silenceFileIsUnknown, classifySilenceBackupFile(backup, "external-edit"))
}

func TestValidateSilenceMargin(t *testing.T) {
	for _, margin := range []float64{0, 0.1, 2} {
		assert.NoError(t, validateSilenceMargin(margin))
	}
	for _, margin := range []float64{-0.001, 2.001} {
		assert.Error(t, validateSilenceMargin(margin))
	}
}

func TestValidateSilenceBackupLocationChecksEveryLibrary(t *testing.T) {
	root := t.TempDir()
	libraryOne := filepath.Join(root, "library-one")
	libraryTwo := filepath.Join(root, "library-two")
	require.NoError(t, os.MkdirAll(libraryOne, 0o755))
	require.NoError(t, os.MkdirAll(libraryTwo, 0o755))

	repository := &tests.MockLibraryRepo{}
	repository.SetData(model.Libraries{
		{ID: 1, Name: "One", Path: libraryOne},
		{ID: 2, Name: "Two", Path: libraryTwo},
	})
	router := &Router{ds: &tests.MockDataStore{MockedLibrary: repository}}

	err := router.validateSilenceBackupLocation(
		context.Background(), filepath.Join(libraryTwo, "silence trim backup"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Two")

	require.NoError(t, router.validateSilenceBackupLocation(
		context.Background(), filepath.Join(root, "data", "silence trim backup"),
	))
}

func TestSilenceScanTargetUsesTheChangedSongFolder(t *testing.T) {
	library := t.TempDir()
	folder := filepath.Join(library, "Artist", "Album")
	require.NoError(t, os.MkdirAll(folder, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(folder, "Song.mp3"), []byte("test"), 0o600))
	target, err := silenceScanTarget(&model.MediaFile{
		LibraryID: 7, LibraryPath: library, Path: filepath.FromSlash("Artist/Album/Song.mp3"),
	})
	require.NoError(t, err)
	assert.Equal(t, model.ScanTarget{LibraryID: 7, FolderPath: "Artist/Album"}, target)
}
