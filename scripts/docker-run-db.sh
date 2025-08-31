#!/usr/bin/env bash

set -euo pipefail

name=${1:-test_postgres}
db_name=${2:-test}
docker container rm -f "${name}" 2> /dev/null || true

port="$(docker container port \
    "$(docker container run -d --name "${name}" \
        -e POSTGRES_PASSWORD=secret \
        -e POSTGRES_DB="${db_name}" \
        -e POSTGRES_USER=postgres \
        --health-cmd "pg_isready -U postgres -d ${db_name}" \
        --health-interval 10s \
        --health-timeout 5s \
        --health-retries 5 \
        -p 0.0.0.0:0:5432 postgres:17)" 5432 |
    head -1 | sed 's/.*:\([0-9]*\)/\1/')"

DB_DSN="postgres://postgres:secret@localhost:${port}/${db_name}?sslmode=disable"
export DB_DSN

echo "waiting for database container to be ready..."

count=10
for i in $(seq 1 ${count}); do
    health_status=$(docker container inspect "$name" -f '{{.State.Health.Status}}')
    [[ ${health_status} == "healthy" ]] && break
    echo "not ready (${i}/${count})"
    sleep 5
done

echo "database container is ready"

set +x
set +e
set +u
set +o pipefail
