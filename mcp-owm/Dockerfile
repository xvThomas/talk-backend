FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY .git/ ./.git/
COPY talk-libs/ ./talk-libs/
COPY mcp-owm/ ./mcp-owm/

ENV GOWORK=off
RUN VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev") && \
    cd mcp-owm && go mod download && \
    CGO_ENABLED=0 go build -ldflags "-X github.com/xvThomas/talk-backend/talk-libs/version.Version=${VERSION}" -o /bin/mcp-owm ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/mcp-owm /usr/local/bin/mcp-owm

EXPOSE 8080

ENTRYPOINT ["mcp-owm", "--transport", "http", "--addr", "0.0.0.0:8080"]
