# Build from the REPO ROOT:
#   docker build -f engine/Dockerfile -t agent_sim_engine .
# or via Fly (configured in deploy/fly.toml at the repo root).
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY engine/go.mod engine/go.sum ./
RUN go mod download
COPY engine .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/agent_sim_engine ./cmd/engine

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/agent_sim_engine /usr/local/bin/agent_sim_engine

# Worlds (with their art) baked into the image so the engine boots
# standalone. Each world is a self-contained bundle under
# /worlds/<name>/ — art lives at /worlds/<name>/art/.
# Override BUNDLE env to switch maps.
COPY worlds /worlds

EXPOSE 8080

# Defaults — Fly env overrides these in fly.toml.
ENV ADDR="0.0.0.0:8080"
ENV BUNDLE="/worlds/eldoria"
ENV SNAP_DIR="/data/snapshots"
ENV SNAP_EVERY="60s"
ENV EVENT_LOG="/data/events.jsonl"
ENV CORS_ALLOW=""
ENV JWT_SECRET=""
ENV REGISTER_RATE="1"
ENV REGISTER_BURST="5"

CMD ["sh", "-c", "mkdir -p /data/snapshots && agent_sim_engine \
  -addr $ADDR \
  -bundle $BUNDLE \
  -snapshot-dir $SNAP_DIR \
  -snapshot-every $SNAP_EVERY \
  -event-log $EVENT_LOG \
  -cors-allow \"$CORS_ALLOW\" \
  -jwt-secret \"$JWT_SECRET\" \
  -register-rate $REGISTER_RATE \
  -register-burst $REGISTER_BURST"]
