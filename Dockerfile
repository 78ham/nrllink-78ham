# ---- Stage 0: Build codec2 native library ----
FROM alpine:3.21 AS codec2build
RUN apk add --no-cache cmake g++ make git
RUN git clone --depth 1 https://github.com/drowe67/codec2.git /codec2
WORKDIR /codec2
RUN mkdir build && cd build && cmake .. -DCMAKE_BUILD_TYPE=Release && make -j$(nproc)
RUN mkdir -p /codec2/src/codec2 && cp /codec2/src/*.h /codec2/src/codec2/ && \
    cp /codec2/build/codec2/version.h /codec2/src/codec2/version.h

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
# 本镜像只含后端服务，不打包前端：
#   - 生产前端用 nrllink-web 仓库，产物挂到 /nrllink/www
#   - 本仓库的 www/ 只是开发用测试前端，不随镜像分发
# 镜像内置一份默认 udphub.yaml，开箱即可运行；
# 想自定义就挂一个配置到 /nrllink/conf/udphub.yaml（启动脚本会自动优先加载它）。
# wget 供 compose 健康检查使用。
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata opus opusfile wget
COPY --from=codec2build /codec2/build/src/libcodec2.so* /usr/lib/
RUN mkdir -p /nrllink/udphub /nrllink/data /nrllink/conf /nrllink/www/uploads
COPY --from=builder /app/udphub /nrllink/udphub/udphub
COPY docker/udphub.default.yaml /nrllink/udphub/udphub.yaml
COPY start.sh /nrllink/start.sh
RUN chmod +x /nrllink/start.sh

WORKDIR /nrllink
EXPOSE 9000 60050/udp
ENTRYPOINT ["/nrllink/start.sh"]