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
