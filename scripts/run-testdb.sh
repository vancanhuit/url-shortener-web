#!/usr/bin/env bash

set -euo pipefail

docker container rm -f test_postgres || true

port="$(docker container port \
    "$(docker container run -d --name test_postgres \
        -e POSTGRES_PASSWORD=secret \
        -e POSTGRES_DB=postgres \
        -e POSTGRES_USER=postgres \
        --health-cmd 'pg_isready -U postgres -d postgres' \
        --health-interval 10s \
        --health-timeout 5s \
        --health-retries 5 \
        -p 0.0.0.0:0:5432 postgres:17)" 5432 |
    head -1 | sed 's/.*:\([0-9]*\)/\1/')"

TEST_DB_DSN="postgres://postgres:secret@localhost:${port}/postgres?sslmode=disable"
export TEST_DB_DSN

echo "waiting for test database to be ready..."

count=10
for i in $(seq 1 ${count}); do
    health_status=$(docker container inspect test_postgres -f '{{.State.Health.Status}}')
    [[ ${health_status} == "healthy" ]] && break
    echo "not ready (${i}/${count})"
    sleep 5
done

echo "test database is ready"

set +x
set +e
set +u
set +o pipefail
