# whatisgoing.com

News analytics platform: scrapes news articles, extracts named entities (PERSON/ORG/EVENT), tracks mention frequency and sentiment over time, and surfaces trending topics/persons/orgs and entity "reputation" trends.

## Services

- `cmd/core` — Go modular monolith: source scheduler, RSS/scraper fetcher, pipeline coordinator, internal JSON API.
- `cmd/ui` — Go + htmx BFF, renders the public dashboard from `core`'s JSON API.
- `cmd/rollup` — computes windowed entity-mention rollups (hot topics, reputation trend), then exits; run on a schedule rather than as a long-lived service.
- `services/ner-sentiment` — Python (FastAPI): article extraction (`trafilatura`), NER (spaCy), sentence-level sentiment (DistilBERT).

## Ingestion sources

`cmd/core` reads its source list from `configs/sources.yaml` (path overridable via `SOURCES_CONFIG_PATH`). Copy [`configs/sources.example.yaml`](./configs/sources.example.yaml) to get started — it's gitignored on purpose, since the real curated source list is a content decision, not something to fabricate in code. Each source is either `type: rss` (default — polled via its feed URL) or `type: html` (fallback for sources without a feed — article links are discovered from a listing page). Every outbound request, RSS or HTML, goes through a shared politeness layer: robots.txt is checked per domain, and both a minimum delay and a concurrency cap are enforced per domain.

If no config file is found, the service still starts and serves `/healthz` — it just won't fetch anything until one exists. Under docker-compose, `./configs` is mounted read-only into the container, so `docker compose up` picks up whatever's at `configs/sources.yaml` on the host.

## Storage

Postgres is the system of record (schema in `internal/core/store/postgres/migrations`, applied automatically at startup). Every save dual-writes to Meilisearch for full-text search; Postgres is authoritative — if the Meilisearch write fails, the article is still saved with `indexed_at` left `NULL`, and a background reconciliation job (hourly) retries any article stuck in that state. Persistent article dedup is enforced by a unique constraint on `dedup_key`, not application logic.

## NER + sentiment

`services/ner-sentiment` exposes a single `POST /extract` endpoint: `{"html": "...", "url": "..."}` (raw HTML or RSS item content; `url` is only ever used as an extraction hint, never fetched) returns extracted plain text plus a flat list of entity mentions, each with a whitelisted type (`PERSON`/`ORG`/`EVENT`), character offsets into the returned text, and a sentiment score in `[-1, 1]`.

- **Extraction**: `trafilatura` for full article pages; falls back to a plain tag-strip for HTML fragments too short for trafilatura to find boilerplate to remove from (common for RSS item content).
- **NER**: spaCy `en_core_web_sm`, filtered to `PERSON`/`ORG`/`EVENT`.
- **Sentiment**: `distilbert-base-uncased-finetuned-sst-2-english`, computed per sentence (via spaCy sentence boundaries in the same text) and attributed to every entity mentioned in that sentence — not whole-article sentiment.
- **CPU-only**: no GPU, no LLM; model weights are baked into the image at build time so the container needs no network access at runtime.
- **Latency**: benchmarked locally at ~150-370ms/article end-to-end (cold first request ~370ms, steady-state ~150ms), comfortably inside the ~1,000 articles/day budget.

## Batch aggregation (hot topics, reputation trend)

`cmd/rollup` recomputes `entity_rollups` — one row per (entity, window, window start) with a mention count and averaged sentiment — for `day`/`week`/`month`/`year` windows, joining `mentions` and `articles` and grouping by `date_trunc`. It always recomputes every window from scratch rather than updating incrementally; at v1's ~1,000 articles/day volume that's cheap and stays correct-by-construction. It's meant to run on a schedule (a k8s CronJob in production) and exit, not as a long-lived process — running it twice in a row is safe (`ON CONFLICT ... DO UPDATE`, verified idempotent).

This powers two query shapes, exposed by `core`'s JSON API (below) and implemented as `RollupStore` methods: `TopEntities` (ranked mention frequency for a window — "hot topics/persons/orgs") and `ReputationTrend` (an entity's sentiment over time within a window granularity).

Run it locally against the compose stack: `docker compose run --rm rollup`.

## Core API + UI/BFF

`core`'s internal JSON API (`internal/core/api`), consumed only by `cmd/ui`:

- `GET /api/trending?window=day&limit=20` — most-mentioned entities in the current window (`day`/`week`/`month`/`year`).
- `GET /api/entities/{id}?window=day` — an entity's detail + reputation trend at that window granularity. 404s if the entity doesn't exist or hasn't been through a rollup yet.
- `GET /api/search?q=...&limit=20` — full-text article search via Meilisearch.

`cmd/ui` is a Go + htmx BFF (`internal/ui/coreclient` is its HTTP client for the API above) that server-renders three pages: trending (with day/week/month/year tabs, htmx-swapped without a full reload), entity detail (mention history + sentiment trend table), and search (htmx-submitted, partial results swap). No auth/admin UI in v1 — read-only public dashboard.

## Local development

```sh
docker compose up --build
```

- core: http://localhost:8080/healthz
- ui: http://localhost:8081
- ner-sentiment: http://localhost:8000/healthz
- Postgres: localhost:5432 (`whatisgoing`/`whatisgoing`)
- Meilisearch: http://localhost:7700

Postgres/Meilisearch-backed tests are gated behind `TEST_DATABASE_URL` / `TEST_MEILISEARCH_URL` (+ `TEST_MEILISEARCH_KEY`) and skip themselves if unset — run `docker compose up -d postgres meilisearch` first, then e.g.:

```sh
TEST_DATABASE_URL="postgres://whatisgoing:whatisgoing@localhost:5432/whatisgoing?sslmode=disable" \
TEST_MEILISEARCH_URL="http://localhost:7700" \
TEST_MEILISEARCH_KEY="dev-master-key" \
go test ./...
```

## Deployment

Images are built and pushed to a self-hosted Harbor registry by Drone CI (path-triggered per service). Drone commits the new image tag to [`whatisgoing-gitops`](https://github.com/whatisgoing-com/whatisgoing-gitops), which ArgoCD syncs to the k3s cluster.

## Tracking

Work is tracked on the [v1 project board](https://github.com/orgs/whatisgoing-com/projects/2).
