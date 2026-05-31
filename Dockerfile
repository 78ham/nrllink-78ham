FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o udphub .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/udphub /nrllink/udphub
RUN mkdir -p /nrllink/conf /nrllink/data
COPY start.sh /nrllink/
ENTRYPOINT ["/nrllink/start.sh"]