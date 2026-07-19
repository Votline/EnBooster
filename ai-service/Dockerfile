FROM golang:1.26.3-alpine AS builder

WORKDIR /build/aiservice

COPY ai-service/go.mod ai-service/go.sum ./
RUN go mod download -x

COPY ai-service ./
RUN go build -o aiservice main.go

FROM archlinux:latest

WORKDIR /aiservice

RUN pacman -Syu --noconfirm \
    rhvoice \
    rhvoice-language-russian \
    rhvoice-voice-anna \
    bash \
    && pacman -Scc --noconfirm

COPY --from=builder /build/aiservice/aiservice ./
COPY --from=builder /build/aiservice/ssl ./ssl

COPY ai-service/scripts ./scripts

RUN chmod +x ./scripts/rhvoice

CMD ["./aiservice"]
