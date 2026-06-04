package ffmpeg

import "testing"

func TestParseLoudnessAnalysis(t *testing.T) {
	output := []byte(`ffmpeg output
{
	"input_i" : "-10.24",
	"input_tp" : "-1.75",
	"input_lra" : "7.80",
	"input_thresh" : "-20.31",
	"target_offset" : "-2.36"
}
more output`)

	analysis, err := parseLoudnessAnalysis(output)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.InputIntegrated != -10.24 {
		t.Fatalf("InputIntegrated = %v", analysis.InputIntegrated)
	}
	if analysis.InputTruePeak != -1.75 {
		t.Fatalf("InputTruePeak = %v", analysis.InputTruePeak)
	}
	if analysis.InputLRA != 7.80 {
		t.Fatalf("InputLRA = %v", analysis.InputLRA)
	}
	if analysis.InputThreshold != -20.31 {
		t.Fatalf("InputThreshold = %v", analysis.InputThreshold)
	}
	if analysis.TargetOffset != -2.36 {
		t.Fatalf("TargetOffset = %v", analysis.TargetOffset)
	}
}

func TestParseLoudnessAnalysisIgnoresMetadataBraces(t *testing.T) {
	output := []byte(`ffmpeg output
Metadata:
  title           : Nicky Thomas - Love Of The Common People
  id3v2_priv.TRAKTOR4: DMRT\S\{}\TAAD;S\not-json
  comment         : Can't Get You out of My Head (Nu Disco Mix)
[Parsed_loudnorm_0 @ 0x123]
{
	"input_i" : "-13.24",
	"input_tp" : "-1.25",
	"input_lra" : "6.80",
	"input_thresh" : "-23.31",
	"output_i" : "-12.61",
	"target_offset" : "0.01"
}
more output`)

	analysis, err := parseLoudnessAnalysis(output)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.InputIntegrated != -13.24 {
		t.Fatalf("InputIntegrated = %v", analysis.InputIntegrated)
	}
	if analysis.TargetOffset != 0.01 {
		t.Fatalf("TargetOffset = %v", analysis.TargetOffset)
	}
}

func TestParseLoudnessAnalysisRejectsInvalidData(t *testing.T) {
	_, err := parseLoudnessAnalysis([]byte(`{"input_i":"N/A"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoudnormFilter(t *testing.T) {
	target := LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -1.5, LRA: 11}
	analysis := &LoudnessAnalysis{InputIntegrated: -10.24, InputTruePeak: -1.75, InputLRA: 7.8, InputThreshold: -20.31, TargetOffset: -2.36}

	got := loudnormFilter(target, analysis, false)
	want := "loudnorm=I=-12.6:TP=-1.5:LRA=11:measured_I=-10.24:measured_TP=-1.75:measured_LRA=7.8:measured_thresh=-20.31:offset=-2.36:linear=true"
	if got != want {
		t.Fatalf("filter = %q, want %q", got, want)
	}
}

func TestAnalyzeLoudnessArgsSelectsAudioOnly(t *testing.T) {
	got := analyzeLoudnessArgs("song.mp3", "loudnorm=I=-12.6")
	want := []string{"-nostdin", "-hide_banner", "-i", "song.mp3", "-map", "0:a:0", "-vn", "-af", "loudnorm=I=-12.6", "-f", "null", "-"}
	assertStringSliceEqual(t, got, want)
}

func TestNormalizeLoudnessArgsSelectsAudioOnly(t *testing.T) {
	got := normalizeLoudnessArgs("in.mp3", "out.mp3", "loudnorm=I=-12.6")
	want := []string{"-nostdin", "-hide_banner", "-y", "-i", "in.mp3", "-map", "0:a:0", "-map_metadata", "0", "-vn", "-af", "loudnorm=I=-12.6", "out.mp3"}
	assertStringSliceEqual(t, got, want)
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q: %#v", i, got[i], want[i], got)
		}
	}
}
