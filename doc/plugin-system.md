# NRL Link 插件系统总体规划（v2 - 语音桥接版）

> 版本：2026-11-28 | 模块：`udphub` | 适用：78ham 分支
>
> 本版规划以「语音桥接」为核心目标：让其他平台（其他 NRL 服务器 / Brandmeister / 自建平台）通过插件接入本服务器。

---

## 1. 目标

1. **插件化底座**（已完成）：Hook 派发、插件生命周期、启停/配置持久化、管理 API。
2. **语音桥接插件**（本期核心）：将外部平台的语音/信令双向桥接到本地群组。
3. **开放接入**：第三方平台可按外部插件协议接入，无需修改主程序。

## 2. 架构总览

```
                    ┌────────────────────────────┐
                    │        主服务 (udphub)       │
                    │                            │
  NRL2 设备 ──UDP──▶│  udphub.go ──▶ hook 派发    │
                    │      │             │        │
                    │      ▼             ▼        │
                    │  转发引擎   ┌─────────────┐  │
                    │            │ HookRegistry │  │
                    │            └─────┬───────┘  │
                    │                  │          │
                    │        ┌─────────┼────────┐ │
                    │        ▼         ▼        ▼ │
                    │   ┌───────┐ ┌────────┐ ┌──────────┐
                    │   │ aprs  │ │ bridge │ │ 外部插件  │──▶ 第三方平台
                    │   └───────┘ └───┬────┘ └──────────┘
                    │                 │
                    └─────────────────┼────────────
                                      ▼
                          外部平台（NRL 对端 / BM / 自建）
```

## 3. 已完成：插件底座（第一期）

| 组件 | 文件 | 说明 |
|---|---|---|
| Hook 注册表 | `plugin.go` | 8 个 hook 点，异步派发，panic 隔离 |
| 插件管理器 | `pluginmgr.go` | `Plugin` 接口、生命周期、`plugin_status` 表、管理 API |
| APRS 插件 | `plugin_aprs.go` | 首个内置插件（原 APRS 模块迁移） |

**Hook 点清单**：

| Hook | 触发时机 | 典型用途 |
|---|---|---|
| `voice.before_forward` | 语音帧转发前（`forwardVoice` 前） | 录音、桥接出站采样 |
| `msg.before_forward` | 文本消息转发前 | 消息桥接、审计 |
| `ctl.before_forward` | 控制信令转发前 | 信令审计 |
| `device.online` / `device.offline` | 设备上下线 | 通知、统计 |
| `group.join` / `group.leave` | 群组进出（预留） | 计费、权限 |
| `server.tick`（30s） | 全局定时器 | 心跳、上报 |

## 4. 核心：语音桥接插件设计

### 4.1 桥接方向与数据流

```
本端设备 ──voice.before_forward──▶ bridge 插件 ──NRL2 over UDP──▶ 对端平台
对端平台 ──NRL2 over UDP──▶ bridge 插件 ──NRL2 注入──▶ 本地群组转发（forwardServerVoice 同款路径）
```

### 4.2 桥接身份：虚拟设备（DevModel=255）

- 桥接插件为每个桥接会话构造一个**虚拟设备**（DevModel=255、SSID=255，即"外部服务器"身份），
  复用 `forwardServerVoice` 的既有转发逻辑，与现有 200/255 互联机制完全兼容；
- 呼号使用桥接配置中的 `bridge_callsign`（如 `BRIDGEM`），`deviceInfo` 由插件按需构造；
- 对端来的语音：解码后以桥接身份注入群组，组内设备收到与外部服务器互联同构的包；
- 本端去的语音：在 `voice.before_forward` hook 中采样，编码为 NRL2 包发往对端。

### 4.3 配置模型（存于 plugin_status.config，管理 API 可改）

