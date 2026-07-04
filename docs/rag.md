# Local RAG development

RAG is enabled by default in this customized branch. Navidrome still starts if
Qdrant is unavailable, and the status endpoint reports the real offline state.

## Run Qdrant locally

From the Navidrome repository root, run:

```sh
docker run -p 6333:6333 -p 6334:6334 \
  -v "$(pwd)/qdrant_storage:/qdrant/storage" \
  qdrant/qdrant
```

Qdrant should then be available at:

```text
http://localhost:6333
```

The `qdrant_storage` directory keeps local Qdrant data between container runs.
It is separate from Navidrome's music library and SQLite database.

For a background container that restarts automatically whenever Docker is
running, use the included Compose service:

```sh
docker compose -f docker-compose.rag.yml up -d
```

The service uses `restart: unless-stopped`. Docker Desktop or another Docker
daemon must itself be running; the UI never reports a false online state.

## Enable the development RAG endpoints

Configure Navidrome with:

```sh
export ND_RAGVECTORURL=http://localhost:6333
export ND_RAGCOLLECTION=navidrome_songs
export ND_RAGTOPK=20
```

Set `ND_ENABLERAG=false` to opt out. `GET /api/ai/rag/status` checks Qdrant when
RAG is enabled. An unavailable
Qdrant instance is reported in the JSON status and does not stop Navidrome.
Administrators can also enable or disable RAG at runtime from the AI page. The
runtime choice lasts until Navidrome restarts; `ND_ENABLERAG` remains the
startup default.

Administrators can start a bounded indexing pass with:

```http
POST /api/ai/rag/index
Content-Type: application/json

{"limit":50,"force":false}
```

The AI page lets administrators choose between 1 and 500 songs per request
(default 50). The indexer generates Gemini embeddings and upserts them into
Qdrant. Existing Qdrant points are skipped, and the indexer continues through
the library in bounded pages to find new songs. It does not update Navidrome
songs, playlists, lyrics, comments, or metadata. `GeminiAPIKey`/
`ND_GEMINIAPIKEY` must be configured.

Administrators can use **View indexed songs** to inspect the configured Qdrant
collection and up to 100 stored song payloads in a table. The corresponding
read-only endpoint is `GET /api/ai/rag/documents?limit=100`; it never returns
embedding vectors or modifies the collection.

The protected `POST /api/ai/rag/search` endpoint accepts a query and `topK`
(maximum 50). The AI page has two independent floating windows: `RAG` sends
`useRag: true`, adds read-only library context, and returns songs in its
`sources` field; `AI Chat` sends `useRag: false` and talks to the selected model
without library retrieval. Requests that omit `useRag` retain the original RAG
behavior for compatibility.

AI chat, explicit classification, and metadata fetching can use either Gemma
26B or the Ollama-compatible `Gemma 3:4b` provider. Configure the latter with
`ND_GEMMA4APIURL`; it defaults to
`http://34.172.168.194:8084/api/generate`. RAG indexing and query embeddings
continue to use Gemini because Gemma 3:4b is a text-generation model, not the
vector embedder for this collection.

The AI page also selects the runtime default Whisper model from `tiny`, `base`,
`small`, `medium`, `large-v3`, or `turbo`. `ND_WHISPERMODEL` sets the startup
default. Navidrome sends the selection as a multipart `model` field; the
configured Whisper server must support that field to change models. Fetched
text is saved as `./lyrics/<songID>.txt` (configurable with
`ND_WHISPERLYRICSFOLDER`) and is also kept in Navidrome's existing lyrics
record.
