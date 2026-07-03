package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/metrics"
	playlistsvc "github.com/navidrome/navidrome/core/playlists"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/server"
	"github.com/navidrome/navidrome/core/publicurl"
)

const (
	coverArtDefaultSize = consts.DefaultUICoverArtSize
	coverCacheDirName   = "covercache"
)

var releaseMBIDRegex = regexp.MustCompile(`^[a-fA-F0-9-]+$`)

var defaultCoverPlaceholder = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x04, 0x00, 0x00, 0x00, 0xB5, 0x1C, 0x0C, 0x02, 0x00, 0x00, 0x00,
	0x0B, 0x49, 0x44, 0x41, 0x54, 0x78, 0xDA, 0x63, 0xFC, 0xFF, 0x1F, 0x00,
	0x03, 0x03, 0x02, 0x00, 0xEF, 0x26, 0x05, 0x9B, 0x00, 0x00, 0x00, 0x00,
	0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// PluginManager defines the interface for plugin management operations.
// This interface is used by the API handlers to enable/disable plugins and update configuration.
type PluginManager interface {
	EnablePlugin(ctx context.Context, id string) error
	DisablePlugin(ctx context.Context, id string) error
	ValidatePluginConfig(ctx context.Context, id, configJSON string) error
	UpdatePluginConfig(ctx context.Context, id, configJSON string) error
	UpdatePluginUsers(ctx context.Context, id, usersJSON string, allUsers bool) error
	UpdatePluginLibraries(ctx context.Context, id, librariesJSON string, allLibraries, allowWriteAccess bool) error
	RescanPlugins(ctx context.Context) error
	UnloadDisabledPlugins(ctx context.Context)
}

type Router struct {
	http.Handler
	ds            model.DataStore
	share         core.Share
	playlists     playlistsvc.Playlists
	insights      metrics.Insights
	libs          core.Library
	users         core.User
	maintenance   core.Maintenance
	pluginManager PluginManager
	imgUpload     core.ImageUploadService
	devices       *retailPlayerDeviceResolver
	metadataJob   *musicBrainzMetadataJob
	spotifyJob    *spotifyMetadataJob
}

func New(ds model.DataStore, share core.Share, playlists playlistsvc.Playlists, insights metrics.Insights, libraryService core.Library, userService core.User, maintenance core.Maintenance, pluginManager PluginManager, imgUpload core.ImageUploadService) *Router {
	r := &Router{
		ds:            ds,
		share:         share,
		playlists:     playlists,
		insights:      insights,
		libs:          libraryService,
		users:         userService,
		maintenance:   maintenance,
		pluginManager: pluginManager,
		imgUpload:     imgUpload,
		devices:       newRetailPlayerDeviceResolver(),
		metadataJob:   newMusicBrainzMetadataJob(),
		spotifyJob:    newSpotifyMetadataJob(),
	}
	r.ensureCoverCacheDir()
	r.preloadRetailPlayerDeviceMappings()
	r.Handler = r.routes()
	return r
}

func (n *Router) preloadRetailPlayerDeviceMappings() {
	if n.ds == nil || n.devices == nil {
		return
	}

	ctx := context.Background()
	repo := n.ds.RetailPlayerDeviceMapping(ctx)
	if repo == nil {
		return
	}

	mappings, err := allRetailPlayerDeviceMappings(ctx, repo)
	if err != nil {
		log.Error(ctx, "Unable to preload retail player device mappings", "err", err)
		return
	}

	if len(mappings) == 0 {
		return
	}

	devices := make([]retailPlayerDevice, 0, len(mappings))
	for _, mapping := range mappings {
		id := strings.TrimSpace(mapping.DeviceID)
		if id == "" {
			continue
		}

		devices = append(devices, retailPlayerDevice{
			ID:           id,
			Name:         strings.TrimSpace(mapping.DeviceName),
			Channel:      strings.TrimSpace(mapping.Channel),
			ChannelList:  strings.TrimSpace(mapping.ChannelList),
			Organization: strings.TrimSpace(mapping.Organization),
			TimeZone:     strings.TrimSpace(mapping.TimeZone),
		})
	}

	n.devices.RememberDevices(devices)
}

