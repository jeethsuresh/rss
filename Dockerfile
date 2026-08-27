FROM golang:1.24.2-alpine AS build

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rss-server ./cmd/server

FROM oven/bun:1.4.0-alpine AS web

WORKDIR /src
COPY package.json bun.lock ./
COPY apps/desktop/package.json apps/desktop/package.json
COPY packages/shared/package.json packages/shared/package.json
# Install only renderer runtime dependencies, keeping Electron and
# electron-builder out of the image stage.
RUN bun install --production --frozen-lockfile --ignore-scripts --filter @rss-reader/desktop
COPY docker/web-package.json /tools/package.json
RUN cd /tools && bun install --production --ignore-scripts \
    && mkdir -p /src/node_modules/@vitejs \
    && ln -s /tools/node_modules/vite /src/node_modules/vite \
    && ln -s /tools/node_modules/@vitejs/plugin-react /src/node_modules/@vitejs/plugin-react
COPY apps/desktop/ apps/desktop/
COPY packages/shared/ packages/shared/
RUN cd apps/desktop && /tools/node_modules/.bin/vite build

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S rss \
    && adduser -S -G rss rss \
    && mkdir -p /data \
    && chown rss:rss /data

COPY --from=build /out/rss-server /usr/local/bin/rss-server
COPY --from=web /src/apps/desktop/dist-renderer /app/web

ENV RSS_SERVER_WEB_DIR=/app/web

USER rss
VOLUME ["/data"]
EXPOSE 8787

ENTRYPOINT ["/usr/local/bin/rss-server"]
