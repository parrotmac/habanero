gen:
    #!/usr/bin/env bash
    (docker kill habanero-gen-db || true); docker run -d --rm --name habanero-gen-db -p 5409:5432 -e POSTGRES_HOST_AUTH_METHOD=trust timescale/timescaledb:latest-pg15

    for i in {1..10}; do
      if docker exec habanero-gen-db pg_isready; then
        break
      fi
      sleep 1
    done
    psql 'postgres://postgres:postgres@localhost:5409/postgres' < models/schema.sql
    pggen gen go --postgres-connection 'postgres://postgres:postgres@localhost:5409/postgres?sslmode=disable' --query-glob models/**/queries*.sql --go-type 'int8=int' --go-type 'float8=float64' --go-type 'text=string' --go-type 'varchar=string' --go-type 'uuid=github.com/google/uuid.UUID' --go-type 'timestamp=time.Time' --go-type 'timestamptz=time.Time' --go-type 'jsonb=[]byte'
    docker kill habanero-gen-db


buf:
    buf lint

buf-es:
    cd web && npx $(which zsh) -c 'cd .. && buf build'
    cd web && npx $(which zsh) -c 'cd .. && buf generate'

psql:
    psql 'postgres://postgres:password@localhost:5429/habanero'

db:
    docker compose down -v
    docker compose up -d

initdb:
    psql 'postgres://postgres:password@localhost:5429/postgres' -c 'drop database if exists habanero'
    psql 'postgres://postgres:password@localhost:5429/postgres' -c 'create database habanero'
    psql 'postgres://postgres:password@localhost:5429/habanero' < models/schema.sql

dev:
    go run main.go