func allRetailPlayerDeviceMappings(ctx context.Context, repo model.RetailPlayerDeviceMappingRepository) (mappings []model.RetailPlayerDeviceMapping, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("retail player device mapping repository unavailable: %v", recovered)
		}
	}()
	return repo.All(ctx)
}
func (api *Router) routes() http.Handler {
	r := chi.NewRouter()

	// Public
	api.addRetailPlayerPublicRoutes(r)
	api.addCoverRoute(r)
	api.addSongRoute(r)
	api.RX(r, "/translation", newTranslationRepository, false)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(server.Authenticator(api.ds))
		r.Use(server.JWTRefresher)
		r.Use(server.UpdateLastAccessMiddleware(api.ds))
		api.RX(r, "/user", api.users.NewRepository, true)
		api.R(r, "/album", model.Album{}, false)
		api.addArtistRoute(r)
		api.R(r, "/genre", model.Genre{}, false)
		api.R(r, "/player", model.Player{}, true)
		api.R(r, "/transcoding", model.Transcoding{}, conf.Server.EnableTranscodingConfig)
		api.addRadioRoute(r)
		api.R(r, "/tag", model.Tag{}, true)
		if conf.Server.EnableSharing {
			api.RX(r, "/share", api.share.NewRepository, true)
		}

		api.addPlaylistRoute(r)
		api.addPlaylistFolderRoute(r)
		api.addPlaylistTrackRoute(r)
		api.addDiscoveryRoute(r)
		api.addSongPlaylistsRoute(r)
		api.addSongDiscoveriesRoute(r)
		api.addSongCommentRoute(r)
		api.addQueueRoute(r)
		api.addMissingFilesRoute(r)
		api.addNotificationsRoute(r)
		api.addKeepAliveRoute(r)
		api.addInsightsRoute(r)
		api.addRetailPlayerPrivateRoutes(r)
		api.addAIChatRoute(r)

		r.With(adminOnlyMiddleware).Group(func(r chi.Router) {
			api.addInspectRoute(r)
			api.addConfigRoute(r)
			api.addUserLibraryRoute(r)
			api.addSyncRoute(r)
			api.addSongLoudnessRoute(r)
			api.addMusicBrainzMetadataRoute(r)
			api.addPluginRoute(r)
			api.RX(r, "/library", api.libs.NewRepository, true)
		})
	})

	return r
}

func (api *Router) R(r chi.Router, pathPrefix string, model any, persistable bool) {
	constructor := func(ctx context.Context) rest.Repository {
		return api.ds.Resource(ctx, model)
	}
	api.RX(r, pathPrefix, constructor, persistable)
}

func (api *Router) RX(r chi.Router, pathPrefix string, constructor rest.RepositoryConstructor, persistable bool) {
	r.Route(pathPrefix, func(r chi.Router) {
		r.Get("/", rest.GetAll(constructor))
		if persistable {
			r.Post("/", rest.Post(constructor))
		}
		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", rest.Get(constructor))
			if persistable {
				r.Put("/", rest.Put(constructor))
				r.Delete("/", rest.Delete(constructor))
			}
		})
	})
}

func (n *Router) coverCacheDir() string {
	return filepath.Join(conf.Server.DataFolder.String(), coverCacheDirName)
}

func (n *Router) ensureCoverCacheDir() {
	if err := os.MkdirAll(n.coverCacheDir(), 0o755); err != nil {
		log.Error(context.Background(), "Could not create cover cache directory", "dir", n.coverCacheDir(), "err", err)
	}
}

func (n *Router) coverFilePath(releaseMBID string) string {
	return filepath.Join(n.coverCacheDir(), releaseMBID+".jpg")
}

func (n *Router) addCoverRoute(r chi.Router) {
	r.Get("/cover/{releaseMBID}", func(w http.ResponseWriter, req *http.Request) {
		releaseMBID := strings.TrimSpace(chi.URLParam(req, "releaseMBID"))
		if releaseMBID == "" || !releaseMBIDRegex.MatchString(releaseMBID) {
			n.writeDefaultCover(w)
			return
		}

		filePath := n.coverFilePath(releaseMBID)
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
		if _, err := os.Stat(filePath); err == nil {
			http.ServeFile(w, req, filePath)
			return
		}

		n.writeDefaultCover(w)
	})
}

func (n *Router) writeDefaultCover(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(defaultCoverPlaceholder)
}

