# Compiles a real, minified production Tailwind CSS bundle via the
# standalone CLI — no Node/npm anywhere in this toolchain. Only needs
# cmd/ui's own files: Tailwind v4 scans them as plain text for candidate
# class names, no JS/HTML parsing involved.
FROM alpine:3.20 AS assets
# TARGETARCH is a BuildKit-populated build arg (amd64/arm64) — picking the
# matching binary natively avoids needing QEMU/Rosetta emulation, which
# isn't reliably set up in every environment (hit exactly that failure
# building on Apple Silicon with a hardcoded x64 binary).
ARG TARGETARCH
# libstdc++: the standalone CLI (Bun-compiled) dynamically links it even
# in its "musl" build — bare alpine only ships libc, not libstdc++.
RUN apk add --no-cache curl libstdc++
RUN case "$TARGETARCH" in \
        amd64) tw_arch=x64 ;; \
        arm64) tw_arch=arm64 ;; \
        *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && curl -sL -o /usr/local/bin/tailwindcss \
        "https://github.com/tailwindlabs/tailwindcss/releases/download/v4.3.3/tailwindcss-linux-${tw_arch}-musl" \
    && chmod +x /usr/local/bin/tailwindcss
WORKDIR /src
COPY cmd/ui/static/input.css cmd/ui/static/input.css
COPY cmd/ui/templates.go cmd/ui/templates.go
RUN tailwindcss -i cmd/ui/static/input.css -o /out/style.css --minify

# htmx and Chart.js were previously loaded from unpkg.com/cdn.jsdelivr.net
# at runtime — a slow or unreachable CDN on any given page load silently
# breaks tab-switching (htmx) and leaves blank chart canvases (Chart.js),
# which reads as "the site is broken" with no error the user can see.
# Downloading pinned copies at build time removes that runtime dependency
# entirely, same as the Tailwind CLI binary above.
RUN curl -sL -o /out/htmx.min.js "https://unpkg.com/htmx.org@2.0.3/dist/htmx.min.js" \
    && curl -sL -o /out/chart.min.js "https://cdn.jsdelivr.net/npm/chart.js@4.5.1/dist/chart.umd.min.js"

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/ui ./cmd/ui

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ui /ui
COPY --from=assets /out/style.css /static/style.css
COPY --from=assets /out/htmx.min.js /static/htmx.min.js
COPY --from=assets /out/chart.min.js /static/chart.min.js
COPY cmd/ui/static/favicon.svg /static/favicon.svg
EXPOSE 8081
ENTRYPOINT ["/ui"]
