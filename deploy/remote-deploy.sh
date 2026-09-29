#!/usr/bin/env bash
set -euo pipefail
tag=${1:?release tag required}
[[ "$tag" =~ ^[a-zA-Z0-9-]+$ ]] || { echo 'Invalid release tag' >&2; exit 1; }
root=/opt/tksu-pl
release="$root/releases/$tag"
test ! -e "$release" || { echo 'Release already exists' >&2; exit 1; }
install -d -m 755 "$root" "$root/releases" "$release"
install -d -m 700 "$root/shared" "$root/backups"
install -d -m 700 -o 10001 -g 10001 "$root/shared/data"
if [ ! -e "$root/shared/.env" ]; then
    umask 077
    secret=$(openssl rand -hex 32)
    printf 'HOST=0.0.0.0\nPORT=8080\nDATABASE_PATH=/data/expenses.db\nSESSION_SECRET=%s\nCOOKIE_SECURE=true\nPUBLIC_URL=https://integradar.org\nTZ=Europe/Moscow\n' "$secret" > "$root/shared/.env"
fi
if [ -f "$root/shared/data/expenses.db" ]; then
    python3 - "$root/shared/data/expenses.db" "$root/backups/pre-$tag.db" <<'PY'
import sqlite3, sys
with sqlite3.connect(sys.argv[1]) as source, sqlite3.connect(sys.argv[2]) as destination:
    source.backup(destination)
PY
    chmod 600 "$root/backups/pre-$tag.db"
fi
tar -xzf "/tmp/tksu-pl-$tag.tar.gz" -C "$release"
docker build -f "$release/Dockerfile.runtime" -t "tksu-pl:$tag" "$release"
printf 'RELEASE_TAG=%s\n' "$tag" > "$release/.env"
previous=$(readlink -f "$root/current" || true)
docker compose -p tksu-pl --env-file "$release/.env" -f "$release/compose.production.yaml" up -d
healthy=false
for attempt in $(seq 1 20); do
    if curl --fail --silent http://127.0.0.1:8095/ping | grep -qx pong; then healthy=true; break; fi
    sleep 2
done
if [ "$healthy" != true ]; then
    docker logs --tail 30 tksu-pl
    if [ -n "$previous" ] && [ -f "$previous/compose.production.yaml" ]; then
        docker compose -p tksu-pl --env-file "$previous/.env" -f "$previous/compose.production.yaml" up -d
    fi
    echo 'Deployment health check failed' >&2
    exit 1
fi
if [ -n "$previous" ]; then printf '%s\n' "$previous" > "$root/shared/previous-release"; fi
ln -sfn "$release" "$root/current"
echo "Deployed tksu-pl:$tag; local /ping is healthy."
