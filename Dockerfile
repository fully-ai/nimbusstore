FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/nimbusstore ./cmd/nimbusstore

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 nimbus
WORKDIR /app
COPY --from=builder /out/nimbusstore /app/nimbusstore
RUN mkdir -p /app/data && chown -R nimbus:nimbus /app
USER nimbus
EXPOSE 8095
ENV NIMBUS_ADDR=":8095" \
    NIMBUS_DATA_DIR="/app/data" \
    NIMBUS_SEED_DEMO="true"
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8095/healthz || exit 1
ENTRYPOINT ["/app/nimbusstore"]
