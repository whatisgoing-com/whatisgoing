# whatisgoing.com

News analytics platform: scrapes news articles, extracts named entities (PERSON/ORG/TOPIC), tracks mention frequency and sentiment over time, and surfaces trending topics/persons/orgs and entity "reputation" trends.

## Services

- `cmd/core` — Go modular monolith: source scheduler, RSS/scraper fetcher, pipeline coordinator, internal JSON API.
- `web` — React SPA dashboard, calls `core`'s JSON API directly from the browser.
- `cmd/rollup` — computes windowed entity-mention rollups (hot topics, reputation trend), then exits; run on a schedule rather than as a long-lived service.
- `cmd/entity-resolver` — canonicalizes entity name variants ("Trump" / "Donald Trump" / "Donald J. Trump" → one entity) against Wikidata, then exits; same run-on-a-schedule shape as `cmd/rollup`.
- `services/ner-sentiment` — Python (FastAPI): article extraction (`trafilatura`), NER (GLiNER), sentence-level sentiment (DistilBERT). Runs natively on the host in local dev, not in Docker — see [its README](./services/ner-sentiment/README.md).

## Ingestion sources

`cmd/core` reads its source list from `configs/sources.yaml` (path overridable via `SOURCES_CONFIG_PATH`). Copy [`configs/sources.example.yaml`](./configs/sources.example.yaml) to get started — it's gitignored on purpose, since the real curated source list is a content decision, not something to fabricate in code. Each source is either `type: rss` (default — polled via its feed URL) or `type: html` (fallback for sources without a feed — article links are discovered from a listing page). Every outbound request, RSS or HTML, goes through a shared politeness layer: robots.txt is checked per domain, and both a minimum delay and a concurrency cap are enforced per domain.

If no config file is found, the service still starts and serves `/healthz` — it just won't fetch anything until one exists. Under docker-compose, `./configs` is mounted read-only into the container, so `docker compose up` picks up whatever's at `configs/sources.yaml` on the host.

## Storage

Postgres is the system of record (schema in `internal/core/store/postgres/migrations`, applied automatically at startup). Every save dual-writes to Meilisearch for full-text search; Postgres is authoritative — if the Meilisearch write fails, the article is still saved with `indexed_at` left `NULL`, and a background reconciliation job (hourly) retries any article stuck in that state. Persistent article dedup is enforced by a unique constraint on `dedup_key`, not application logic.

## NER + sentiment

`services/ner-sentiment` exposes a single `POST /extract` endpoint: `{"html": "...", "url": "..."}` (raw HTML or RSS item content; `url` is only ever used as an extraction hint, never fetched) returns extracted plain text plus a flat list of entity mentions, each with a whitelisted type (`PERSON`/`ORG`/`TOPIC`), character offsets into the returned text, and a sentiment score in `[-1, 1]`.

- **Extraction**: `trafilatura` for full article pages; falls back to a plain tag-strip for HTML fragments too short for trafilatura to find boilerplate to remove from (common for RSS item content).
- **Entities**: [GLiNER](https://huggingface.co/urchade/gliner_medium-v2.1), a zero-shot NER model — extracts `PERSON`/`ORG`/`TOPIC` directly via labeled prompts. Replaced spaCy's `en_core_web_sm` NER (issue #37) after real-data testing showed it mistagging obvious cases; spaCy stays in the pipeline for sentence segmentation and a GPE/LOC safety net on topic candidates, not entity recognition. Full writeup in [the service's README](./services/ner-sentiment/README.md#why-gliner-issue-37).
- **Sentiment**: `distilbert-base-uncased-finetuned-sst-2-english`, computed per sentence (via spaCy sentence boundaries in the same text) and attributed to every entity mentioned in that sentence — not whole-article sentiment.
- **CPU-only**: no GPU, no LLM; model weights are baked into the Docker image at build time (production) or pre-downloaded once by `run.sh` (local dev) so the service needs no network access at runtime.
- **Runs natively in local dev, not in Docker** — GLiNER + torch make the image slow to rebuild on every change. See [services/ner-sentiment/README.md](./services/ner-sentiment/README.md) for `run.sh` usage; `docker-compose.yml`'s `core` service reaches it via `host.docker.internal:8000`.

## Batch aggregation (hot topics, reputation trend)

`cmd/rollup` recomputes `entity_rollups` — one row per (entity, window, window start) with a mention count and averaged sentiment — for `day`/`week`/`month`/`year` windows, joining `mentions` and `articles` and grouping by `date_trunc`. It always recomputes every window from scratch rather than updating incrementally; at v1's ~1,000 articles/day volume that's cheap and stays correct-by-construction. It's meant to run on a schedule (a k8s CronJob in production) and exit, not as a long-lived process — running it twice in a row is safe (`ON CONFLICT ... DO UPDATE`, verified idempotent).

This powers two query shapes, exposed by `core`'s JSON API (below) and implemented as `RollupStore` methods: `TopEntities` (ranked mention frequency for a window — "hot topics/persons/orgs") and `ReputationTrend` (an entity's sentiment over time within a window granularity).

