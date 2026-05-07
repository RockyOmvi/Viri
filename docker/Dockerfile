FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .

RUN apk add --no-cache make git
RUN make build

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/build/virid /usr/local/bin/virid
COPY --from=builder /app/build/virictl /usr/local/bin/virictl

RUN addgroup -g 1000 viri && \
    adduser -D -u 1000 -G viri viri

USER viri

EXPOSE 8545 8546 30303

ENTRYPOINT ["virid"]