func (n *Router) withSongArtwork(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()

		handler(rec, r)

		resp := rec.Result()
		defer resp.Body.Close()

		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		if resp.StatusCode >= http.StatusMultipleChoices {
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error(r.Context(), "Failed to read song response", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(body) == 0 {
			w.WriteHeader(resp.StatusCode)
			return
		}

		augmented, err := n.enrichSongArtwork(r, body)
		if err != nil {
			log.Error(r.Context(), "Failed to augment song response", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(augmented)))
		w.WriteHeader(resp.StatusCode)
		w.Write(augmented)
	}
}

func (n *Router) enrichSongArtwork(r *http.Request, body []byte) ([]byte, error) {
	var list model.MediaFiles
	if err := json.Unmarshal(body, &list); err == nil {
		for i := range list {
			n.populateSongArtwork(r, &list[i])
		}
		return json.Marshal(list)
	}

	var single model.MediaFile
	if err := json.Unmarshal(body, &single); err == nil {
		n.populateSongArtwork(r, &single)
		return json.Marshal(single)
	}

	return body, nil
}

func (n *Router) populateSongArtwork(r *http.Request, song *model.MediaFile) {
	if song == nil {
		return
	}

	if strings.TrimSpace(song.ArtworkID) != "" || strings.TrimSpace(song.ArtworkURL) != "" || song.HasCoverArt {
		coverArtID := strings.TrimSpace(song.ArtworkID)
		if coverArtID == "" {
			coverArtID = song.CoverArtID().String()
		}
		song.ArtworkID = coverArtID
		coverArtURL := strings.TrimSpace(song.ArtworkURL)
		if coverArtURL == "" && coverArtID != "" {
			coverArtURL = publicurl.ImageURL(r, song.CoverArtID(), coverArtDefaultSize)
		}
		if coverArtURL != "" {
			if strings.Contains(coverArtURL, "?") {
				coverArtURL += "&square=true"
			} else {
				coverArtURL += "?square=true"
			}
		}
		song.ArtworkURL = coverArtURL
		song.CoverArtURL = coverArtURL
		return
	}

	if releaseMBID := strings.TrimSpace(song.MbzReleaseID); releaseMBID != "" && releaseMBIDRegex.MatchString(releaseMBID) {
		song.CoverArtURL = "/api/cover/" + releaseMBID
		song.ArtworkURL = song.CoverArtURL
		return
	}

	song.CoverArtURL = ""
}

func (n *Router) addSongRoute(r chi.Router) {
	constructor := func(ctx context.Context) rest.Repository {
		return n.ds.Resource(ctx, model.MediaFile{})
	}

	r.Route("/song", func(r chi.Router) {
		r.Get("/", n.withSongArtwork(rest.GetAll(constructor)))

		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", n.withSongArtwork(rest.Get(constructor)))
		})
	})
}

func (api *Router) addPlaylistRoute(r chi.Router) {
	constructor := func(ctx context.Context) rest.Repository {
		return api.playlists.NewRepository(ctx)
	}

	r.Route("/playlist", func(r chi.Router) {
		r.Get("/", rest.GetAll(constructor))
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Content-type") == "application/json" {
				createPlaylist(api.ds, api.playlists)(w, r)
				return
			}
			createPlaylistFromM3U(api.playlists)(w, r)
		})

		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", rest.Get(constructor))
			r.Put("/", rest.Put(constructor))
			r.Delete("/", rest.Delete(constructor))

			r.Post("/publish", publishPlaylist(api.playlists))

			r.Patch("/folder", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				type reqBody struct {
					FolderID *string `json:"folderId"`
				}
				var body reqBody
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if body.FolderID != nil && *body.FolderID == "" {
					http.Error(w, "folderId cannot be empty; use null for unassigned", http.StatusBadRequest)
					return
				}
				if err := api.playlists.SetFolder(r.Context(), id, body.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})
			r.Post("/image", uploadPlaylistImage(api.playlists))
			r.Delete("/image", deletePlaylistImage(api.playlists))
		})
	})
}

func (n *Router) addPlaylistFolderRoute(r chi.Router) {
	constructor := func(ctx context.Context) rest.Repository {
		return n.ds.Resource(ctx, model.PlaylistFolder{})
	}

	r.Route("/folder", func(r chi.Router) {
		// Combined list (folders + playlists) with paging
		r.Get("/", ListFoldersAndPlaylists(n.ds))
		r.Post("/", rest.Post(constructor))

		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", rest.Get(constructor))
			r.Put("/", rest.Put(constructor))
			r.Delete("/", rest.Delete(constructor))

			r.Patch("/parent", MoveFolder(n.ds))
		})

		r.Patch("/move", BulkMove(n.ds, n.playlists))
	})
}

func (api *Router) addPlaylistTrackRoute(r chi.Router) {
	r.Route("/playlist/{playlistId}/tracks", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			getPlaylist(api.playlists)(w, r)
		})
		r.With(server.URLParamsMiddleware).Route("/", func(r chi.Router) {
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				deleteFromPlaylist(api.playlists)(w, r)
			})
			r.Post("/", func(w http.ResponseWriter, r *http.Request) {
				addToPlaylist(api.playlists)(w, r)
			})
		})
		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				getPlaylistTrack(api.playlists)(w, r)
			})
			r.Put("/", func(w http.ResponseWriter, r *http.Request) {
				reorderItem(api.playlists)(w, r)
			})
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				deleteFromPlaylist(api.playlists)(w, r)
			})
		})
	})
}

