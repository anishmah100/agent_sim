FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/agent_sim_engine ./cmd/engine

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/agent_sim_engine /usr/local/bin/agent_sim_engine
# Worlds + art are mounted at runtime so we don't bake them in.
EXPOSE 8080
ENV ADDR=":8080"
ENV WORLD="/worlds/dev_test.json"
ENV SCENARIO="fantasy_town"
CMD ["sh", "-c", "agent_sim_engine -addr $ADDR -world $WORLD -scenario $SCENARIO"]
