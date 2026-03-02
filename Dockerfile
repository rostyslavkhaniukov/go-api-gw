FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates
RUN adduser -D -H -s /sbin/nologin appuser

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/api-gw ./cmd/api-gw
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/seed ./cmd/seed

FROM scratch

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/api-gw /api-gw
COPY --from=builder /app/seed /seed
COPY config.yaml /config.yaml

USER appuser

ENTRYPOINT ["/api-gw", "--config", "/config.yaml"]