Run it locally against the compose stack: `docker compose run --build --rm rollup` (or `make run-rollup`) — the `--build` matters: `rollup` is excluded from `docker compose up --build` (it's a `profiles: [tools]` service, only ever run on demand), so without it you'll silently run whatever image was last built, however stale.

## Entity resolution (canonicalizing name variants)

NER extracts entity identity from raw text, so different articles referring to the same real-world entity by different surface forms ("Trump", "Donald Trump", "Donald J. Trump") each become their own `entities` row by default — splitting mention counts and sentiment across several rows instead of one (issue #26).

Two independent fixes:

- **At ingestion**: `internal/core/entityname.Normalize` strips a trailing possessive marker ("Donald Trump's" → "Donald Trump") before an extracted name becomes entity identity — cheap, no external dependency, catches that one class of noise outright.
- **`cmd/entity-resolver`**: for each entity without a `wikidata_id` yet, searches Wikidata for a matching item and merges any entities that resolve to the same QID. Deliberately only trusts Wikidata's *top* search result, and only if it's not a generic/meta entry (a small denylist — "family name", "disambiguation page", etc.) — tried scanning further down the result list for a better candidate and it's actively unsafe, not just noisier: searching a bare "Trump" surfaces "Trump (family name)" as its top hit, and the next "specific" result past that was an unrelated open star cluster matched via an alias, confirmed against the live API. When Wikidata gives nothing confident (typically a bare surname), it falls back to matching against already-resolved entities of the same type by name-token containment (e.g. "Trump" ⊆ "Donald Trump" once that's resolved) — only when that match is unambiguous; a genuinely ambiguous short name is left unresolved rather than guessed wrong.

Merging migrates `mentions` (averaging sentiment for any article both already had a row for) onto the surviving entity, then deletes the other — `entity_cooccurrence` and `entity_rollups` rows for it cascade-delete via their foreign keys, and the next rollup run recomputes rollups for the merged entity from scratch regardless.

Whichever entity becomes canonical for a QID also gets a short descriptive paragraph fetched from Wikipedia's public REST summary API (`internal/core/wikipedia`, looked up by Wikidata's canonical label — the two usually share a title for well-known entities), stored in `entities.description` and shown on the entity detail page. A failed or missing Wikipedia page doesn't block resolving the entity itself — it's still correctly deduplicated either way, just without a description.

Not solved here (accepted limitations, not gaps in disguise): no type-aware disambiguation beyond the denylist + token heuristic, so genuinely ambiguous names correctly stay unresolved rather than risk a wrong merge; a mention NER mistyped as the wrong entity type (e.g. "Trump" tagged `ORG` in some mentions) stays separate from the correctly-typed entity rather than being fixed here — that's a different, pre-existing bug. Verified end-to-end against real ingested data: `Donald Trump`/`Trump`/`Donald J. Trump` (280 real entities, several genuine duplicate clusters) collapsed into one `Donald Trump` entity carrying a real Wikidata QID, while the ORG-mistyped `Trump` correctly stayed separate and unresolved.

Run it locally: `docker compose run --build --rm entity-resolver` (or `make run-entity-resolver`).

## Core API + UI/BFF

`core`'s internal JSON API (`internal/core/api`), consumed only by `cmd/ui`:

- `GET /api/trending?window=day&limit=20` — most-mentioned entities in the current window (`day`/`week`/`month`/`year`).
- `GET /api/trend/overall?window=day&limit=30` — aggregate mentions + average sentiment per window_start across every entity, for the home dashboard's time-series chart.
- `GET /api/entities?q=...&limit=10` — find entities by a case-insensitive name substring match, for the dashboard's entity search bar.
- `GET /api/entities/{id}?window=day` — an entity's detail + reputation trend at that window granularity (each point includes positive/neutral/negative mention counts), plus a short Wikipedia description if the entity has been resolved (see "Entity resolution" below; `""` if not yet resolved). 404s if the entity doesn't exist or hasn't been through a rollup yet.
- `GET /api/search?q=...&limit=20` — full-text article search via Meilisearch.
- `GET /api/sentiment?window=day` — positive/neutral/negative mention counts summed across every entity for the window's current bucket, for the home dashboard's sentiment pie chart. A real total (backed by `positive_count`/`neutral_count`/`negative_count` columns on `entity_rollups`, populated by the rollup job's bucketing of each mention's sentiment score), not an approximation from only the top-N trending entities.
- `GET /api/entities/{id}/sources` — one entity's mention count + average sentiment per source, across all time — which outlets cover it, and how differently they cover it.
- `GET /api/articles/recent?limit=20&entity_id=...` — the most recently published articles (headline, source, link, publish time), optionally filtered to one entity via `entity_id`.

`cmd/ui` is a Go + htmx BFF (`internal/ui/coreclient` is its HTTP client for the API above) that server-renders three pages: trending, entity detail, and article search. No auth/admin UI in v1 — read-only public dashboard.

