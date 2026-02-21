package metadataenrichment

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	musicBrainzRecordingURL = "https://musicbrainz.org/ws/2/recording"
	musicBrainzUserAgent    = "Navidrome-MetadataEnrichment/1.0"
)

var (
	musicBrainzHTTPClient = &http.Client{Timeout: 30 * time.Second}
	musicBrainzTicker     = time.NewTicker(time.Second)
	musicBrainzRateLimit  = musicBrainzTicker.C
)

// SearchMusicBrainzRecording searches for recordings in MusicBrainz by title and artist.
// It returns the response body, HTTP status code and an error.
func SearchMusicBrainzRecording(title string, artist string) ([]byte, int, error) {
	<-musicBrainzRateLimit

	mbQuery := fmt.Sprintf(`recording:"%s" AND artist:"%s"`, title, artist)
	values := url.Values{}
	values.Set("query", mbQuery)
	values.Set("fmt", "json")

	endpoint := musicBrainzRecordingURL + "?" + values.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", musicBrainzUserAgent)

	resp, err := musicBrainzHTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
