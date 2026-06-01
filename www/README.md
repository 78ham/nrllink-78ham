# NRLLink Web Frontend

## 功能特性

- **暗色主题** - 无线电爱好者风格
- **多编码支持** - G.711, Opus, Codec2 (通过 WASM)
- **Web 语音对讲** - WebSocket 实时音频
- **设备管理** - 设备列表、状态监控
- **群组管理** - 创建、编辑、删除群组

## 编码支持

| 编码 | Type | 码率 | 采样率 | 浏览器支持 |
|------|------|------|--------|-----------|
| G.711 | 1 | 64kbps | 8kHz | ✅ 原生 JS |
| Opus | 8 | 32-40kbps | 16kHz | ⚠️ 需要 Chrome 94+ |
| Codec2 700C | 12 | 700bps | 8kHz | ✅ WASM |
| Codec2 1300 | 13 | 1300bps | 8kHz | ✅ WASM |
| Codec2 1600 | 14 | 1600bps | 8kHz | ✅ WASM |
| Codec2 2400 | 15 | 2400bps | 8kHz | ✅ WASM |
| Codec2 3200 | 16 | 3200bps | 8kHz | ✅ WASM |

## 安装

```bash
# 安装依赖
npm install

# 编译 Codec2 WASM (需要 emsdk)
bash scripts/build-codec2-wasm.sh

# 开发服务器
npm run dev

# 生产构建
npm run build
```

## 开发

```bash
# 启动开发服务器 (代理 API 到 localhost:9000)
npm run dev

# 构建生产版本
npm run build

# 预览构建结果
npm run preview
```

## WASM 编译

需要先安装 [Emscripten SDK](https://emscripten.org/docs/getting_started/downloads.html):

```bash
# 安装 emsdk
git clone https://github.com/emscripten-core/emsdk.git
cd emsdk
./emsdk install latest
./emsdk activate latest
source ./emsdk_env.sh

# 编译 Codec2
bash scripts/build-codec2-wasm.sh
```

## 项目结构

```
www/
├── public/              # 静态资源
│   └── wasm/           # WASM 文件
├── src/
│   ├── assets/         # 样式文件
│   ├── components/     # 组件
│   ├── stores/         # Pinia store
│   ├── types/          # TypeScript 类型
│   ├── utils/          # 工具函数
│   │   ├── audio-decoder.ts  # 多编码解码器
│   │   └── index.ts    # 统一导出
│   ├── views/          # 页面组件
│   │   ├── Login.vue
│   │   ├── Dashboard.vue
│   │   ├── DeviceList.vue
│   │   ├── GroupList.vue
│   │   ├── VoiceChat.vue
│   │   └── Settings.vue
│   ├── App.vue
│   ├── main.ts
│   └── router.ts
├── scripts/
│   └── build-codec2-wasm.sh
├── package.json
├── tsconfig.json
├── vite.config.ts
└── env.d.ts
```

## WebSocket 协议

音频帧格式:
```
[1字节 CodecType] + [N字节 音频数据]
```

CodecType 值:
- `1` = G.711 A-law
- `8` = Opus
- `12-16` = Codec2

## 主题颜色

```css
--bg-primary: #0a0e14;    /* 深空黑 */
--bg-card: #1e2538;        /* 卡片背景 */
--accent: #00d4aa;         /* 科技绿 */
--accent-warning: #ff6b35; /* 警告橙 */
--online: #00ff88;         /* 在线绿 */
--offline: #ff4444;        /* 离线红 */
```
