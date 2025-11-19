package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/navidrome/navidrome/log"
)

const (
	fpcalcTimeout    = 10 * time.Second
	acoustIDEndpoint = "https://api.acoustid.org/v2/lookup"
	acoustIDMeta     = "recordings+releasegroups+releases+tracks"
	httpTimeout      = 10 * time.Second
)

// FPResult represents the parsed output of fpcalc.
type FPResult struct {
	Fingerprint string
	Duration    int
}

// AcoustIDResult captures the most relevant IDs from the AcoustID API.
type AcoustIDResult struct {
	RecordingMBID    string
	ReleaseGroupMBID string
	ReleaseMBID      string
	Score            float64
}

// FingerprintLookup represents a successful fingerprint and lookup match.
type FingerprintLookup struct {
	FP       FPResult
	AcoustID AcoustIDResult
}

type fpcalcResponse struct {
	Fingerprint string `json:"fingerprint"`
	Duration    int    `json:"duration"`
}

type acoustIDResponse struct {
	Status  string             `json:"status"`
	Results []acoustIDMatch    `json:"results"`
	Error   *acoustIDErrorInfo `json:"error,omitempty"`
}

type acoustIDErrorInfo struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type acoustIDMatch struct {
	Score      float64             `json:"score"`
	Recordings []acoustIDRecording `json:"recordings"`
}

type acoustIDRecording struct {
	ID            string           `json:"id"`
	ReleaseGroups []acoustIDEntity `json:"releasegroups"`
	Releases      []acoustIDEntity `json:"releases"`
}

type acoustIDEntity struct {
	ID string `json:"id"`
}

// IdentifySongByAudio generates an audio fingerprint and queries the AcoustID
// service to retrieve the best matching metadata. In case of failures during
// fingerprinting or lookup, it returns nil without propagating an error to keep
// the metadata pipeline resilient.
func IdentifySongByAudio(path string, apiKey string) (*FingerprintLookup, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path is empty")
	}

	fp, err := generateFingerprint(path)
	if err != nil {
		log.Warn("Audio fingerprint generation failed", "file", path, err)
		return nil, nil
	}
	if fp == nil {
		return nil, nil
	}

	if strings.TrimSpace(apiKey) == "" {
		log.Warn("AcoustID API key not configured; skipping lookup", "file", path)
		return nil, nil
	}

	lookup, err := lookupAcoustID(*fp, apiKey)
	if err != nil {
		log.Warn("AcoustID lookup failed", "file", path, err)
		return nil, nil
	}
	if lookup == nil {
		log.Warn("AcoustID lookup returned no results", "file", path)
		return nil, nil
	}

	return &FingerprintLookup{FP: *fp, AcoustID: *lookup}, nil
}

func generateFingerprint(path string) (*FPResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fpcalcTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "fpcalc", "-json", path)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("fpcalc timed out: %w", err)
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("fpcalc failed: %s", errMsg)
		}
		return nil, fmt.Errorf("fpcalc failed: %w", err)
	}

	var result fpcalcResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to decode fpcalc output: %w", err)
	}

	if result.Fingerprint == "" || result.Duration <= 0 {
		return nil, errors.New("fpcalc returned incomplete data")
	}

	return &FPResult{Fingerprint: result.Fingerprint, Duration: result.Duration}, nil
}

func lookupAcoustID(fp FPResult, apiKey string) (*AcoustIDResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	params := url.Values{}
	params.Set("client", apiKey)
	params.Set("meta", acoustIDMeta)
	params.Set("duration", strconv.Itoa(fp.Duration))
	params.Set("fingerprint", fp.Fingerprint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, acoustIDEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build AcoustID request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AcoustID request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read AcoustID response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AcoustID returned status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var data acoustIDResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to decode AcoustID response: %w", err)
	}

	if strings.ToLower(data.Status) != "ok" {
		if data.Error != nil {
			return nil, fmt.Errorf("AcoustID error %d: %s", data.Error.Code, data.Error.Message)
		}
		return nil, fmt.Errorf("AcoustID returned status %s", data.Status)
	}

	if len(data.Results) == 0 {
		return nil, nil
	}

	best := selectBestMatch(data.Results)
	if best == nil {
		return nil, nil
	}

	return best, nil
}

func selectBestMatch(matches []acoustIDMatch) *AcoustIDResult {
	var best *AcoustIDResult

	for _, match := range matches {
		candidate := &AcoustIDResult{Score: match.Score}
		if len(match.Recordings) > 0 {
			rec := match.Recordings[0]
			candidate.RecordingMBID = rec.ID
			if len(rec.ReleaseGroups) > 0 {
				candidate.ReleaseGroupMBID = rec.ReleaseGroups[0].ID
			}
			if len(rec.Releases) > 0 {
				candidate.ReleaseMBID = rec.Releases[0].ID
			}
		}

		if best == nil || candidate.Score > best.Score {
			best = candidate
		}
	}

	return best
}
