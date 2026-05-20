FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY .git/ ./.git/
COPY talk-libs/ ./talk-libs/
COPY mcp-playground/ ./mcp-playground/

ENV GOWORK=off
RUN cd mcp-playground && go mod download
RUN cd mcp-playground && CGO_ENABLED=0 go build -o /bin/mcp-playground ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/mcp-playground /usr/local/bin/mcp-playground

EXPOSE 8080

ENTRYPOINT ["mcp-playground", "--transport", "http", "--addr", "0.0.0.0:8080"]
