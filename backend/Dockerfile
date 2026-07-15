FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.20
# Run as an unprivileged user and give it ownership of the storage volume.
RUN adduser -D -u 10001 app \
    && mkdir -p /data/storage \
    && chown -R app:app /data
COPY --from=build /out/api /usr/local/bin/api
USER app
WORKDIR /data
ENV STORAGE_ROOT=/data/storage
EXPOSE 8008
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8008/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/api"]
