package silence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type mediaSnapshot struct {
	Streams  []snapshotStream  `json:"streams"`
	Format   snapshotFormat    `json:"format"`
	Chapters []json.RawMessage `json:"chapters"`
}

type snapshotFormat struct {
	Tags map[string]string `json:"tags"`
}

type snapshotStream struct {
	CodecName   string              `json:"codec_name"`
	CodecType   string              `json:"codec_type"`
	Disposition snapshotDisposition `json:"disposition"`
	Tags        map[string]string   `json:"tags"`
}

type snapshotDisposition struct {
	AttachedPic int `json:"attached_pic"`
}

func probeMediaSnapshot(ctx context.Context, ffmpegPath, path string) (*mediaSnapshot, error) {
	probePath := siblingFFprobePath(ffmpegPath)
	args := []string{
		"-v", "error", "-print_format", "json",
		"-show_streams", "-show_format", "-show_chapters", path,
	}
	output, err := exec.CommandContext(ctx, probePath, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("probe media structure: %w", err)
	}
	var snapshot mediaSnapshot
	if err := json.Unmarshal(output, &snapshot); err != nil {
		return nil, fmt.Errorf("parse media structure: %w", err)
	}
	return &snapshot, nil
}

func (s *mediaSnapshot) validateForMP3Trim() error {
	audioCount, artCount := 0, 0
	for _, stream := range s.Streams {
		switch stream.CodecType {
		case "audio":
			audioCount++
		case "video":
			if stream.Disposition.AttachedPic != 1 {
				return errors.New("MP3 contains a non-cover video stream")
			}
			artCount++
		default:
			return fmt.Errorf("MP3 contains unsupported %s stream", stream.CodecType)
		}
	}
	if audioCount != 1 {
		return fmt.Errorf("safe trimming requires exactly one audio stream (found %d)", audioCount)
	}
	if artCount > 1 {
		return fmt.Errorf("safe trimming currently supports at most one embedded cover (found %d)", artCount)
	}
	if len(s.Chapters) > 0 {
		return errors.New("MP3 chapters require timeline rewriting and are not supported for safe trimming")
	}
	return nil
}

func (s *mediaSnapshot) hasArtwork() bool {
	for _, stream := range s.Streams {
		if stream.CodecType == "video" && stream.Disposition.AttachedPic == 1 {
			return true
		}
	}
	return false
}

func compareMediaSnapshots(before, after *mediaSnapshot) error {
	if len(before.Streams) != len(after.Streams) {
		return errors.New("trimmed file does not contain the same number of streams")
	}
	if len(after.Chapters) != 0 {
		return errors.New("trimmed file unexpectedly contains chapters")
	}
	beforeStreams := streamsInSemanticOrder(before.Streams)
	afterStreams := streamsInSemanticOrder(after.Streams)
	for index := range beforeStreams {
		left, right := beforeStreams[index], afterStreams[index]
		if left.CodecName != right.CodecName || left.CodecType != right.CodecType ||
			left.Disposition.AttachedPic != right.Disposition.AttachedPic {
			return fmt.Errorf("trimmed stream %d does not match the original", index)
		}
		if err := requireOriginalTags(left.Tags, right.Tags); err != nil {
			return fmt.Errorf("trimmed stream %d metadata: %w", index, err)
		}
	}
	if err := requireOriginalTags(before.Format.Tags, after.Format.Tags); err != nil {
		return fmt.Errorf("trimmed file metadata: %w", err)
	}
	return nil
}

func streamsInSemanticOrder(streams []snapshotStream) []snapshotStream {
	ordered := make([]snapshotStream, 0, len(streams))
	for _, kind := range []string{"audio", "video"} {
		for _, stream := range streams {
			if stream.CodecType == kind {
				ordered = append(ordered, stream)
			}
		}
	}
	return ordered
}

func requireOriginalTags(before, after map[string]string) error {
	afterNormalized := make(map[string]string, len(after))
	for key, value := range after {
		afterNormalized[strings.ToLower(key)] = value
	}
	for key, expected := range before {
		actual, exists := afterNormalized[strings.ToLower(key)]
		if !exists || actual != expected {
			return fmt.Errorf("tag %q was not preserved", key)
		}
	}
	return nil
}

func artworkSHA256(ctx context.Context, ffmpegPath, path string) (string, error) {
	args := []string{
		"-nostdin", "-hide_banner", "-loglevel", "error", "-i", path,
		"-map", "0:v:0", "-c:v", "copy", "-f", "image2pipe", "pipe:1",
	}
	content, err := exec.CommandContext(ctx, ffmpegPath, args...).Output()
	if err != nil {
		return "", fmt.Errorf("read embedded artwork: %w", err)
	}
	if len(content) == 0 {
		return "", errors.New("embedded artwork is empty")
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

func siblingFFprobePath(ffmpegPath string) string {
	directory, base := filepath.Dir(ffmpegPath), filepath.Base(ffmpegPath)
	probeBase := strings.Replace(base, "ffmpeg", "ffprobe", 1)
	if directory == "." {
		return probeBase
	}
	return filepath.Join(directory, probeBase)
}