func (n *Router) addDiscoveryRoute(r chi.Router) {
	constructor := func(ctx context.Context) rest.Repository {
		return n.ds.Resource(ctx, model.Discovery{})
	}

	r.Route("/discovery", func(r chi.Router) {
		r.Get("/", rest.GetAll(constructor))
		r.Post("/sync", syncDiscoveries(n.ds))
		r.With(server.URLParamsMiddleware).Get("/{discoveryId}", getDiscovery(n.ds))
		r.Route("/{discoveryId}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", getDiscovery(n.ds))
			r.Post("/publish", publishDiscovery(n.ds))
			r.Get("/tracks", getDiscoveryTracks(n.ds))
			r.Delete("/tracks", deleteDiscoveryTracks(n.ds))
		})
	})
}

func (api *Router) addSongPlaylistsRoute(r chi.Router) {
	r.With(server.URLParamsMiddleware).Get("/song/{id}/playlists", func(w http.ResponseWriter, r *http.Request) {
		getSongPlaylists(api.playlists)(w, r)
	})
}

func (n *Router) addSongDiscoveriesRoute(r chi.Router) {
	r.With(server.URLParamsMiddleware).Get("/song/{id}/discoveries", func(w http.ResponseWriter, r *http.Request) {
		getSongDiscoveries(n.ds)(w, r)
	})
}

func (n *Router) addSongCommentRoute(r chi.Router) {
	r.With(adminOnlyMiddleware).Put("/song/comment", updateSongComments(n.ds))
}

func (api *Router) addQueueRoute(r chi.Router) {
	r.Route("/queue", func(r chi.Router) {
		r.Get("/", getQueue(api.ds))
		r.Post("/", saveQueue(api.ds))
		r.Put("/", updateQueue(api.ds))
		r.Delete("/", clearQueue(api.ds))
	})
}

func (api *Router) addMissingFilesRoute(r chi.Router) {
	r.Route("/missing", func(r chi.Router) {
		api.RX(r, "/", newMissingRepository(api.ds), false)
		r.Delete("/", deleteMissingFiles(api.maintenance))
	})
}

func writeDeleteManyResponse(w http.ResponseWriter, r *http.Request, ids []string) {
	var resp []byte
	var err error
	if len(ids) == 1 {
		resp = []byte(`{"id":"` + html.EscapeString(ids[0]) + `"}`)
	} else {
		resp, err = json.Marshal(&struct {
			Ids []string `json:"ids"`
		}{Ids: ids})
		if err != nil {
			log.Error(r.Context(), "Error marshaling response", "ids", ids, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	_, err = w.Write(resp) //nolint:gosec
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (api *Router) addInspectRoute(r chi.Router) {
	if conf.Server.Inspect.Enabled {
		r.Group(func(r chi.Router) {
			if conf.Server.Inspect.MaxRequests > 0 {
				log.Debug("Throttling inspect", "maxRequests", conf.Server.Inspect.MaxRequests,
					"backlogLimit", conf.Server.Inspect.BacklogLimit, "backlogTimeout",
					conf.Server.Inspect.BacklogTimeout)
				r.Use(middleware.ThrottleBacklog(conf.Server.Inspect.MaxRequests, conf.Server.Inspect.BacklogLimit, time.Duration(conf.Server.Inspect.BacklogTimeout)))
			}
			r.Get("/inspect", inspect(api.ds))
		})
	}
}

func (api *Router) addConfigRoute(r chi.Router) {
	if conf.Server.DevUIShowConfig {
		r.Get("/config/*", getConfig)
	}
}

func (n *Router) addSyncRoute(r chi.Router) {
	r.Get("/sync", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get("https://push.jareddietch.com")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.Copy(w, resp.Body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func (api *Router) addKeepAliveRoute(r chi.Router) {
	r.Get("/keepalive/*", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":"ok", "id":"keepalive"}`))
	})
}

func (api *Router) addInsightsRoute(r chi.Router) {
	r.Get("/insights/*", func(w http.ResponseWriter, r *http.Request) {
		last, success := api.insights.LastRun(r.Context())
		if conf.Server.EnableInsightsCollector {
			_, _ = w.Write([]byte(`{"id":"insights_status", "lastRun":"` + last.Format("2006-01-02 15:04:05") + `", "success":` + strconv.FormatBool(success) + `}`)) //nolint:gosec
		} else {
			_, _ = w.Write([]byte(`{"id":"insights_status", "lastRun":"disabled", "success":false}`))
		}
	})
}

// Middleware to ensure only admin users can access endpoints
func adminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := request.UserFrom(r.Context())
		if !ok || !user.IsAdmin {
			http.Error(w, "Access denied: admin privileges required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
