package nativeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
)

const (
	discoveryItemTypeFolder = "folder"
	discoveryItemTypeFile   = "file"
)

var discoveryAudioExtensions = map[string]struct{}{
	".mp3":  {},
	".m4a":  {},
	".flac": {},
	".wav":  {},
	".ogg":  {},
}

type discoveryItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type discoveryListResponse struct {
	Path  string          `json:"path"`
	Items []discoveryItem `json:"items"`
}

func (n *Router) addDiscoveryFSRoute(r chi.Router) {
	r.Route("/discoveryfs", func(r chi.Router) {
		r.Get("/list", n.discoveryListHandler)
		r.Post("/folder", n.discoveryCreateFolderHandler)
		r.Post("/upload", n.discoveryUploadHandler)
	})
}

func (n *Router) discoveryListHandler(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	clean, abs, err := sanitizeDiscoveryPath(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "path not found", http.StatusNotFound)
			return
		}
		http.Error(w, "unable to list directory", http.StatusInternalServerError)
		return
	}

	items := make([]discoveryItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			items = append(items, discoveryItem{Name: entry.Name(), Type: discoveryItemTypeFolder})
			continue
		}
		if isAllowedDiscoveryFile(entry.Name()) {
			items = append(items, discoveryItem{Name: entry.Name(), Type: discoveryItemTypeFile})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Type == items[j].Type {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return items[i].Type == discoveryItemTypeFolder
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(discoveryListResponse{Path: clean, Items: items})
}

func (n *Router) discoveryCreateFolderHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	_, parentAbs, err := sanitizeDiscoveryPath(payload.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		http.Error(w, "folder name required", http.StatusBadRequest)
		return
	}
	if name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		http.Error(w, "invalid folder name", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(parentAbs); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "path not found", http.StatusNotFound)
			return
		}
		http.Error(w, "unable to create folder", http.StatusInternalServerError)
		return
	}

	folderPath := filepath.Join(parentAbs, name)
	if err := os.Mkdir(folderPath, os.ModePerm); err != nil {
		if errors.Is(err, os.ErrExist) {
			http.Error(w, "folder already exists", http.StatusConflict)
			return
		}
		http.Error(w, "unable to create folder", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (n *Router) discoveryUploadHandler(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	_, abs, err := sanitizeDiscoveryPath(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(abs); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "path not found", http.StatusNotFound)
			return
		}
		http.Error(w, "unable to upload", http.StatusInternalServerError)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files uploaded", http.StatusBadRequest)
		return
	}

	uploaded := make([]string, 0, len(files))
	for _, fh := range files {
		name := filepath.Base(fh.Filename)
		if name == "" {
			http.Error(w, "invalid file name", http.StatusBadRequest)
			return
		}
		if !isAllowedDiscoveryFile(name) {
			http.Error(w, fmt.Sprintf("unsupported file type: %s", name), http.StatusBadRequest)
			return
		}

		src, err := fh.Open()
		if err != nil {
			http.Error(w, "unable to read uploaded file", http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(abs, name)
		dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
		if err != nil {
			src.Close()
			http.Error(w, "unable to save file", http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			http.Error(w, "unable to save file", http.StatusInternalServerError)
			return
		}
		dst.Close()
		src.Close()
		uploaded = append(uploaded, name)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"uploaded": uploaded})
}

func sanitizeDiscoveryPath(rel string) (string, string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		root := conf.DiscoveryRoot()
		return "", root, nil
	}

	rel = strings.ReplaceAll(rel, "\\", "/")
	if strings.HasPrefix(rel, "/") {
		return "", "", errors.New("invalid path")
	}
	if filepath.IsAbs(rel) {
		return "", "", errors.New("invalid path")
	}

	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return "", "", errors.New("invalid path")
		}
	}

	clean := path.Clean(rel)
	if clean == "." {
		return "", conf.DiscoveryRoot(), nil
	}

	parts := strings.Split(clean, "/")
	for _, p := range parts {
		if p == ".." || p == "" {
			return "", "", errors.New("invalid path")
		}
	}

	root := conf.DiscoveryRoot()
	abs := filepath.Join(append([]string{root}, parts...)...)
	normalized := strings.Join(parts, "/")
	return normalized, abs, nil
}

func isAllowedDiscoveryFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := discoveryAudioExtensions[ext]
	return ok
}
