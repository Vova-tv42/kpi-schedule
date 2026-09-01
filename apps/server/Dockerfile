FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
# Distroless has no shell to mkdir with, so create the SQLite data directory
# here, in the build stage, and copy the (empty) directory into the final image.
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
COPY --from=build /out/data /data
# Persist the SQLite database across container restarts / VM sleep-wake —
# mount a volume (or bind a host path on the persistent disk) at /data. See
# docs/architecture/data-storage.md §5.
VOLUME ["/data"]
ENV DATABASE_PATH=/data/kpi.db
EXPOSE 8080
ENTRYPOINT ["/server"]
