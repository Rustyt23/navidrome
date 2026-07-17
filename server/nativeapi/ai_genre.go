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
// poor when the model juggled several fields at once, and that prompt was
// allowed to return nothing for songs it couldn't identify. This one is not —
// an unidentified song is classified from the artist's dominant style, an
// unknown artist from the title and album, so a non-empty answer always comes
// back. The model returns a primary genre and a more specific subgenre, which
// are parsed and stored separately.

const aiGenreMaxAttempts = 3

// genreTracingProvider records the existing genre calls without changing the
// provider's request, response, error, retry, or parsing behavior. It also
// sums the token usage the provider reports across every attempt for the song.
type genreTracingProvider struct {
	provider aiChatProvider
	attempts []genreDeveloperAttempt
	usage    tokenUsage
}

func (p *genreTracingProvider) Chat(ctx context.Context, prompt string) (string, error) {
	var (
		answer string
		err    error
		usage  tokenUsage
	)
	if u, ok := p.provider.(usageAwareChatProvider); ok {
		answer, usage, err = u.ChatUsage(ctx, prompt)
	} else {
		answer, err = p.provider.Chat(ctx, prompt)
	}
	p.usage.add(usage)

	attempt := genreDeveloperAttempt{
		Number:   len(p.attempts) + 1,
		Prompt:   redactAIChatTraceText(prompt),
		Response: redactAIChatTraceText(answer),
	}
	if !usage.isZero() {
		attemptUsage := usage
		attempt.Tokens = &attemptUsage
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

// fetchAIGenre classifies one song's genre with the focused single-song
// prompt (the most accurate path; multi-song batching lives in
// fetchAIGenres). It returns the primary genre and the more specific subgenre
// separately, retrying a few times on transport or parse failures; only a
// persistently failing provider yields empty strings.
func fetchAIGenre(ctx context.Context, provider aiChatProvider, mf *model.MediaFile) (genre, subgenre string) {
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
		if genre, subgenre := parseAIGenre(answer); genre != "" {
			return genre, subgenre
		}
		log.Debug(ctx, "AI genre answer unusable", "title", mf.Title, "artist", mf.Artist, "attempt", attempt, "answer", answer)
	}
	return "", ""
}

// aiGenreResult is one song's classification from the AI.
type aiGenreResult struct {
	Genre    string
	Subgenre string
}

// fetchAIGenres classifies a slice of songs. A single song uses the focused
// one-song prompt; multiple songs share one prompt so the instruction block
// is paid for once (the user picks the batch size, trading accuracy for
// tokens). Songs the batched answer misses fall back to individual prompts so
// no song is left blank.
func fetchAIGenres(ctx context.Context, provider aiChatProvider, mfs []*model.MediaFile) []aiGenreResult {
	results := make([]aiGenreResult, len(mfs))
	if len(mfs) == 0 {
		return results
	}
	if len(mfs) == 1 {
		genre, subgenre := fetchAIGenre(ctx, provider, mfs[0])
		results[0] = aiGenreResult{Genre: genre, Subgenre: subgenre}
		return results
	}

	prompt := aiGenreBatchPrompt(mfs)
	for attempt := 1; attempt <= aiGenreMaxAttempts; attempt++ {
		answer, err := provider.Chat(ctx, prompt)
		if err != nil {
			log.Debug(ctx, "AI genre batch lookup failed", "songs", len(mfs), "attempt", attempt, "err", err)
			if attempt < aiGenreMaxAttempts {
				time.Sleep(aiGenreRetryDelay(err))
			}
			continue
		}
		parsed := parseAIGenreBatch(answer, len(mfs))
		usable := 0
		for _, r := range parsed {
			if r.Genre != "" {
				usable++
			}
		}
		if usable > 0 {
			results = parsed
			break
		}
		log.Debug(ctx, "AI genre batch answer unusable", "songs", len(mfs), "attempt", attempt, "answer", answer)
	}

	for i := range results {
		if results[i].Genre == "" {
			genre, subgenre := fetchAIGenre(ctx, provider, mfs[i])
			results[i] = aiGenreResult{Genre: genre, Subgenre: subgenre}
		}
	}
	return results
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

// aiGenreBatchPrompt asks for every numbered song in one prompt. The
// instruction block matches the single-song prompt; only the output shape
// (a JSON array with one object per song) and the song list differ.
func aiGenreBatchPrompt(mfs []*model.MediaFile) string {
	var b strings.Builder
	b.WriteString(`You are an expert musicologist. Classify the genre of each numbered song below, each song independently.

Work through it in this order for every song:
1. Identify the exact recording from the title, artist, album, and year given. Title or artist tags may contain junk (file names, "[Unknown Artist]", featured-artist suffixes) — extract the real song from them when possible.
2. If you know the exact song, classify the SONG itself, not the artist's reputation: a country song by a pop artist is Country, a rap verse over a pop beat is judged by the overall track.
3. If you cannot identify the exact song, classify it by the artist's dominant style in that album/year era.
4. If the artist is unknown too, infer the most likely genre from the title and album name.

Return only valid JSON, no markdown: a JSON array with exactly one object per song, in the same order as the songs, in exactly this shape:
[{"song":1,"genre":"primary genre","subgenre":"more specific style or empty string","basis":"song|artist|inferred","confidence":0}]

Rules:
- genre MUST be one of: `)
	b.WriteString(aiGenrePrimaryGenres)
	b.WriteString(`.
- subgenre is the single most specific style that fits (e.g. "Electropop", "Grunge", "Country Soul", "East Coast Hip Hop"); use an empty string only when nothing more specific applies.
- genre must NEVER be empty. Always return your best classification for every song, even at low confidence.
- confidence is a whole number 0-100: 90+ when you recognize the exact recording, 60-89 when classifying from the artist, below 60 when inferring.

Songs:
`)
	for i, mf := range mfs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(".\nTitle: ")
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
		b.WriteString("\n")
	}
	return b.String()
}

// parseAIGenreBatch extracts one result per song from the model's JSON array
// answer. Entries are matched by their "song" number, falling back to array
// position when the numbers are unusable; missing songs stay empty so the
// caller can fetch them individually.
func parseAIGenreBatch(answer string, n int) []aiGenreResult {
	results := make([]aiGenreResult, n)
	answer = strings.TrimSpace(answer)
	start := strings.Index(answer, "[")
	end := strings.LastIndex(answer, "]")
	if start < 0 || end <= start {
		return results
	}

	var raw []struct {
		Song     int    `json:"song"`
		Genre    string `json:"genre"`
		Subgenre string `json:"subgenre"`
	}
	if err := json.Unmarshal([]byte(answer[start:end+1]), &raw); err != nil {
		return results
	}
	positional := len(raw) == n
	for pos, entry := range raw {
		idx := entry.Song - 1
		if idx < 0 || idx >= n {
			if !positional {
				continue
			}
			idx = pos
		}
		genre := strings.TrimSpace(entry.Genre)
		if genre == "" || results[idx].Genre != "" {
			continue
		}
		subgenre := strings.TrimSpace(entry.Subgenre)
		if strings.EqualFold(subgenre, genre) {
			subgenre = ""
		}
		results[idx] = aiGenreResult{Genre: genre, Subgenre: subgenre}
	}
	return results
}

// parseAIGenre extracts the primary genre and subgenre from the model's JSON
// answer. Returns empty strings for an unusable answer so the caller can
// retry; the subgenre is dropped when it is empty or merely repeats the genre.
func parseAIGenre(answer string) (genre, subgenre string) {
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
		return "", ""
	}
	genre = strings.TrimSpace(raw.Genre)
	if genre == "" {
		return "", ""
	}
	subgenre = strings.TrimSpace(raw.Subgenre)
	if strings.EqualFold(subgenre, genre) {
		subgenre = ""
	}
	return genre, subgenre
}
