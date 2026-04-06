# ---- Build stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /usr/local/bin/vipe ./cmd/vipe

# ---- Runtime stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tini

COPY --from=builder /usr/local/bin/vipe /usr/local/bin/vipe

# VipeDB stores all data under VIPE_HOME.
# Mount a volume at /data/.vipe to persist models, index, and cache.
ENV VIPE_HOME=/data/.vipe
VOLUME ["/data/.vipe"]

RUN mkdir -p /data/.vipe

ENTRYPOINT ["tini", "--", "vipe"]
CMD ["--help"]

LABEL org.opencontainers.image.title="VipeDB" \
      org.opencontainers.image.description="AI-powered semantic search & real-time log analyzer" \
      org.opencontainers.image.version="0.3.0" \
      org.opencontainers.image.source="https://github.com/hashemzargari/vipedb"
