package nativeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/model"
)

func ListFoldersAndPlaylists(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		opts := parseQueryOptions(r)

		folders, err := ds.PlaylistFolder(ctx).GetAllByParent(opts.FolderOpts)
		if err != nil {
			http.Error(w, err.Error(), statusFor(err)); return
		}

		playlists, err := ds.Playlist(ctx).GetAllByPlaylistFolder(opts.PlaylistOpts)
		if err != nil {
			http.Error(w, err.Error(), statusFor(err)); return
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
		if start > total { start = total }
		if end > total { end = total }

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", fmt.Sprintf("%d", total))
		w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")
		_ = json.NewEncoder(w).Encode(commons[start:end])
	}
}

func MoveFolder(ds model.DataStore) http.HandlerFunc {
	type reqBody struct{ ParentID *string `json:"parentId"` }

	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body reqBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest); return
		}
		if body.ParentID != nil && *body.ParentID == "" {
			http.Error(w, "parentId cannot be empty; use null for root", http.StatusBadRequest); return
		}
		if err := ds.PlaylistFolder(r.Context()).UpdateParent(id, body.ParentID); err != nil {
			http.Error(w, err.Error(), statusFor(err)); return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func BulkMove(ds model.DataStore) http.HandlerFunc {
	type movePayload struct {
		Ids      []string `json:"ids"`
		Types    []string `json:"types"`    // "folder" | "playlist"
		FolderID *string  `json:"folderId"` // null => root
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var payload movePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest); return
		}
		if len(payload.Ids) != len(payload.Types) {
			http.Error(w, "ids and types length mismatch", http.StatusBadRequest); return
		}
		if payload.FolderID != nil && *payload.FolderID == "" {
			http.Error(w, "folderId cannot be empty; use null for root/unassigned", http.StatusBadRequest); return
		}

		ctx := r.Context()
		updated := 0
		for i, id := range payload.Ids {
			switch payload.Types[i] {
			case "playlist":
				if err := ds.Playlist(ctx).UpdatePlaylistFolder(id, payload.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err)); return
				}
				updated++
			case "folder":
				if err := ds.PlaylistFolder(ctx).UpdateParent(id, payload.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err)); return
				}
				updated++
			default:
				http.Error(w, "invalid type "+payload.Types[i], http.StatusBadRequest); return
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
}

func parseQueryOptions(r *http.Request) DualQueryOptions {
    q := r.URL.Query()

    base := model.QueryOptions{Order: "ASC", Filters: And{}}
    out := DualQueryOptions{Offset: 0, Max: 100}

    if sortField := q.Get("_sort"); sortField != "" {
        base.Sort = sortField
    }
    if order := q.Get("_order"); order != "" {
        base.Order = strings.ToUpper(order)
    }
    if start, err := strconv.Atoi(q.Get("_start")); err == nil {
        out.Offset = start
    }
    if end, err := strconv.Atoi(q.Get("_end")); err == nil && end > out.Offset {
        out.Max = end - out.Offset
    }

    folderOpts, playlistOpts := base, base

    if search := strings.TrimSpace(q.Get("q")); search != "" {
        folderOpts.Filters = append(folderOpts.Filters.(And), playlistFolderFilter("q", search))
        playlistOpts.Filters = append(playlistOpts.Filters.(And), playlistFilter("q", search))
    }

    if raw, present := q["parent_id"]; present {
        v := ""
        if len(raw) > 0 { v = raw[0] }
        if v == "" || strings.EqualFold(v, "null") {
            folderOpts.Filters   = append(folderOpts.Filters.(And), Eq{"parent_id":  nil})
            playlistOpts.Filters = append(playlistOpts.Filters.(And), Eq{"folder_id": nil})
        } else {
            folderOpts.Filters   = append(folderOpts.Filters.(And), Eq{"parent_id":  v})
            playlistOpts.Filters = append(playlistOpts.Filters.(And), Eq{"folder_id": v})
        }
    }

    if owner := strings.TrimSpace(q.Get("owner_id")); owner != "" {
        folderOpts.Filters   = append(folderOpts.Filters.(And), Eq{"owner_id": owner})
        playlistOpts.Filters = append(playlistOpts.Filters.(And), Eq{"owner_id": owner})
    }

    out.FolderOpts, out.PlaylistOpts = folderOpts, playlistOpts
    return out
}

func playlistFilter(_ string, value interface{}) Sqlizer {
	return Or{
		substringFilter("playlist.name", value),
		substringFilter("playlist.comment", value),
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