```json
{
  "bridges": [
    {
      "name": "bm-4601",
      "type": "nrl",
      "remote_host": "1.2.3.4",
      "remote_port": 60050,
      "local_group_id": 5,
      "bridge_callsign": "BRIDGEM",
      "bridge_ssid": 88,
      "codec": 1,
      "tx_enabled": true,
      "rx_enabled": true,
      "max_duration_sec": 120,
      "hook_priority": 50
    }
  ]
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `type` | 协议类型：`nrl`（NRL2 over UDP）；`bm`/`custom` 预留 |
| `local_group_id` | 桥接绑定的本地公共群组 ID |
| `bridge_callsign` / `bridge_ssid` | 桥接虚拟身份（用于防环与语音权） |
| `codec` | 出站编码：1=G.711（对端为 NRL 设备时常用） |
| `tx_enabled` / `rx_enabled` | 本端→对端 / 对端→本端 方向开关 |
| `max_duration_sec` | 单次通话最长时间（防呆，超时自动切断） |
| `hook_priority` | hook 优先级（数值小先执行；桥接建议 50） |

### 4.4 运行时与状态

- 每条桥一个 goroutine：UDP 会话 + 心跳（NRL2 Type=2 心跳包）+ 重连退避（1s/2s/5s/10s/30s，60s 封顶）。
- 状态持久化：独立表 `bridge_status`（name、connected、rx_packets、tx_packets、last_error、last_active）。
- 统计：rx/tx 包数、丢包率、最近错误、最近活跃时间，管理 API 直接读表展示。

### 4.5 防环与防呆

1. **防环**：注入本地的包以桥接呼号 `BRIDGEM-88` 为源；`voice.before_forward` hook 过滤"来源呼号 == 自身桥接呼号"的帧，杜绝本端↔对端乒乓循环。
2. **语音权抢占**：与 `forwardServerVoice` 一致，遵守群组语音权状态机（200ms 抢占窗口），桥接出站先检查群组语音权，避免对端与本端同时说话造成混叠。
3. **限流**：单桥最大转发帧率 ≤50 帧/秒，超出丢弃并计数。

## 5. 外部插件协议（供第三方平台接入）

第三方平台以独立进程实现，通过 HTTP + 回调接入主服务，适合不实现 NRL2 协议的平台。

### 5.1 注册

```
POST /api/v1/plugins/external/register
Header: X-Plugin-Token: <共享密钥（配置项 PluginToken）>
Body: {
  "name": "my-platform",
  "display": "我的平台",
  "hooks": ["voice.before_forward", "device.online"],
  "callback_url": "http://127.0.0.1:9100/hook"
}
```

### 5.2 事件回调转发（主服务 → 插件）

```
POST <callback_url>
Body: {
  "hook": "voice.before_forward",
  "ts": 1764280000,
  "device": { "callsign": "BG7XXX", "ssid": 1, "group_id": 5 },
  "audio": "<base64 G.711 帧（可选，按订阅声明下发）>"
}
```

2 秒超时，连续 3 次失败自动摘除该插件。语音帧 payload 为 base64 G.711 帧，按订阅声明下发。

### 5.3 主服务向外部插件开放只读 API

- `GET /api/v1/plugins/external/status` —— 当前注册的外部插件与统计。

## 6. 前端对接要点

详见 `doc/plugin-frontend-api.md`：

- 管理页新增「插件管理」：列表 + 开关 + 配置编辑器（JSON）+ 桥接状态面板。
- 接口前缀 `/api/v1/plugins*`，鉴权沿用 x-token + admin 角色。

## 7. 实施顺序

1. ~~插件底座~~（已完成）。
2. `bridge` 插件：NRL2 over UDP 桥接 + 防环 + `bridge_status` 表 + 管理 API（复用 /plugins/config）。
3. 外部插件注册协议 + token 鉴权 + 回调转发器。
4. 前端「插件管理」页（对接文档：`doc/plugin-frontend-api.md`）。
5. 插件开发指南：`doc/plugin-dev.md`。

> 注：`server.tick` 每 30 秒一次；桥接心跳用各自独立 ticker，`server.tick` 只做统计刷新等轻量任务。