The trending (home) page: a sentiment overview as 4 stat tiles (real positive/neutral/negative mention counts + the window's real average sentiment, both from `/api/sentiment` and `/api/trend/overall` rather than an approximation), a bar chart of top entities by mention count, a time-series line chart of aggregate mentions/sentiment, a pie chart of the sentiment breakdown, and a recent-articles list (headlines, linking out) — charts via Chart.js (CDN, a rendering library rather than a SPA framework, so it doesn't conflict with the "Go + htmx, no JS framework" decision). Switching the day/week/month/year tab htmx-swaps the entire panel together via `hx-target`, so nothing goes stale relative to the others; Chart.js re-initializes because htmx evaluates `<script>` tags in swapped content. A debounced entity search bar (`hx-trigger="keyup changed delay:300ms"`) finds an entity by name and links straight to its detail page: a mentions/sentiment trend line chart, its own sentiment breakdown pie chart, a by-source breakdown (mention count + avg sentiment per source, with a proportional bar), and a recent-articles list scoped to that entity.

Styling is Tailwind CSS, compiled to a static file by the standalone CLI at Docker build time (`cmd/ui/Dockerfile`) — no Node/npm in the toolchain, still just server-rendered `html/template`. `cmd/ui/static/input.css` is the only committed input; the compiled `style.css` is a build artifact, never committed, served from `/static/style.css` (read from disk at runtime, not `go:embed`, so `go build`/`go test` never depend on it existing). The layout is responsive via Tailwind's `sm:`/`lg:` breakpoints rather than a hand-written media query.

The header logo (issue #28) is the brand mark: "going" is a solid block that always inverts against its background, no separate icon — set in Bricolage Grotesque (weight 800 only, embedded as a base64 `@font-face` in `input.css`, deliberately not used anywhere else — body copy, tables, and charts stay the default Tailwind sans stack). The favicon (`cmd/ui/static/favicon.svg`) is a placeholder monogram in the same ink/paper values, since no dedicated icon-only mark exists yet.

## Local development

`ner-sentiment` runs natively, not in docker-compose (see above) — start it first, in its own terminal:

```sh
services/ner-sentiment/run.sh
```

Then everything else:

```sh
docker compose up --build
```

- core: http://localhost:8080/healthz
- web: http://localhost:8081
- ner-sentiment: http://localhost:8000/healthz
- Postgres: localhost:5432 (`whatisgoing`/`whatisgoing`)
- Meilisearch: http://localhost:7700

`make help` lists the day-to-day commands (`make build`, `make test-core`/`test-ui`/`test-rollup` — each mirrors one of `.drone.yml`'s Go pipelines exactly, so a local failure means CI will fail the same way — `make test-py`, `make up`/`down`, etc).

### Getting real data locally

`cmd/core`'s fetch scheduler only runs every 15 minutes, which is a long wait for a fresh dev environment. `cmd/devseed` runs one real ingestion pass on demand — real RSS feeds, real NER/sentiment, purely additive (dedups on `dedup_key`, so re-running it never erases or duplicates anything already there):

```sh
make dev-bootstrap   # fresh start: stack up + seed + rollup + entity-resolver, one command
```

or piece by piece once the stack is already up: `make seed`, then `make run-rollup`, then `make run-entity-resolver` (entity resolution can run again any time — it's idempotent and only touches not-yet-resolved entities).

### Testing against Postgres/Meilisearch

Postgres/Meilisearch-backed tests are gated behind `TEST_DATABASE_URL` / `TEST_MEILISEARCH_URL` (+ `TEST_MEILISEARCH_KEY`) and skip themselves if unset — run `docker compose up -d postgres meilisearch` (or `make up-deps`) first, then e.g.:

```sh
TEST_DATABASE_URL="postgres://whatisgoing:whatisgoing@localhost:5432/whatisgoing?sslmode=disable" \
TEST_MEILISEARCH_URL="http://localhost:7700" \
TEST_MEILISEARCH_KEY="dev-master-key" \
go test ./...
```

(or `make test-db`, which sets those the same way)

**Careful what you point `TEST_DATABASE_URL` at**: `internal/core/store/postgres`'s tests `TRUNCATE` every table before each one runs, against whatever database that URL names. Pointing it at your main dev-stack Postgres (the example above does) wipes any seeded data as a side effect of running the test suite — re-run `make seed && make run-rollup && make run-entity-resolver` afterward, or use a separate/disposable Postgres for test runs if you want to keep dev data and test runs from stepping on each other.

## Deployment

Images are built and pushed to a self-hosted Harbor registry by Drone CI (path-triggered per service). Drone commits the new image tag to [`whatisgoing-gitops`](https://github.com/whatisgoing-com/whatisgoing-gitops), which ArgoCD syncs to the k3s cluster.

## Tracking

Work is tracked on the [v1 project board](https://github.com/orgs/whatisgoing-com/projects/2).
