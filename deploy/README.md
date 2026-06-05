# Deploy

Single-machine Fly.io deploy for the agent_sim engine.

## Files
- `fly.toml` — app config: persistent volume mount at `/data`, secrets-driven CORS allowlist + JWT secret, healthcheck on `/healthz`, `auto_stop_machines = off` so the world keeps ticking.
- `../engine/Dockerfile` — multi-stage build of the engine binary; bakes the `worlds/` and `art/processed/` directories into the image so the deployed container is standalone.

## First-time setup

```sh
# from the repo root
brew install flyctl                  # or: curl -L https://fly.io/install.sh | sh
fly auth signup                      # or: fly auth login
fly launch --copy-config --no-deploy --dockerfile engine/Dockerfile

# persistent storage for snapshots + event log
fly volumes create agentsim_data --size 1 --region iad

# rotate-friendly secrets
fly secrets set JWT_SECRET=$(openssl rand -hex 32)
fly secrets set CORS_ALLOW=https://your-frontend.example,https://another.example
```

## Deploy

```sh
fly deploy --config deploy/fly.toml
fly status                            # check that the machine is healthy
fly logs                              # tail engine logs
```

## Smoke check the deployed instance

```sh
APP=https://agent-sim-engine.fly.dev
curl -sS $APP/healthz                 # → ok
curl -sS $APP/api/v1/world/info | jq  # → engine metadata
curl -sS $APP/metrics | head          # → prometheus exposition
```

## Snapshot persistence

The engine writes snapshots to `/data/snapshots/latest.json` every `SNAP_EVERY` (default 60s) and on shutdown. Because `/data` is mounted from the Fly volume, snapshots survive deploys, restarts, and machine moves.

To verify after deploy:

```sh
fly ssh console -C "ls -la /data/snapshots/"
# expect: latest.json + a few timestamped JSON files
fly machine restart                    # force a restart
fly ssh console -C "head -1 /data/snapshots/latest.json"
# expect: the snapshot just written before the restart
```

## Auth check

Once `JWT_SECRET` is set, `/api/v1/agent/register` rejects unsigned requests:

```sh
# unsigned → 401
curl -sS -X POST $APP/api/v1/agent/register \
  -H 'Content-Type: application/json' \
  -d '{"persona_blob":{"name":"x"}}'
# → {"error":"auth required"}
```

For a friends-only launch (no real auth backend), use the
`tools/issue_jwt.py` CLI to mint per-friend tokens with the same
secret you set on Fly:

```sh
# Issue a 30-day token
JWT_SECRET="$(fly secrets list --json | jq -r '.[]|select(.Name=="JWT_SECRET").Value')" \
  python3 tools/issue_jwt.py --subject alice@example.com --ttl-days 30

# Use it
curl -sS -X POST $APP/api/v1/agent/register \
  -H 'Content-Type: application/json' \
  -d "{\"user_token\":\"$TOKEN\",\"persona_blob\":{\"name\":\"alice\"}}"
# → {"agent_id":"...","agent_secret":"...","ws_url":"...","entity_id":"..."}
```

When you outgrow per-friend tokens, swap in a real auth backend
(Auth.js / Supabase / Clerk) that issues the same HS256 JWTs.
