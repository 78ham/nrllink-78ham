#!/bin/bash
# 编译 libcodec2 为 WebAssembly
# 需要安装 emsdk: https://emscripten.org/docs/getting_started/downloads.html

set -e

echo "编译 Codec2 WASM 模块..."

# 克隆 codec2
if [ ! -d "codec2" ]; then
    git clone --depth 1 https://github.com/drowe67/codec2.git
fi

cd codec2

# 创建 build 目录
mkdir -p build-wasm
cd build-wasm

# 使用 emscripten 配置
emcmake cmake .. -DCMAKE_BUILD_TYPE=Release

# 编译
emmake make -j$(nproc)

# 复制产物
echo "复制 WASM 文件到 public/wasm..."
mkdir -p ../../public/wasm
cp src/codec2.js ../../public/wasm/codec2.js
cp src/codec2.wasm ../../public/wasm/codec2.wasm

echo "编译完成!"
echo "生成文件:"
echo "  - public/wasm/codec2.js"
echo "  - public/wasm/codec2.wasm"
