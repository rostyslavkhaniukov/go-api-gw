FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go build -o upstream-2 .

FROM alpine:3.21
COPY --from=builder /app/upstream-2 /usr/local/bin/upstream-2
ENTRYPOINT ["upstream-2"]
