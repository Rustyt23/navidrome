package rag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/navidrome/navidrome/model"
)

const (
	DefaultIndexLimit = 50
	MaxIndexLimit     = 500
	indexPageSize     = 50
)

// Indexer defines a provider-independent document-indexing boundary.
type Indexer interface {
	Index(context.Context, []RAGDocument) error
}

// SongRepository is the read-only subset of Navidrome's media repository used
// by the RAG indexer.
type SongRepository interface {
	GetAll(options ...model.QueryOptions) (model.MediaFiles, error)
}

// VectorStore is the Qdrant subset used by the indexing pipeline.
type VectorStore interface {
	PointExists(ctx context.Context, logicalID string) (bool, error)
	UpsertPoint(ctx context.Context, logicalID string, vector []float32, payload map[string]any) error
}

// IndexResult is returned by the first bounded song indexing operation.
type IndexResult struct {
	Indexed int    `json:"indexed"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
	Error   string `json:"error,omitempty"`
}

// IndexSongs indexes up to MaxIndexLimit new songs and writes only their
// derived vectors to the configured vector store. Existing points are skipped
// while the library is scanned in bounded pages. It never updates Navidrome
// records.
func IndexSongs(
	ctx context.Context,
	repository SongRepository,
	embedder Embedder,
	store VectorStore,
	limit int,
	force bool,
) (IndexResult, error) {
	result := IndexResult{}
	if limit <= 0 {
		limit = DefaultIndexLimit
	}
	if limit > MaxIndexLimit {
		return result, fmt.Errorf("index limit must not exceed %d", MaxIndexLimit)
	}

	for offset := 0; result.Indexed+result.Failed < limit; {
		pageSize := min(indexPageSize, limit-result.Indexed-result.Failed)
		songs, err := repository.GetAll(model.QueryOptions{
			Sort:   "id",
			Order:  "ASC",
			Max:    pageSize,
			Offset: offset,
		})
		if err != nil {
			return result, fmt.Errorf("could not read songs: %w", err)
		}
		if len(songs) == 0 {
			break
		}
		offset += len(songs)

		for i := range songs {
			song := songs[i]
			logicalID := StableSongPointID(song.ID)
			if !force {
				exists, err := store.PointExists(ctx, logicalID)
				if err != nil {
					recordIndexFailure(&result, fmt.Errorf("could not check song %q in Qdrant: %w", song.ID, err))
					if result.Indexed+result.Failed == limit {
						break
					}
					continue
				}
				if exists {
					result.Skipped++
					continue
				}
			}

			document := DocumentFromMediaFile(song)
			vector, err := embedder.EmbedText(ctx, document.Text)
			if err != nil {
				recordIndexFailure(&result, fmt.Errorf("could not embed song %q: %w", song.ID, err))
				if result.Indexed+result.Failed == limit {
					break
				}
				continue
			}
			if err := store.UpsertPoint(ctx, logicalID, vector, songPayload(song)); err != nil {
				recordIndexFailure(&result, fmt.Errorf("could not upsert song %q: %w", song.ID, err))
				if result.Indexed+result.Failed == limit {
					break
				}
				continue
			}
			result.Indexed++
			if result.Indexed+result.Failed == limit {
				break
			}
		}

		if len(songs) < pageSize {
			break
		}
	}

	return result, nil
}

func recordIndexFailure(result *IndexResult, err error) {
	result.Failed++
	if result.Error == "" && err != nil {
		result.Error = err.Error()
	}
}

func songPayload(song model.MediaFile) map[string]any {
	genre := strings.TrimSpace(song.Genre)
	if values := song.Tags.Values(model.TagGenre); len(values) > 0 {
		genre = strings.Join(values, ", ")
	}
	return map[string]any{
		"songId":   song.ID,
		"type":     "song",
		"title":    song.Title,
		"artist":   song.Artist,
		"album":    song.Album,
		"year":     song.Year,
		"genre":    genre,
		"explicit": song.ExplicitStatus == "e",
		"bpm":      song.BPM,
		"lufs":     songLUFS(song),
	}
}

func songLUFS(song model.MediaFile) float64 {
	for _, name := range []model.TagName{
		"loudnorm_final_lufs",
		"final_lufs",
		"finallufs",
		"lufs",
	} {
		for _, value := range song.Tags.Values(name) {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

var _ VectorStore = (*QdrantClient)(nil)
