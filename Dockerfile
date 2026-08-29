# ---- Stage 1: Build ----
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc musl-dev pkgconf opus-dev opusfile-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o udphub .

# ---- Stage 2: Runtime ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata opus opusfile

RUN mkdir -p /nrllink/udphub /nrllink/data /nrllink/conf
COPY --from=builder /app/udphub /nrllink/udphub/udphub
COPY start.sh /nrllink/start.sh
RUN chmod +x /nrllink/start.sh

WORKDIR /nrllink
EXPOSE 9000 60050/udp
ENTRYPOINT ["/nrllink/start.sh"]
