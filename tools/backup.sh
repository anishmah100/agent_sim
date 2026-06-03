#!/usr/bin/env bash
# Postgres backup for the agent_sim engine.
#
# Usage:
#   tools/backup.sh                    # write to ./backups/agent_sim_YYYY-MM-DD.sql.gz
#   tools/backup.sh --rotate           # also drop event_log rows older than 7 days
#   tools/backup.sh --s3 s3://bucket   # upload after creation
#
# Assumes PGHOST/PGUSER/PGDATABASE env vars or a libpq URI in PGURL.

set -euo pipefail

cd "$(dirname "$0")/.."

OUT_DIR="${BACKUP_DIR:-./backups}"
mkdir -p "$OUT_DIR"

STAMP="$(date +%Y-%m-%d-%H%M%S)"
DB="${PGDATABASE:-agent_sim}"
OUT="${OUT_DIR}/${DB}_${STAMP}.sql.gz"

ROTATE=false
S3=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --rotate) ROTATE=true; shift ;;
        --s3) S3="$2"; shift 2 ;;
        *) echo "unknown arg: $1" >&2; exit 1 ;;
    esac
done

echo "==> pg_dump → $OUT"
pg_dump --no-owner --no-privileges "$DB" | gzip > "$OUT"
echo "    wrote $(du -h "$OUT" | cut -f1)"

if $ROTATE; then
    echo "==> trimming event_log > 7 days"
    psql "$DB" -c "DELETE FROM event_log WHERE created_at < NOW() - INTERVAL '7 days';"
fi

if [[ -n "$S3" ]]; then
    echo "==> uploading to $S3"
    aws s3 cp "$OUT" "$S3/"
fi

echo "==> pruning local backups > 30 days"
find "$OUT_DIR" -name "${DB}_*.sql.gz" -mtime +30 -delete

echo "done."
