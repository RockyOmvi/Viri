FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /virid ./cmd/virid
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /virictl ./cmd/virictl

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tini curl jq

RUN addgroup -g 1000 viri && \
    adduser -u 1000 -G viri -s /bin/sh -D viri

WORKDIR /home/viri

COPY --from=builder /virid /usr/local/bin/virid
COPY --from=builder /virictl /usr/local/bin/virictl

COPY deploy/entrypoint.sh /entrypoint.sh
COPY deploy/peer-discovery.sh /home/viri/peer-discovery.sh
RUN chmod +x /entrypoint.sh /home/viri/peer-discovery.sh

RUN mkdir -p /home/viri/.viri /home/viri/config /home/viri/data /keys && \
    chown -R viri:viri /home/viri /keys

USER viri

EXPOSE 30303 8545 8546 8547 8080 8081 9090

VOLUME ["/home/viri/.viri", "/home/viri/config", "/home/viri/data"]

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
CMD ["virid", "--config", "/home/viri/config/config.json"]
