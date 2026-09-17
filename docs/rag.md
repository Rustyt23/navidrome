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
export ND_RAGMINSCORE=0.5
export ND_RAGQDRANTTIMEOUT=15s
export ND_RAGEMBEDDINGTIMEOUT=60s
export ND_RAGRETRYMAX=2
export ND_RAGRETRYBACKOFF=250ms
# Optional local Ollama-compatible embedding endpoint:
export ND_RAGEMBEDDINGURL=http://localhost:11434/api/embed
export ND_RAGEMBEDDINGMODEL=embeddinggemma
# Require both local embeddings and the local Gemma 3:4b chat provider:
export ND_RAGOFFLINE=true
```

`ND_RAGMINSCORE` removes weak cosine-similarity matches before they reach chat
or recommendation responses, allowing the no-match fallback to fire. Set it to
`0` to disable the cutoff for embedding models with a different score scale.
Transient embedding and Qdrant failures (`429`, `502`, `503`, and `504`) are
retried with exponential backoff. The timeout and retry settings above can be
tuned for larger libraries and slower local models.

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

Each collection contains a reserved metadata point with the RAG index schema
version, embedding-model tag, and vector dimensions. Search and indexing stop
with an actionable reindex error when those values drift. To migrate, submit an
explicit forced full sync; this recreates only the incompatible Qdrant
collection before rebuilding it:

```http
POST /api/ai/rag/index
Content-Type: application/json

{"limit":500,"force":true,"sync":true}
```

The AI page lets administrators choose between 1 and 500 songs per request
(default 50). The indexer generates embeddings and upserts them into Qdrant.
When `ND_RAGEMBEDDINGURL` is configured, lyrics and metadata are embedded by the
local Ollama-compatible model and are never sent to Gemini. Otherwise,
`GeminiAPIKey`/`ND_GEMINIAPIKEY` is required. `ND_RAGOFFLINE=true` additionally
rejects cloud chat providers for RAG, ensuring query planning, retrieved
context, and final answers use the local Gemma 3:4b provider. Existing Qdrant
points are skipped, and the indexer continues through the library in bounded
pages. It does not update Navidrome songs, playlists, lyrics, or metadata.

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

The RAG window sends its eight most recent user/assistant turns. Follow-up
questions are rewritten into standalone retrieval queries, while the original
conversation is included in the final answer prompt. Recommendation results are
selected from an expanded candidate pool with a hard per-artist diversity cap.
Natural-language constraints are extracted into the validated search-filter
schema by the selected chat model; invalid structured output safely falls back
to unfiltered semantic retrieval.

RAG chat also supports dedicated modes selected by the structured query plan:

- Lyric identification: “find the song that goes ‘midnight train’” searches
  lyric-bearing songs and returns a highlighted lyric snippet.
- Library analytics: “how many clean songs are from 2015?” and “which artists
  dominate my library?” use exact aggregation over the Navidrome library, not a
  retrieved sample.
- Cleanup: questions about duplicate rips, remixes, live versions, or radio
  edits use embedding proximity plus title/artist/duration heuristics.

The same read-only capabilities are available directly through
`POST /api/ai/rag/lyrics/search` and
`GET /api/ai/rag/duplicates?limit=1000&threshold=0.92`.

When Prometheus is enabled, RAG exports `rag_operation_duration_seconds`,
`rag_http_retries_total`, `rag_retrieval_result_count`, and
`rag_retrieval_top_score`. Retrieval completion logs contain latency, result
count, top score, and requested `topK`, but never the user's query text.

AI chat, explicit classification, and metadata fetching can use either Gemma
26B or the Ollama-compatible `Gemma 3:4b` provider. Configure the latter with
`ND_GEMMA4APIURL`; it defaults to
`http://34.172.168.194:8084/api/generate`. RAG indexing and query embeddings use
the local embedding endpoint when configured; Gemma 3:4b remains the local
text-generation model used for planning and answers.

The AI page also selects the runtime default Whisper model from `tiny`, `base`,
`small`, `medium`, `large-v3`, or `turbo`. `ND_WHISPERMODEL` sets the startup
default. Navidrome sends the selection as a multipart `model` field; the
configured Whisper server must support that field to change models. Fetched
text is saved as `./lyrics/<songID>.txt` (configurable with
`ND_WHISPERLYRICSFOLDER`) and is also kept in Navidrome's existing lyrics
record.
