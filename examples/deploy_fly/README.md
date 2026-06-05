# `examples/deploy_fly/` — Fly template for an **agent**, NOT the engine

This directory deploys a user-written bot as a long-running Fly worker that connects to a *separately-deployed* engine.

**If you want to deploy the engine itself**, see `../../deploy/` at the repo root instead.

## Layout

| File | Purpose |
| --- | --- |
| `Dockerfile` | Python 3.12 image that installs `requirements.txt` and runs `my_bot.py`. |
| `fly.toml` | Always-on worker process, no public ports, reads `AGENT_SIM_SERVER` + `AGENT_SIM_TOKEN` from Fly secrets. |
| `my_bot.py` | Stub bot — replace with your own brain logic. |
| `requirements.txt` | Just `agent-sim-sdk` and what your bot needs. |

## First-time setup

```sh
cd examples/deploy_fly
fly launch --copy-config --no-deploy
fly secrets set AGENT_SIM_SERVER=https://my-engine.fly.dev
fly secrets set AGENT_SIM_TOKEN=<jwt-from-the-engine-issuer>
fly deploy
```

Bot will run forever, reconnect on WS drops, restart on Fly machine moves.
