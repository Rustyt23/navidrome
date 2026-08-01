package silence

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrimBacksUpWithoutReencodingAndRestoresExactOriginal(t *testing.T) {
	cmdPath, err := ffmpeg.New().CmdPath()
	if err != nil {
		t.Skip("ffmpeg is not available")
	}
	root := t.TempDir()
	sourcePath := filepath.Join(root, "song with spaces.mp3")
	audioOnlyPath := filepath.Join(root, "audio.mp3")
	generate := exec.Command(cmdPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", `aevalsrc=if(between(t\,1\,3)\,0.2*sin(2*PI*440*t)\,0):s=44100:d=4`,
		"-c:a", "libmp3lame", "-b:a", "192k", audioOnlyPath,
	)
	require.NoError(t, generate.Run())
	coverPath := filepath.Join(root, "cover.png")
	writeTestCover(t, coverPath)
	mux := exec.Command(cmdPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", audioOnlyPath, "-i", coverPath,
		"-map", "0:a:0", "-map", "1:v:0", "-c", "copy",
		"-metadata", "title=Integrity Test", "-metadata", "artist=Navidrome",
		"-metadata", "album=Silence", "-metadata", "comment=keep this tag",
		"-metadata:s:v:0", "title=Front Cover", "-metadata:s:v:0", "comment=Cover (front)",
		"-disposition:v:0", "attached_pic", sourcePath,
	)
	require.NoError(t, mux.Run())
	require.NoError(t, os.Chmod(sourcePath, 0o640))
	originalModTime := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(sourcePath, originalModTime, originalModTime))

	originalHash, err := HashFile(sourcePath)
	require.NoError(t, err)
	originalProbe, err := ffmpeg.New().ProbeAudioStream(context.Background(), sourcePath)
	require.NoError(t, err)
	originalAnalysis, err := NewAnalyzer().Analyze(context.Background(), sourcePath, 4)
	require.NoError(t, err)
	require.Greater(t, originalAnalysis.Leading, 0.9)
	require.Greater(t, originalAnalysis.Trailing, 0.9)
	originalSnapshot, err := probeMediaSnapshot(context.Background(), cmdPath, sourcePath)
	require.NoError(t, err)
	require.True(t, originalSnapshot.hasArtwork())
	originalArtworkHash, err := artworkSHA256(context.Background(), cmdPath, sourcePath)
	require.NoError(t, err)

	trimmer := NewTrimmer(filepath.Join(root, "data"))
	backup, err := trimmer.EnsureBackup(sourcePath, "media-id-1")
	require.NoError(t, err)
	assert.Equal(t, originalHash, backup.SHA256)
	assert.Equal(t, BackupFolderName, filepath.Base(trimmer.BackupRoot()))
	assert.Contains(t, backup.FileName, "/", "large backup sets must be sharded")
	assert.Equal(t, uint32(0o640), backup.Mode)
	assert.True(t, backup.ModTime.Equal(originalModTime))
	backupPath, err := trimmer.BackupPath(backup.FileName)
	require.NoError(t, err)
	backupHash, err := HashFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, originalHash, backupHash)

	prepared, err := trimmer.PrepareTrim(
		context.Background(), sourcePath, originalHash, originalAnalysis.Duration,
		originalAnalysis.Leading, originalAnalysis.Trailing,
	)
	require.NoError(t, err)
	defer prepared.Discard()
	stillOriginalHash, err := HashFile(sourcePath)
	require.NoError(t, err)
	assert.Equal(t, originalHash, stillOriginalHash, "prepare must not replace the song")
	require.NoError(t, trimmer.CommitTrim(sourcePath, originalHash, prepared))
	trimmed := prepared.Result
	assert.NotEqual(t, originalHash, trimmed.SHA256)
	assert.True(t, trimmed.DecodeVerified)
	assert.True(t, trimmed.AudioPropertiesVerified)
	assert.True(t, trimmed.MetadataVerified)
	assert.True(t, trimmed.ArtworkVerified)
	assert.True(t, trimmed.PacketIntegrityVerified)
	assert.Less(t, trimmed.Duration, originalAnalysis.Duration-1.5)
	trimmedProbe, err := ffmpeg.New().ProbeAudioStream(context.Background(), sourcePath)
	require.NoError(t, err)
	assert.Equal(t, originalProbe.Codec, trimmedProbe.Codec)
	assert.Equal(t, originalProbe.SampleRate, trimmedProbe.SampleRate)
	assert.Equal(t, originalProbe.Channels, trimmedProbe.Channels)
	assert.Equal(t, originalProbe.BitDepth, trimmedProbe.BitDepth)
	trimmedSnapshot, err := probeMediaSnapshot(context.Background(), cmdPath, sourcePath)
	require.NoError(t, err)
	require.NoError(t, compareMediaSnapshots(originalSnapshot, trimmedSnapshot))
	trimmedArtworkHash, err := artworkSHA256(context.Background(), cmdPath, sourcePath)
	require.NoError(t, err)
	assert.Equal(t, originalArtworkHash, trimmedArtworkHash)

	restored, err := trimmer.Restore(
		sourcePath, backup.FileName, backup.SHA256, trimmed.SHA256,
		backup.Mode, backup.ModTime,
	)
	require.NoError(t, err)
	assert.Equal(t, originalHash, restored.SHA256)
	finalHash, err := HashFile(sourcePath)
	require.NoError(t, err)
	assert.Equal(t, originalHash, finalHash, "restore must reproduce the original byte for byte")
	finalStat, err := os.Stat(sourcePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), finalStat.Mode().Perm())
	assert.True(t, finalStat.ModTime().Equal(originalModTime), "restore must preserve the original modification time")
}

