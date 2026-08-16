# VITE_CORE_API_URL is baked into the static bundle at build time — Vite
# env vars are compile-time, not readable at container runtime, since this
# is a plain client-rendered SPA with no server of its own. Defaults to the
# docker-compose dev setup, where the browser reaches core via its
# host-published port, not the docker-network-internal "core" hostname.
ARG VITE_CORE_API_URL=http://localhost:8080

FROM node:22-alpine AS build
WORKDIR /src
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
ARG VITE_CORE_API_URL
ENV VITE_CORE_API_URL=$VITE_CORE_API_URL
RUN npm run build

FROM nginx:1.27-alpine
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
