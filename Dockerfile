# syntax=docker/dockerfile:1
#
# ServerWatch Community Edition - self-contained image.
#
# One container serves the dashboard (Nginx), proxies /api to the Go backend,
# and runs the monitor scheduler. It expects a Postgres database reachable at
# DATABASE_URL. See deployments/docker/ for the compose file and docs.
#
# Build from the repository root:
#   docker build -t serverwatch:dev .
#
#   docker build --build-arg VERSION=0.1.0 --build-arg COMMIT=$(git rev-parse --short HEAD) .

# --- Stage 1: build the dashboard -------------------------------------------
FROM node:22-alpine AS web
RUN corepack enable
WORKDIR /web
# Copy manifests first so the dependency layer is cached across source edits.
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
ARG VERSION=dev
ENV VITE_APP_VERSION=${VERSION}
RUN pnpm build

# --- Stage 2: build the backend ---------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w \
        -X github.com/serverwatch/serverwatch/server/internal/version.Version=${VERSION} \
        -X github.com/serverwatch/serverwatch/server/internal/version.Commit=${COMMIT} \
        -X github.com/serverwatch/serverwatch/server/internal/version.Date=${DATE}" \
      -o /out/serverwatch ./cmd/serverwatch

# --- Stage 3: runtime -------------------------------------------------------
FROM nginx:1.27-alpine AS runtime
RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /out/serverwatch /usr/local/bin/serverwatch
COPY --from=web /web/dist /usr/share/nginx/html
COPY deployments/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY deployments/docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# The backend listens on loopback; Nginx is the only public listener.
ENV SERVERWATCH_HTTP_ADDR=127.0.0.1:8080

EXPOSE 80

# /health is served by the backend through Nginx and does not touch the DB, so
# it reflects whether both processes are up.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