func writeTestCover(t *testing.T, path string) {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x * 12), G: uint8(y * 12), B: 80, A: 255})
		}
	}
	file, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, png.Encode(file, canvas))
	require.NoError(t, file.Close())
}

func TestTrimRejectsFormatsWithoutAValidatedLosslessPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "song.flac")
	require.NoError(t, os.WriteFile(path, []byte("not audio"), 0o600))
	assert.EqualError(t, ValidateTrimSource(path), "safe trimming currently supports MP3 files only")
}

func TestEnsureBackupRefusesMismatchedExistingBackup(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "song.mp3")
	require.NoError(t, os.WriteFile(sourcePath, []byte("original"), 0o600))
	trimmer := NewTrimmer(filepath.Join(root, "data"))
	backup, err := trimmer.EnsureBackup(sourcePath, "media-id-2")
	require.NoError(t, err)
	backupPath, err := trimmer.BackupPath(backup.FileName)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(backupPath, []byte("tampered"), 0o600))

	_, err = trimmer.EnsureBackup(sourcePath, "media-id-2")
	assert.EqualError(t, err, "an unrelated backup already exists for this song")
}

func TestValidateTrimSourceRejectsHardLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hard-link behavior is covered by the Windows path validator")
	}
	root := t.TempDir()
	sourcePath := filepath.Join(root, "song.mp3")
	linkedPath := filepath.Join(root, "linked.mp3")
	require.NoError(t, os.WriteFile(sourcePath, []byte("audio"), 0o600))
	require.NoError(t, os.Link(sourcePath, linkedPath))
	err := ValidateTrimSource(sourcePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hard-linked")
}

func TestCommitTrimRefusesAnExternalEdit(t *testing.T) {
	cmdPath, err := ffmpeg.New().CmdPath()
	if err != nil {
		t.Skip("ffmpeg is not available")
	}
	root := t.TempDir()
	sourcePath := filepath.Join(root, "song.mp3")
	generate := exec.Command(cmdPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", `aevalsrc=if(between(t\,1\,3)\,0.2*sin(2*PI*440*t)\,0):s=44100:d=4`,
		"-c:a", "libmp3lame", "-b:a", "128k", sourcePath,
	)
	require.NoError(t, generate.Run())
	originalHash, err := HashFile(sourcePath)
	require.NoError(t, err)
	analysis, err := NewAnalyzer().Analyze(context.Background(), sourcePath, 4)
	require.NoError(t, err)
	trimmer := NewTrimmer(filepath.Join(root, "data"))
	prepared, err := trimmer.PrepareTrim(
		context.Background(), sourcePath, originalHash, analysis.Duration,
		analysis.Leading, analysis.Trailing,
	)
	require.NoError(t, err)
	defer prepared.Discard()

	file, err := os.OpenFile(sourcePath, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.WriteString("external edit")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	err = trimmer.CommitTrim(sourcePath, originalHash, prepared)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed")
	currentHash, hashErr := HashFile(sourcePath)
	require.NoError(t, hashErr)
	assert.NotEqual(t, prepared.Result.SHA256, currentHash, "validated output must not replace an externally edited song")
}
