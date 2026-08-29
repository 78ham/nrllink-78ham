# ---- Stage 0: Build codec2 native library ----
FROM alpine:3.21 AS codec2build
RUN apk add --no-cache cmake g++ make git
RUN git clone --depth 1 https://github.com/drowe67/codec2.git /codec2
WORKDIR /codec2
RUN mkdir build && cd build && cmake .. -DCMAKE_BUILD_TYPE=Release && make -j$(nproc)

# ---- Stage 1: Build Go binary ----
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc musl-dev pkgconf opus-dev opusfile-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=codec2build /codec2/src/codec2/ /app/codec2/src/codec2/
COPY --from=codec2build /codec2/build/src/ /app/codec2/build/src/
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o udphub .

# ---- Stage 2: Runtime ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata opus opusfile
COPY --from=codec2build /codec2/build/src/libcodec2.so* /usr/lib/
RUN mkdir -p /nrllink/udphub /nrllink/data /nrllink/conf
COPY --from=builder /app/udphub /nrllink/udphub/udphub
COPY start.sh /nrllink/start.sh
RUN chmod +x /nrllink/start.sh

WORKDIR /nrllink
EXPOSE 9000 60050/udp
ENTRYPOINT ["/nrllink/start.sh"]
