package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/model"
)

func ListFoldersAndPlaylists(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		opts := parseQueryOptions(r)

		if opts.Search != "" {
			descendants, err := collectDescendantFolderIDs(ctx, ds.PlaylistFolder(ctx), opts.Parent)
			if err != nil {
				http.Error(w, err.Error(), statusFor(err))
				return
			}

			opts.FolderOpts.Filters = appendFilter(opts.FolderOpts.Filters, folderIDsCondition(descendants))
			opts.PlaylistOpts.Filters = appendFilter(opts.PlaylistOpts.Filters, playlistFoldersCondition(descendants, opts.Parent))
		}

		folders, err := ds.PlaylistFolder(ctx).GetAllByParent(opts.FolderOpts)
		if err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}

		playlists, err := ds.Playlist(ctx).GetAllByPlaylistFolder(opts.PlaylistOpts)
		if err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}

		commons := make(model.PlaylistAndFolderCommons, 0, len(folders)+len(playlists))
		for _, f := range folders {
			commons = append(commons, &model.PlaylistAndFolderCommon{
				ID:        f.ID,
				Name:      f.Name,
				OwnerID:   f.OwnerID,
				OwnerName: f.OwnerName,
				UpdatedAt: f.UpdatedAt,
				Public:    f.Public,
				Type:      "folder",
			})
		}
		for _, p := range playlists {
			commons = append(commons, &model.PlaylistAndFolderCommon{
				ID:        p.ID,
				Name:      p.Name,
				OwnerID:   p.OwnerID,
				OwnerName: p.OwnerName,
				UpdatedAt: p.UpdatedAt,
				Public:    p.Public,
				Type:      "playlist",
			})
		}

		total := len(commons)
		start, end := opts.Offset, opts.Offset+opts.Max
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", fmt.Sprintf("%d", total))
		w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")
		_ = json.NewEncoder(w).Encode(commons[start:end])
	}
}

func MoveFolder(ds model.DataStore) http.HandlerFunc {
	type reqBody struct {
		ParentID *string `json:"parentId"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body reqBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.ParentID != nil && *body.ParentID == "" {
			http.Error(w, "parentId cannot be empty; use null for root", http.StatusBadRequest)
			return
		}
		if err := ds.PlaylistFolder(r.Context()).UpdateParent(id, body.ParentID); err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func BulkMove(ds model.DataStore, pls core.Playlists) http.HandlerFunc {
	type movePayload struct {
		Ids      []string `json:"ids"`
		Types    []string `json:"types"`    // "folder" | "playlist"
		FolderID *string  `json:"folderId"` // null => root
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var payload movePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(payload.Ids) != len(payload.Types) {
			http.Error(w, "ids and types length mismatch", http.StatusBadRequest)
			return
		}
		if payload.FolderID != nil && *payload.FolderID == "" {
			http.Error(w, "folderId cannot be empty; use null for root/unassigned", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		updated := 0
		for i, id := range payload.Ids {
			switch payload.Types[i] {
			case "playlist":
				if err := pls.SetFolder(ctx, id, payload.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err))
					return
				}
				updated++
			case "folder":
				if err := ds.PlaylistFolder(ctx).UpdateParent(id, payload.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err))
					return
				}
				updated++
			default:
				http.Error(w, "invalid type "+payload.Types[i], http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"updated":%d}`, updated)))
	}
}

type DualQueryOptions struct {
	FolderOpts   model.QueryOptions
	PlaylistOpts model.QueryOptions
	Offset       int
	Max          int
	Search       string
	Parent       *string
}

