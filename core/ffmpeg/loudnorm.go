package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// LoudnessNormalizer measures loudness. Applying a change is a separate,
// explicitly decided step - see Apply - so that nothing about the transform is
// left to ffmpeg's own judgement.
type LoudnessNormalizer interface {
	AnalyzeLoudness(ctx context.Context, path string, target LoudnessTarget) (*LoudnessAnalysis, error)
}

func NewLoudnessNormalizer() LoudnessNormalizer {
	return &ffmpeg{}
}

type LoudnessTarget struct {
	IntegratedLUFS float64
	TruePeak       float64
	LRA            float64
}

type LoudnessAnalysis struct {
	InputIntegrated float64
	InputTruePeak   float64
	InputLRA        float64
	InputThreshold  float64
	TargetOffset    float64
}

type loudnormJSON struct {
	InputIntegrated string `json:"input_i"`
	InputTruePeak   string `json:"input_tp"`
	InputLRA        string `json:"input_lra"`
	InputThreshold  string `json:"input_thresh"`
	TargetOffset    string `json:"target_offset"`
}

func (e *ffmpeg) AnalyzeLoudness(ctx context.Context, path string, target LoudnessTarget) (*LoudnessAnalysis, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return nil, err
	}
	if err := fileExists(path); err != nil {
		return nil, err
	}
	filter := loudnormFilter(target)
	args := analyzeLoudnessArgs(path, filter)
	output, err := runCommand(ctx, unknownDurationTimeout, cmdPath, args...)
	if err != nil {
		return nil, fmt.Errorf("analyzing loudness: %w: %s", err, string(output))
	}
	analysis, err := parseLoudnessAnalysis(output)
	if err != nil {
		return nil, fmt.Errorf("analyzing loudness: %w", err)
	}
	return analysis, nil
}

func analyzeLoudnessArgs(path, filter string) []string {
	return []string{"-nostdin", "-hide_banner", "-i", path, "-map", "0:a:0", "-vn", "-af", filter, "-f", "null", "-"}
}

// loudnormFilter builds the measurement-only filter. loudnorm is used purely
// as an analyser: it reports the loudness, true peak and loudness range, and
// never touches the audio.
func loudnormFilter(target LoudnessTarget) string {
	return fmt.Sprintf("loudnorm=I=%s:TP=%s:LRA=%s:print_format=json",
		formatFloat(target.IntegratedLUFS), formatFloat(target.TruePeak), formatFloat(target.LRA))
}

func parseLoudnessAnalysis(output []byte) (*LoudnessAnalysis, error) {
	jsonText, extractErr := extractLoudnormJSON(output)
	if extractErr != nil {
		return nil, extractErr
	}
	var raw loudnormJSON
	if err := json.Unmarshal(jsonText, &raw); err != nil {
		return nil, err
	}
	analysis := &LoudnessAnalysis{}
	var err error
	if analysis.InputIntegrated, err = parseLoudnormFloat(raw.InputIntegrated, "input_i"); err != nil {
		return nil, err
	}
	if analysis.InputTruePeak, err = parseLoudnormFloat(raw.InputTruePeak, "input_tp"); err != nil {
		return nil, err
	}
	if analysis.InputLRA, err = parseLoudnormFloat(raw.InputLRA, "input_lra"); err != nil {
		return nil, err
	}
	if analysis.InputThreshold, err = parseLoudnormFloat(raw.InputThreshold, "input_thresh"); err != nil {
		return nil, err
	}
	if analysis.TargetOffset, err = parseLoudnormFloat(raw.TargetOffset, "target_offset"); err != nil {
		return nil, err
	}
	return analysis, nil
}

func extractLoudnormJSON(output []byte) ([]byte, error) {
	marker := []byte(`"input_i"`)
	markerIndex := bytes.LastIndex(output, marker)
	if markerIndex == -1 {
		return nil, fmt.Errorf("loudnorm JSON not found")
	}

	start := bytes.LastIndexByte(output[:markerIndex], '{')
	if start == -1 {
		return nil, fmt.Errorf("loudnorm JSON start not found")
	}

	end := matchingJSONEnd(output[start:])
	if end == -1 {
		return nil, fmt.Errorf("loudnorm JSON end not found")
	}
	return output[start : start+end], nil
}

func matchingJSONEnd(input []byte) int {
	depth := 0
	inString := false
	escaped := false

	for i, b := range input {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch b {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func parseLoudnormFloat(value, name string) (float64, error) {
	if value == "" || value == "N/A" {
		return 0, fmt.Errorf("invalid %s value %q", name, value)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", name, value, err)
	}
	return parsed, nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
