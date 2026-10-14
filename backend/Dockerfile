FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
# Persist the module and build caches across builds so a small backend change
# recompiles only the affected packages instead of the whole program.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /devmail ./cmd/api

FROM postgres:18-bookworm
RUN mkdir -p /var/lib/devmail/backups && chown postgres:postgres /var/lib/devmail/backups
COPY --from=build /devmail /usr/local/bin/devmail
USER postgres
ENV HTTP_ADDR=:7000 PG_DUMP_PATH=/usr/lib/postgresql/18/bin/pg_dump BACKUP_DIRECTORY=/var/lib/devmail/backups
EXPOSE 7000
ENTRYPOINT ["/usr/local/bin/devmail"]