func parseQueryOptions(r *http.Request) DualQueryOptions {
	q := r.URL.Query()

	base := model.QueryOptions{Order: "ASC", Filters: And{}}
	folderOpts, playlistOpts := base, base
	folderOpts.Filters = And{}
	playlistOpts.Filters = And{}

	out := DualQueryOptions{Offset: 0, Max: 100, Parent: nil}

	if sortField := q.Get("_sort"); sortField != "" {
		base.Sort = sortField
		folderOpts.Sort = sortField
		playlistOpts.Sort = sortField
	}
	if order := q.Get("_order"); order != "" {
		upper := strings.ToUpper(order)
		base.Order = upper
		folderOpts.Order = upper
		playlistOpts.Order = upper
	}
	if start, err := strconv.Atoi(q.Get("_start")); err == nil {
		out.Offset = start
	}
	if end, err := strconv.Atoi(q.Get("_end")); err == nil && end > out.Offset {
		out.Max = end - out.Offset
	}

	search := strings.TrimSpace(q.Get("q"))
	out.Search = search
	if search != "" {
		folderOpts.Filters = appendFilter(folderOpts.Filters, playlistFolderFilter("q", search))
		playlistOpts.Filters = appendFilter(playlistOpts.Filters, playlistFilter("q", search))
	}

	if raw, present := q["parent_id"]; present {
		if len(raw) > 0 {
			v := strings.TrimSpace(raw[0])
			if v != "" && !strings.EqualFold(v, "null") {
				parent := v
				out.Parent = &parent
			}
		}
	}

	if search == "" {
		folderOpts.Filters = appendFilter(folderOpts.Filters, parentCondition(out.Parent))
		playlistOpts.Filters = appendFilter(playlistOpts.Filters, playlistParentCondition(out.Parent))
	}

	if owner := strings.TrimSpace(q.Get("owner_id")); owner != "" {
		folderOpts.Filters = appendFilter(folderOpts.Filters, Eq{"owner_id": owner})
		playlistOpts.Filters = appendFilter(playlistOpts.Filters, Eq{"owner_id": owner})
	}

	folderOpts.Sort = base.Sort
	folderOpts.Order = base.Order
	playlistOpts.Sort = base.Sort
	playlistOpts.Order = base.Order

	out.FolderOpts, out.PlaylistOpts = folderOpts, playlistOpts
	return out
}

func playlistFilter(_ string, value interface{}) Sqlizer {
	songMatch := Or{
		substringFilter("mf.title", value),
		substringFilter("mf.artist", value),
		substringFilter("mf.album", value),
	}

	sub := Select("1").
		From("playlist_tracks pt").
		Join("media_file mf on mf.id = pt.media_file_id").
		Where(And{
			Eq{"pt.playlist_id": Expr("playlist.id")},
			songMatch,
		})

	return Or{
		substringFilter("playlist.name", value),
		substringFilter("playlist.comment", value),
		Expr("exists (?)", sub),
	}
}

func playlistFolderFilter(_ string, value interface{}) Sqlizer {
	return substringFilter("playlist_folder.name", value)
}

func substringFilter(field string, value any) Sqlizer {
	parts := strings.Fields(value.(string))
	filters := And{}
	for _, part := range parts {
		filters = append(filters, Like{field: "%" + part + "%"})
	}
	return filters
}

func appendFilter(base Sqlizer, extra Sqlizer) Sqlizer {
	if extra == nil {
		return base
	}
	if base == nil {
		return extra
	}
	if and, ok := base.(And); ok {
		return append(and, extra)
	}
	return And{base, extra}
}

func parentCondition(parent *string) Sqlizer {
	if parent == nil {
		return Eq{"parent_id": nil}
	}
	return Eq{"parent_id": *parent}
}

func playlistParentCondition(parent *string) Sqlizer {
	if parent == nil {
		return Eq{"folder_id": nil}
	}
	return Eq{"folder_id": *parent}
}

func folderIDsCondition(ids []string) Sqlizer {
	if len(ids) == 0 {
		return Eq{"1": 0}
	}
	return Eq{"playlist_folder.id": uniqueStrings(ids)}
}

func playlistFoldersCondition(ids []string, parent *string) Sqlizer {
	allowed := uniqueStrings(ids)
	if parent != nil {
		allowed = appendIfMissing(allowed, *parent)
	}
	if len(allowed) == 0 {
		if parent == nil {
			return Eq{"playlist.folder_id": nil}
		}
		return Eq{"1": 0}
	}
	filter := Eq{"playlist.folder_id": allowed}
	if parent == nil {
		return Or{
			filter,
			Eq{"playlist.folder_id": nil},
		}
	}
	return filter
}

func appendIfMissing(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func collectDescendantFolderIDs(ctx context.Context, repo model.PlaylistFolderRepository, parent *string) ([]string, error) {
	queue := []*string{parent}
	result := make([]string, 0)
	visited := make(map[string]struct{})

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		opts := model.QueryOptions{Filters: parentCondition(current)}
		children, err := repo.GetAllByParent(opts)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			if _, ok := visited[child.ID]; ok {
				continue
			}
			visited[child.ID] = struct{}{}
			result = append(result, child.ID)
			childID := child.ID
			queue = append(queue, &childID)
		}
	}

	return result, nil
}
