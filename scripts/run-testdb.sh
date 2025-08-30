#!/usr/bin/env bash

set -euo pipefail

docker container rm -f test_postgres || true

port="$(docker container port \
        "$(docker container run -d --name test_postgres \
                                -e POSTGRES_PASSWORD=secret \
                                -e POSTGRES_DB=postgres \
                                -e POSTGRES_USER=postgres \
                                -p 0.0.0.0:0:5432 postgres:17)" 5432 \
                                | head -1 | sed 's/.*:\([0-9]*\)/\1/')"

TEST_DB_DSN="postgres://postgres:secret@localhost:${port}/postgres?sslmode=disable"
export TEST_DB_DSN

set +x
set +e
set +u
set +o pipefail