package nativeapi

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// AI genre classification. Genre gets its own focused prompt instead of
// piggybacking on the album/year metadata prompt: classification quality was
// poor when the model juggled three fields at once, and that prompt was
// allowed to return nothing for songs it couldn't identify. This one is not —
// an unidentified song is classified from the artist's dominant style, an
// unknown artist from the title, album, and lyrics, so a non-empty answer
// always comes back while the "basis" field keeps the fallback honest.

const aiGenreMaxAttempts = 3

// genreTracingProvider records the existing genre calls without changing the
// provider's request, response, error, retry, or parsing behavior.
type genreTracingProvider struct {
	provider aiChatProvider
	attempts []genreDeveloperAttempt
}

func (p *genreTracingProvider) Chat(ctx context.Context, prompt string) (string, error) {
	answer, err := p.provider.Chat(ctx, prompt)
	attempt := genreDeveloperAttempt{
		Number:   len(p.attempts) + 1,
		Prompt:   redactAIChatTraceText(prompt),
		Response: redactAIChatTraceText(answer),
	}
	if err != nil {
		attempt.Error = redactAIChatTraceText(err.Error())
	}
	p.attempts = append(p.attempts, attempt)
	return answer, err
}

func (p *genreTracingProvider) trace(fetchedGenre string) *genreSourceDeveloperTrace {
	if strings.TrimSpace(fetchedGenre) == "" || len(p.attempts) == 0 {
		return nil
	}
	last := p.attempts[len(p.attempts)-1]
	return &genreSourceDeveloperTrace{
		Source:       "ai",
		Prompt:       last.Prompt,
		Response:     last.Response,
		FetchedGenre: fetchedGenre,
		Attempts:     append([]genreDeveloperAttempt(nil), p.attempts...),
	}
}

// aiGenrePrimaryGenres is the closed set the model must pick the primary
// genre from. It mirrors the iTunes store taxonomy so the AI vote and the
// iTunes vote in the genre consensus speak the same language.
const aiGenrePrimaryGenres = `Pop, Rock, Hip-Hop/Rap, R&B/Soul, Country, Dance/Electronic, Latin, Jazz, Classical, Metal, Folk, Reggae, Blues, Gospel/Christian, K-Pop, Afrobeats, Soundtrack, World`

// fetchAIGenre classifies one song's genre via the chat provider, one request
// per song (never batched — a single-recording prompt keeps the model
// focused). It returns "primary" or "primary, subgenre", retrying a few times
// on transport or parse failures; only a persistently failing provider
// yields "".
func fetchAIGenre(ctx context.Context, provider aiChatProvider, mf *model.MediaFile) string {
	prompt := aiGenrePrompt(mf)
	for attempt := 1; attempt <= aiGenreMaxAttempts; attempt++ {
		answer, err := provider.Chat(ctx, prompt)
		if err != nil {
			log.Debug(ctx, "AI genre lookup failed", "title", mf.Title, "artist", mf.Artist, "attempt", attempt, "err", err)
			if attempt < aiGenreMaxAttempts {
				time.Sleep(aiGenreRetryDelay(err))
			}
			continue
		}
		if genre := parseAIGenre(answer); genre != "" {
			return genre
		}
		log.Debug(ctx, "AI genre answer unusable", "title", mf.Title, "artist", mf.Artist, "attempt", attempt, "answer", answer)
	}
	return ""
}

var aiRetryDelayRegex = regexp.MustCompile(`retry in ([0-9.]+)s`)

// aiGenreRetryDelay picks how long to wait before retrying a failed provider
// call. Free-tier Gemini keys hit per-minute quotas mid-batch; the 429 body
// says exactly how long to wait, and retrying sooner just burns more quota.
func aiGenreRetryDelay(err error) time.Duration {
	msg := err.Error()
	if strings.Contains(msg, "RESOURCE_EXHAUSTED") || strings.Contains(msg, "429") {
		if m := aiRetryDelayRegex.FindStringSubmatch(msg); m != nil {
			if s, parseErr := strconv.ParseFloat(m[1], 64); parseErr == nil {
				return min(time.Duration((s+1)*float64(time.Second)), 70*time.Second)
			}
		}
		return 30 * time.Second
	}
	return 2 * time.Second
}

func aiGenrePrompt(mf *model.MediaFile) string {
	var b strings.Builder
	b.WriteString(`You are an expert musicologist. Classify the genre of one specific song.

Work through it in this order:
1. Identify the exact recording from the title, artist, album, and year below. Title or artist tags may contain junk (file names, "[Unknown Artist]", featured-artist suffixes) — extract the real song from them when possible.
2. If you know the exact song, classify the SONG itself, not the artist's reputation: a country song by a pop artist is Country, a rap verse over a pop beat is judged by the overall track.
3. If you cannot identify the exact song, classify it by the artist's dominant style in that album/year era.
4. If the artist is unknown too, infer the most likely genre from the title and album name.

Return only valid JSON, no markdown, in exactly this shape:
{"genre":"primary genre","subgenre":"more specific style or empty string","basis":"song|artist|inferred","confidence":0}

Rules:
- genre MUST be one of: `)
	b.WriteString(aiGenrePrimaryGenres)
	b.WriteString(`.
- subgenre is the single most specific style that fits (e.g. "Electropop", "Grunge", "Country Soul", "East Coast Hip Hop"); use an empty string only when nothing more specific applies.
- genre must NEVER be empty. Always return your best classification, even at low confidence.
- confidence is a whole number 0-100: 90+ when you recognize the exact recording, 60-89 when classifying from the artist, below 60 when inferring.

Song:
`)
	b.WriteString("Title: ")
	b.WriteString(mf.Title)
	b.WriteString("\nArtist: ")
	b.WriteString(mf.Artist)
	if album := strings.TrimSpace(mf.Album); album != "" && !strings.EqualFold(album, "[unknown album]") {
		b.WriteString("\nAlbum: ")
		b.WriteString(album)
	}
	if mf.Year > 0 {
		b.WriteString("\nYear: ")
		b.WriteString(yearAsString(mf.Year))
	}
	return b.String()
}

// parseAIGenre extracts "primary" or "primary, subgenre" from the model's
// JSON answer. Returns "" for unusable answers so the caller can retry.
func parseAIGenre(answer string) string {
	answer = strings.TrimSpace(answer)
	if start := strings.Index(answer, "{"); start >= 0 {
		if end := strings.LastIndex(answer, "}"); end > start {
			answer = answer[start : end+1]
		}
	}

	var raw struct {
		Genre    string `json:"genre"`
		Subgenre string `json:"subgenre"`
	}
	if err := json.Unmarshal([]byte(answer), &raw); err != nil {
		return ""
	}
	genre := strings.TrimSpace(raw.Genre)
	if genre == "" {
		return ""
	}
	subgenre := strings.TrimSpace(raw.Subgenre)
	if subgenre != "" && !strings.EqualFold(subgenre, genre) {
		return genre + ", " + subgenre
	}
	return genre
}
