FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/go-sdk-server ./cmd/mcp/go-sdk-server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/go-sdk-server /usr/local/bin/go-sdk-server

EXPOSE 8080

ENTRYPOINT ["go-sdk-server", "--transport", "http", "--addr", "0.0.0.0:8080"]
