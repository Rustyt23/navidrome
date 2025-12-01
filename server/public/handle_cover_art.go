package public

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
)

func (pub *Router) handleGetCoverArt() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		options := parseRestQueryOptions(r.URL.Query())
		repo := pub.ds.MediaFile(r.Context())

		entities, err := repo.ReadAll(options)
		if err != nil {
			rest.RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		mediaFiles, _ := entities.(model.MediaFiles)
		usedFallback := false
		if len(mediaFiles) == 0 {
			usedFallback = true
			if fallback, err := searchCoverArtFallback(repo, options); err == nil {
				mediaFiles = fallback
			} else {
				rest.RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		count := len(mediaFiles)
		if count == 0 {
			w.Header().Set("X-Total-Count", "0")
			rest.RespondWithJSON(w, http.StatusOK, &mediaFiles)
			return
		}

		if !usedFallback {
			if total, err := repo.Count(options); err == nil {
				count = int(total)
			}
		}

		w.Header().Set("X-Total-Count", strconv.Itoa(count))
		rest.RespondWithJSON(w, http.StatusOK, &mediaFiles)
	}
}

func parseRestQueryOptions(params url.Values) rest.QueryOptions {
	start, _ := strconv.Atoi(params.Get("_start"))
	end, _ := strconv.Atoi(params.Get("_end"))

	return rest.QueryOptions{
		Sort:    params.Get("_sort"),
		Order:   strings.ToLower(params.Get("_order")),
		Offset:  start,
		Max:     int(math.Max(0, float64(end-start))),
		Filters: parseFilters(params),
	}
}

func parseFilters(params url.Values) map[string]any {
	filters := map[string]any{}
	if filterStr := params.Get("_filters"); filterStr != "" {
		if decoded, err := url.QueryUnescape(filterStr); err == nil {
			_ = json.Unmarshal([]byte(decoded), &filters)
		}
	}
	for k, v := range params {
		if strings.HasPrefix(k, "_") {
			continue
		}
		if len(v) == 1 {
			filters[k] = v[0]
		} else {
			filters[k] = v
		}
	}
	return filters
}

func searchCoverArtFallback(repo model.MediaFileRepository, options rest.QueryOptions) (model.MediaFiles, error) {
	title := firstString(options.Filters["title"])
	artist := firstString(options.Filters["artist"])
	q := strings.TrimSpace(strings.Join([]string{title, artist}, " "))
	if q == "" {
		return nil, nil
	}

	queryOptions := model.QueryOptions{Offset: options.Offset, Max: options.Max, Filters: buildSearchFilters(options.Filters)}
	return repo.Search(q, options.Offset, options.Max, queryOptions)
}

func buildSearchFilters(filters map[string]any) sq.Sqlizer {
	var conditions sq.And

	if v, ok := filters["missing"]; ok {
		val := strings.ToLower(fmt.Sprint(v)) == "true"
		conditions = append(conditions, sq.Eq{"media_file.missing": val})
	}
	if v, ok := filters["library_id"]; ok {
		conditions = append(conditions, sq.Eq{"media_file.library_id": v})
	}

	if len(conditions) == 0 {
		return nil
	}
	return conditions
}

func firstString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	}
	return ""
}
