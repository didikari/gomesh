# Stage 1: Build static binary
FROM golang:1.23-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile pure Go static binary without CGO
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/gomesh ./cmd/gomesh

# Stage 2: Minimal runtime image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/gomesh /app/gomesh

RUN mkdir -p /app/data

VOLUME ["/app/data"]

EXPOSE 8080

ENTRYPOINT ["/app/gomesh"]
CMD ["-port", "8080", "-db", "/app/data/gomesh.db"]
