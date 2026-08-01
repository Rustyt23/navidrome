package silence

import (
	"context"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutput(t *testing.T) {
	output := `[silencedetect @ x] silence_start: 0
[silencedetect @ x] silence_end: 1.25 | silence_duration: 1.25
[silencedetect @ x] silence_start: 8.75
[silencedetect @ x] silence_end: 10 | silence_duration: 1.25`

	result, err := parseOutput(output, 10)
	require.NoError(t, err)
	assert.InDelta(t, 1.25, result.Leading, 0.0001)
	assert.InDelta(t, 1.25, result.Trailing, 0.0001)
}

func TestParseOutputIgnoresMiddleSilence(t *testing.T) {
	output := `[silencedetect @ x] silence_start: 3
[silencedetect @ x] silence_end: 4 | silence_duration: 1`

	result, err := parseOutput(output, 10)
	require.NoError(t, err)
	assert.Zero(t, result.Leading)
	assert.Zero(t, result.Trailing)
}

func TestParseOutputClosesTrailingSilenceAtEOF(t *testing.T) {
	output := `[silencedetect @ x] silence_start: 9.2`

	result, err := parseOutput(output, 10)
	require.NoError(t, err)
	assert.Zero(t, result.Leading)
	assert.InDelta(t, 0.8, result.Trailing, 0.0001)
}

func TestParseOutputUsesDecodedDurationInsteadOfStaleMetadata(t *testing.T) {
	output := `[silencedetect @ x] silence_start: 9.7
[silencedetect @ x] silence_end: 10 | silence_duration: 0.3
out_time_us=10000000
progress=end`

	result, err := parseOutput(output, 10.5)
	require.NoError(t, err)
	assert.InDelta(t, 0.3, result.Trailing, 0.0001)
}

func TestParseOutputUsesDecodedDurationWhenMetadataIsUnknown(t *testing.T) {
	output := `[silencedetect @ x] silence_start: 9.8
[silencedetect @ x] silence_end: 10 | silence_duration: 0.2
out_time_us=10000000
progress=end`

	result, err := parseOutput(output, 0)
	require.NoError(t, err)
	assert.InDelta(t, 0.2, result.Trailing, 0.0001)
}

func TestParseOutputDetectsShortEncoderPadding(t *testing.T) {
	output := `[silencedetect @ x] silence_start: 0
[silencedetect @ x] silence_end: 0.08 | silence_duration: 0.08
[silencedetect @ x] silence_start: 9.91
[silencedetect @ x] silence_end: 10 | silence_duration: 0.09
out_time_us=10000000
progress=end`

	result, err := parseOutput(output, 10)
	require.NoError(t, err)
	assert.InDelta(t, 0.08, result.Leading, 0.0001)
	assert.InDelta(t, 0.09, result.Trailing, 0.0001)
}

func TestParseOutputUsesZeroForAnalyzedAudioWithoutSilence(t *testing.T) {
	result, err := parseOutput("ordinary ffmpeg output", 10)
	require.NoError(t, err)
	assert.Equal(t, Result{Duration: 10}, result)
}

func TestAnalyzerMeasuresGeneratedAudioWithoutChangingIt(t *testing.T) {
	cmdPath, err := ffmpeg.New().CmdPath()
	if err != nil {
		t.Skip("ffmpeg is not available")
	}
	path := filepath.Join(t.TempDir(), "silence-test.wav")
	generate := exec.Command(cmdPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", `aevalsrc=if(between(t\,1\,2)\,0.2*sin(2*PI*440*t)\,0):s=44100:d=4`,
		"-c:a", "pcm_s16le", path,
	)
	require.NoError(t, generate.Run())
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	beforeHash := sha256.Sum256(before)

	result, err := NewAnalyzer().Analyze(context.Background(), path, 4)
	require.NoError(t, err)
	assert.InDelta(t, 1, result.Leading, 0.02)
	assert.InDelta(t, 2, result.Trailing, 0.02)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, beforeHash, sha256.Sum256(after), "analysis must not change the audio file")
}
