FROM golang:1.26.3-alpine AS builder

WORKDIR /build/aiservice

COPY ai-service/go.mod ai-service/go.sum ./
RUN go mod download -x

COPY ai-service ./
RUN go build -o aiservice main.go

FROM alpine:3.18

WORKDIR /aiservice

COPY --from=builder /build/aiservice/aiservice ./
COPY --from=builder /build/aiservice/ssl ./ssl
CMD ["./aiservice"]
