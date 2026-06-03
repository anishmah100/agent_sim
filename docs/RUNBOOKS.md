# Runbooks

Operational playbooks for the live engine. Keep terse — each entry
goes "symptom → check → fix".

## Engine crashed / process gone

**Symptom:** `/healthz` returns connection-refused or systemd shows
the unit failed.

**Check:**
1. `journalctl -u agent_sim_engine -n 200` — look for the panic stack.
2. Check disk space: `df -h /var/lib/agent_sim`.
3. Check open file descriptors: `lsof -p <pid_before>` (if known).

**Fix:**
- For a panic: capture the stack, restart the service, file a bug.
- For disk full: prune `historian` JSONL logs in
  `/var/lib/agent_sim/event-log/*.jsonl` older than 30 days.
- For fd exhaustion: increase systemd's `LimitNOFILE=` or reduce viewer
  cap.

## Tick rate degraded (< 55 Hz)

**Symptom:** `agentsim_tick` grows slowly in Prometheus; users report
laggy responses from agents.

**Check:**
1. `/metrics` → `agentsim_agents`, `agentsim_events_emitted_total`.
   If agents > 200 or events/sec > 1000, system is under load.
2. `top -p <engine_pid>` — is one core pinned?

**Fix:**
- Add a soak test entry to the harness for the new load profile.
- Profile with `go tool pprof http://engine:6060/debug/pprof/profile`.
- Short-term: cap NPC spawns in `scenarios/<world>/npcs.json`.

## Postgres full

**Symptom:** `/api/v1/world/info` 503s; engine logs `pq: ... no space`.

**Check:**
- `du -sh /var/lib/postgresql/data` and free space on the volume.
- `select pg_size_pretty(pg_total_relation_size('event_log'));` in
  psql — the event log table tends to dominate.

**Fix:**
- Run `tools/backup.sh --rotate` to write a `pg_dump` to S3 then
  drop event_log rows older than 7 days.
- Long-term: enable partitioning on `event_log.created_at` so old
  partitions can be dropped in O(1).

## NPC supervisor won't restart a process

**Symptom:** `agentsim_npc_restarts_total` stops growing for a spec
but its `running` field in `/metrics` is false.

**Check:**
1. `journalctl -u agent_sim_engine | grep 'npc\['` for the spec's
   stderr output.
2. Try running the command manually from the engine host with the
   same cwd + env.

**Fix:**
- If the Python script crashed during import, fix the dep.
- If the engine SDK rejected the registration, check `npcs.json` for
  a stale token.
- Set `auto_restart=false` to stop spamming restarts while debugging.

## Frontend can't reach engine

**Symptom:** Browser shows "engine offline (ok for solo render)".

**Check:**
1. CORS — `curl -i -H "Origin: https://app.example.com"
   https://engine.example.com/api/v1/world/info`. Should return
   `Access-Control-Allow-Origin: *`.
2. WebSocket — open dev tools network tab, look at the `/ws/viewer`
   request; failed handshake usually means a misconfigured reverse
   proxy.

**Fix:**
- nginx: set `proxy_http_version 1.1; proxy_set_header Upgrade
  $http_upgrade; proxy_set_header Connection "upgrade";` on the
  `/ws/` location.
- If running locally without DNS: confirm `VITE_ENGINE_URL` matches
  the engine's `-addr` flag value.

## Speech bubbles / audible events not appearing

**Symptom:** Agents are clearly calling `Speak()` but the frontend
shows no bubbles.

**Check:**
1. Connect to the viewer WS directly:
   `wscat -c ws://engine/ws/viewer | head -3` — the JSON envelopes
   should have an `audible` field when speech happened in the last 2s.
2. If audible array is empty, the speech action might be hitting a
   verb collision. Look at the historian's `TaskProposed` /
   `EnteredBuilding` events for context.

**Fix:**
- Confirm the engine binary was rebuilt after adding
  `RecentAudibleAll`. `strings engine_bin | grep RecentAudibleAll`
  should return ≥1 match.

## Backup / restore drill

**Quarterly drill — run on staging:**
1. `tools/backup.sh` — create a fresh dump.
2. Stop staging engine.
3. `dropdb agent_sim_staging && createdb agent_sim_staging`.
4. `gunzip < dump.sql.gz | psql agent_sim_staging`.
5. Start engine; verify `/healthz` + a viewer connect renders
   recognizable world.
6. Log the elapsed time in `docs/drills.log`.

## Deploy lockstep

When shipping a new release, in this order:

1. Tag the engine commit: `git tag vYY.MM.DD-NN && git push origin --tags`.
2. CI builds the engine binary + frontend bundle.
3. Run the soak harness against the candidate for 30 minutes.
4. If green: promote to staging, smoke-test with a manual viewer.
5. After 24 h with no errors, promote to prod.
6. Tail `/metrics` for the first hour — watch `agentsim_npc_restarts_total`
   for sudden growth.
