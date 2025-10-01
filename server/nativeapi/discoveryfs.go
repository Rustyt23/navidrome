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
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
)

var audioExtensions = map[string]struct{}{
	".mp3":  {},
	".flac": {},
	".ogg":  {},
	".oga":  {},
	".wav":  {},
	".aac":  {},
	".m4a":  {},
	".wma":  {},
	".alac": {},
	".aiff": {},
	".dsf":  {},
	".dff":  {},
}

type discoveryFSHandler struct {
	root string
}

type discoveryItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type discoveryListResponse struct {
	Path    string          `json:"path"`
	Folders []discoveryItem `json:"folders"`
	Files   []discoveryItem `json:"files"`
}

func (n *Router) addDiscoveryFSRoute(r chi.Router) {
	handler := discoveryFSHandler{root: conf.Server.MusicFolder}

	r.Route("/discoveryfs", func(r chi.Router) {
		r.Get("/", handler.list)
		r.Post("/folder", handler.createFolder)
		r.Post("/upload", handler.upload)
	})
}

func (h discoveryFSHandler) list(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	abs, clean, err := h.resolve(rel)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := discoveryListResponse{Path: clean}
	for _, entry := range entries {
		name := entry.Name()
		item := discoveryItem{Name: name, Path: joinRelative(clean, name)}
		if entry.IsDir() {
			resp.Folders = append(resp.Folders, item)
			continue
		}
		if isAudioFile(name) {
			resp.Files = append(resp.Files, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type createFolderRequest struct {
	Name string `json:"name"`
}

func (h discoveryFSHandler) createFolder(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	abs, _, err := h.resolve(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req createFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "folder name is required", http.StatusBadRequest)
		return
	}
	if name == "." || name == ".." {
		http.Error(w, "invalid folder name", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(name, "/\\") {
		http.Error(w, "folder name cannot contain path separators", http.StatusBadRequest)
		return
	}

	target := filepath.Join(abs, name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h discoveryFSHandler) upload(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	abs, _, err := h.resolve(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		http.Error(w, "no files provided", http.StatusBadRequest)
		return
	}

	uploaded := 0
	for _, fh := range files {
		filename := filepath.Base(fh.Filename)
		if !isAudioFile(filename) {
			http.Error(w, fmt.Sprintf("file %s is not an audio file", filename), http.StatusBadRequest)
			return
		}
		src, err := fh.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		dstPath := filepath.Join(abs, filename)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			src.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dst.Close()
		src.Close()
		uploaded++
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"uploaded": uploaded})
}

func (h discoveryFSHandler) resolve(rel string) (string, string, error) {
	root := strings.TrimSpace(h.root)
	if root == "" {
		return "", "", errors.New("music folder not configured")
	}

	cleanInput := strings.TrimSpace(rel)
	cleanInput = strings.TrimPrefix(cleanInput, string(os.PathSeparator))
	cleanInput = filepath.Clean(cleanInput)
	if cleanInput == "." {
		cleanInput = ""
	}

	rootClean := filepath.Clean(root)
	absClean := filepath.Clean(filepath.Join(rootClean, cleanInput))
	relCheck, err := filepath.Rel(rootClean, absClean)
	if err != nil {
		return "", "", errors.New("invalid path")
	}
	if relCheck == ".." || strings.HasPrefix(relCheck, "../") || strings.HasPrefix(relCheck, "..\\") {
		return "", "", errors.New("invalid path")
	}

	clean := filepath.ToSlash(cleanInput)
	if clean == "." {
		clean = ""
	}

	if clean == "" {
		return absClean, "", nil
	}
	return absClean, clean, nil
}

func joinRelative(base, name string) string {
	return strings.TrimPrefix(path.Join(base, name), "./")
}

func isAudioFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := audioExtensions[ext]
	return ok
}
