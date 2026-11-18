#!/usr/bin/env bash
# Brings up what the OpenLineage producer tests run against: a source Postgres the producers and plugins read, a Postgres for Marmot, and Marmot itself built from this checkout with the Postgres and DuckDB plugins. Everything is local and throwaway.
#
#   stack.sh up     start everything and write .stack/env
#   stack.sh down   stop Marmot and remove the containers
#
# Env (optional): MARMOT_IT_PORT (8080), SOURCE_PG_PORT (55443), MARMOT_DB_PORT (55444).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../../.." && pwd)"
stack="$here/.stack"
port="${MARMOT_IT_PORT:-8080}"
src_port="${SOURCE_PG_PORT:-55443}"
db_port="${MARMOT_DB_PORT:-55444}"
src_container=marmot-ol-it-source
db_container=marmot-ol-it-db

wait_for_pg() {
  for _ in $(seq 1 60); do
    docker exec "$1" pg_isready -q -U "$2" && return 0
    sleep 1
  done
  echo "$1 did not start" >&2
  return 1
}

down() {
  if [[ -f "$stack/marmot.pid" ]]; then
    kill "$(cat "$stack/marmot.pid")" 2>/dev/null || true
    rm -f "$stack/marmot.pid"
  fi
  docker rm -f "$src_container" "$db_container" >/dev/null 2>&1 || true
}

up() {
  down
  mkdir -p "$stack/plugins"
  umask 077

  docker run -d --name "$src_container" -e POSTGRES_USER=shop -e POSTGRES_PASSWORD=shop -e POSTGRES_DB=shop -p "127.0.0.1:$src_port:5432" postgres:17 >/dev/null
  docker run -d --name "$db_container" -e POSTGRES_USER=marmot -e POSTGRES_PASSWORD=marmot -e POSTGRES_DB=marmot -p "127.0.0.1:$db_port:5432" postgres:17 >/dev/null
  wait_for_pg "$src_container" shop
  wait_for_pg "$db_container" marmot
  # pg_isready answers before the init scripts finish, so give the server a moment to restart.
  sleep 3
  docker exec -i "$src_container" psql -q -v ON_ERROR_STOP=1 -U shop shop < "$here/source/seed.sql"

  echo "building marmot and plugins"
  (cd "$repo" && go build -o "$stack/marmot" ./cmd/main.go)
  # The plugins build against their own pinned plugin SDK rather than a workspace checkout.
  (cd "$repo/plugins/postgresql" && GOWORK=off go build -o "$stack/plugins/marmot-plugin-postgresql" .)
  (cd "$repo/plugins/duckdb" && GOWORK=off go build -o "$stack/plugins/marmot-plugin-duckdb" .)

  (
    export MARMOT_DATABASE_HOST=127.0.0.1 MARMOT_DATABASE_PORT="$db_port" MARMOT_DATABASE_USER=marmot MARMOT_DATABASE_PASSWORD=marmot MARMOT_DATABASE_NAME=marmot
    export MARMOT_SERVER_PORT="$port" MARMOT_SERVER_ALLOW_UNENCRYPTED=true MARMOT_TELEMETRY_ENABLED=false MARMOT_LOGGING_FORMAT=json
    export MARMOT_PLUGINS_DIR="$stack/plugins" MARMOT_PLUGINS_AUTOINSTALL=false MARMOT_PLUGINS_UPDATES_ENABLED=false MARMOT_PLUGIN_CACHE_DIR="$stack/plugin-cache"
    nohup "$stack/marmot" run > "$stack/marmot.log" 2>&1 &
    echo $! > "$stack/marmot.pid"
  )
  url="http://127.0.0.1:$port"
  for _ in $(seq 1 60); do
    curl -s -o /dev/null "$url/api/v1/mrn/resolve" && break
    sleep 1
  done

  # The first admin is created by a migration with the password admin and must change it; the stack sets a random one.
  password="$(head -c 18 /dev/urandom | base64 | tr -d '/+=' | head -c 20)"
  token="$(curl -sf -X POST "$url/api/v1/users/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin"}' | jq -r .access_token)"
  curl -sf -o /dev/null -X POST "$url/api/v1/users/update-password" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"new_password\":\"$password\"}"
  printf '%s\n' "$password" > "$stack/admin-password"
  token="$(curl -sf -X POST "$url/api/v1/users/login" -H 'Content-Type: application/json' -d "{\"username\":\"admin\",\"password\":\"$password\"}" | jq -r .access_token)"
  key="$(curl -sf -X POST "$url/api/v1/users/apikeys" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"name":"openlineage-it","expires_in_days":1}' | jq -r .key)"

  cat > "$stack/env" <<ENV
MARMOT_URL=$url
MARMOT_API_KEY=$key
MARMOT_BIN=$stack/marmot
SOURCE_PG=host=127.0.0.1 port=$src_port dbname=shop user=shop password=shop
SOURCE_PG_HOST=127.0.0.1
SOURCE_PG_PORT=$src_port
ENV
  echo "Marmot is at $url (user admin, password in $stack/admin-password)"
}

case "${1:-}" in
  up) up ;;
  down) down ;;
  *) echo "usage: $0 up|down" >&2; exit 2 ;;
esac
