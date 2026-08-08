# whatisgoing.com

News analytics platform: scrapes news articles, extracts named entities (PERSON/ORG/EVENT), tracks mention frequency and sentiment over time, and surfaces trending topics/persons/orgs and entity "reputation" trends.

## Services

- `cmd/core` — Go modular monolith: source scheduler, RSS/scraper fetcher, pipeline coordinator, internal JSON API.
- `cmd/ui` — Go + htmx BFF, renders the public dashboard from `core`'s JSON API.
- `services/ner-sentiment` — Python (FastAPI): article extraction (`trafilatura`), NER (spaCy), sentence-level sentiment (DistilBERT).

## Ingestion sources

`cmd/core` reads its source list from `configs/sources.yaml` (path overridable via `SOURCES_CONFIG_PATH`). Copy [`configs/sources.example.yaml`](./configs/sources.example.yaml) to get started — it's gitignored on purpose, since the real curated source list is a content decision, not something to fabricate in code. Each source is either `type: rss` (default — polled via its feed URL) or `type: html` (fallback for sources without a feed — article links are discovered from a listing page). Every outbound request, RSS or HTML, goes through a shared politeness layer: robots.txt is checked per domain, and both a minimum delay and a concurrency cap are enforced per domain.

If no config file is found, the service still starts and serves `/healthz` — it just won't fetch anything until one exists. Under docker-compose, `./configs` is mounted read-only into the container, so `docker compose up` picks up whatever's at `configs/sources.yaml` on the host.

## Storage

Postgres is the system of record (schema in `internal/core/store/postgres/migrations`, applied automatically at startup). Every save dual-writes to Meilisearch for full-text search; Postgres is authoritative — if the Meilisearch write fails, the article is still saved with `indexed_at` left `NULL`, and a background reconciliation job (hourly) retries any article stuck in that state. Persistent article dedup is enforced by a unique constraint on `dedup_key`, not application logic.

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
