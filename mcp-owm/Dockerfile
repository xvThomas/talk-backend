FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY talk-libs/ ./talk-libs/
COPY mcp-owm/ ./mcp-owm/

ARG VERSION=dev
ENV GOWORK=off
RUN cd mcp-owm && go mod download
RUN cd mcp-owm && CGO_ENABLED=0 go build -ldflags "-X github.com/xvThomas/LLMClientWrapper/talk-libs/version.Version=${VERSION}" -o /bin/mcp-owm ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/mcp-owm /usr/local/bin/mcp-owm

EXPOSE 8080

ENTRYPOINT ["mcp-owm", "--transport", "http", "--addr", "0.0.0.0:8080"]
