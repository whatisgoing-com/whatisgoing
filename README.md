# whatisgoing.com

News analytics platform: scrapes news articles, extracts named entities (PERSON/ORG/EVENT), tracks mention frequency and sentiment over time, and surfaces trending topics/persons/orgs and entity "reputation" trends.

## Services

- `cmd/core` — Go modular monolith: source scheduler, RSS/scraper fetcher, pipeline coordinator, internal JSON API.
- `cmd/ui` — Go + htmx BFF, renders the public dashboard from `core`'s JSON API.
- `services/ner-sentiment` — Python (FastAPI): article extraction (`trafilatura`), NER (spaCy), sentence-level sentiment (DistilBERT).

## Ingestion sources

`cmd/core` reads its source list from `configs/sources.yaml` (path overridable via `SOURCES_CONFIG_PATH`). Copy [`configs/sources.example.yaml`](./configs/sources.example.yaml) to get started — it's gitignored on purpose, since the real curated source list is a content decision, not something to fabricate in code. Each source is either `type: rss` (default — polled via its feed URL) or `type: html` (fallback for sources without a feed — article links are discovered from a listing page). Every outbound request, RSS or HTML, goes through a shared politeness layer: robots.txt is checked per domain, and both a minimum delay and a concurrency cap are enforced per domain.

If no config file is found, the service still starts and serves `/healthz` — it just won't fetch anything until one exists.

## Local development

```sh
docker compose up --build
```

- core: http://localhost:8080/healthz
- ui: http://localhost:8081
- ner-sentiment: http://localhost:8000/healthz
- Postgres: localhost:5432 (`whatisgoing`/`whatisgoing`)
- Meilisearch: http://localhost:7700

## Deployment

Images are built and pushed to a self-hosted Harbor registry by Drone CI (path-triggered per service). Drone commits the new image tag to [`whatisgoing-gitops`](https://github.com/whatisgoing-com/whatisgoing-gitops), which ArgoCD syncs to the k3s cluster.

## Tracking

Work is tracked on the [v1 project board](https://github.com/orgs/whatisgoing-com/projects/2).
