FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go build -o upstream-1 .

FROM alpine:3.21
COPY --from=builder /app/upstream-1 /usr/local/bin/upstream-1
ENTRYPOINT ["upstream-1"]
