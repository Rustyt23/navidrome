package silence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type packetHashProbe struct {
	Packets []struct {
		DataHash string `json:"data_hash"`
	} `json:"packets"`
}

// validatePacketSubsequence proves that every encoded audio packet in the
// trimmed output is an unchanged contiguous packet from the original. This is
// stronger than comparing codec metadata: it detects a hidden re-encode while
// still allowing packet-copy trimming to drop frames at either edge.
func validatePacketSubsequence(ctx context.Context, ffmpegPath, originalPath, trimmedPath string) error {
	original, err := packetPayloadHashes(ctx, ffmpegPath, originalPath)
	if err != nil {
		return fmt.Errorf("read original audio packets: %w", err)
	}
	trimmed, err := packetPayloadHashes(ctx, ffmpegPath, trimmedPath)
	if err != nil {
		return fmt.Errorf("read trimmed audio packets: %w", err)
	}
	if len(original) == 0 || len(trimmed) == 0 {
		return errors.New("audio packet integrity check found no packets")
	}
	if len(trimmed) > len(original) {
		return errors.New("trimmed audio contains more packets than the original")
	}
	for start := 0; start+len(trimmed) <= len(original); start++ {
		matched := true
		for index := range trimmed {
			if trimmed[index] != original[start+index] {
				matched = false
				break
			}
		}
		if matched {
			return nil
		}
	}
	return errors.New("trimmed audio packets do not match an unchanged original packet sequence")
}

func packetPayloadHashes(ctx context.Context, ffmpegPath, path string) ([]string, error) {
	probePath := siblingFFprobePath(ffmpegPath)
	args := []string{
		"-v", "error", "-select_streams", "a:0", "-show_packets",
		"-show_entries", "packet=data_hash", "-show_data_hash", "sha256",
		"-of", "json", path,
	}
	output, err := exec.CommandContext(ctx, probePath, args...).Output()
	if err != nil {
		return nil, err
	}
	var probe packetHashProbe
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, err
	}
	hashes := make([]string, 0, len(probe.Packets))
	for _, packet := range probe.Packets {
		hash := strings.TrimSpace(packet.DataHash)
		if hash == "" {
			return nil, errors.New("audio packet has no payload hash")
		}
		hashes = append(hashes, strings.TrimPrefix(hash, "SHA256:"))
	}
	return hashes, nil
}
