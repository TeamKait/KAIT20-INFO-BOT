# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/infobot ./cmd/bot

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& addgroup -S app \
	&& adduser -S -G app -h /data app

COPY --from=build /out/infobot /usr/local/bin/infobot

RUN mkdir -p /data && chown -R app:app /data

USER app
ENV VAULT_DIR=/data/vault
VOLUME ["/data"]

ENTRYPOINT ["infobot"]
