FROM node:22-alpine AS ui
WORKDIR /ui
COPY client/package.json client/package-lock.json ./
RUN npm ci
COPY client/ ./
RUN npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/vocalis ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 vocalis \
    && mkdir -p /var/lib/vocalis/files \
    && chown -R vocalis /var/lib/vocalis
COPY --from=server /out/vocalis /usr/local/bin/vocalis
COPY --from=ui /ui/dist /srv/ui
ENV UI_DIR=/srv/ui \
    STORAGE_DIR=/var/lib/vocalis/files \
    HTTP_ADDR=:8080 \
    ENV=production
VOLUME /var/lib/vocalis
USER vocalis
EXPOSE 8080/tcp
EXPOSE 50000-50999/udp
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["vocalis"]
