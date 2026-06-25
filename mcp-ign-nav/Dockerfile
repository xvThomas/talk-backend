FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY .git/ ./.git/
COPY talk-libs/ ./talk-libs/
COPY mcp-ign-nav/ ./mcp-ign-nav/

ENV GOWORK=off
RUN VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev") && \
    cd mcp-ign-nav && go mod download && \
    CGO_ENABLED=0 go build -ldflags "-X github.com/xvThomas/talk-backend/talk-libs/version.Version=${VERSION}" -o /bin/mcp-ign-nav ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/mcp-ign-nav /usr/local/bin/mcp-ign-nav

EXPOSE 8080

ENTRYPOINT ["mcp-ign-nav", "--transport", "http", "--addr", "0.0.0.0:8080"]
