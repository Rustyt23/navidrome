package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type coverArtArchiveClient struct {
	httpClient *http.Client
}

func newCoverArtArchiveClient(client *http.Client) *coverArtArchiveClient {
	if client == nil {
		client = &http.Client{}
	}
	return &coverArtArchiveClient{httpClient: client}
}

func (c *coverArtArchiveClient) Fetch(ctx context.Context, releaseID string) (string, error) {
	if c == nil || c.httpClient == nil || releaseID == "" {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverArtArchiveURL+releaseID, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover art archive request failed: %s", resp.Status)
	}

	var payload coverArtResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	for _, image := range payload.Images {
		if image.Front && image.Image != "" {
			return ensureHTTPSURL(image.Image), nil
		}
	}
	for _, image := range payload.Images {
		if image.Image != "" {
			return ensureHTTPSURL(image.Image), nil
		}
	}

	return "", nil
}

type coverArtResponse struct {
	Images []coverArtImage `json:"images"`
}

type coverArtImage struct {
	Image string `json:"image"`
	Front bool   `json:"front"`
}
