# Build context = REPO ROOT (works both with `docker compose` using `context: .`
# and the Nrapken builder, which always uses the repo root as context).
# Builds both binaries (api + worker) into one small static image.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 pulse
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker
USER pulse
# overridden per-service in docker-compose (api | worker)
CMD ["api"]
