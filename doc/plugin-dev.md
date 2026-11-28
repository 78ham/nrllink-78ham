# NRL Link 插件开发指南

> 面向：想给 NRL Link 写内置插件（Go，同仓库编译）或外部插件（独立进程，HTTP 回调）的开发者。
>
> 示例代码以仓库内实际接口为准（模块名 `udphub`，Go 1.21+）。

---

## 1. 两种插件形态

| 形态 | 适用 | 载体 |
|---|---|---|
| **内置插件** | 与主程序同生命周期、需要访问内部状态（群组、设备、UDP 连接） | 同仓库 Go 文件，编译进单二进制 |
| **外部插件** | 独立部署、第三方平台接入、非 Go 语言实现 | 独立进程，HTTP 回调 |

## 2. 内置插件开发

### 2.1 实现 Plugin 接口

```go
package main

// myplugin.go

type myPlugin struct {
    stop chan struct{}
}

func newMyPlugin() *myPlugin { return &myPlugin{stop: make(chan struct{})} }

func (p *myPlugin) Name() string { return "myplugin" }

func (p *myPlugin) OnLoad(ctx *PluginContext) error {
    // 1. 读配置
    var conf struct {
        Foo string `json:"foo"`
    }
    if ctx.GetConf(&conf) {
        ctx.Log.Printf("配置 foo=%s", conf.Foo)
    }

    // 2. 注册 hook
    ctx.Hooks.Register(p.Name(), HookDeviceOnline, func(e *HookEvent) {
        ctx.Log.Printf("设备上线: %s-%s", e.Device.CallSign, e.Device.SSID)
    }, 50)

    // 3. 后台任务（必须可被 OnUnload 停止）
    go p.run(ctx)
    return nil
}

func (p *myPlugin) run(ctx *PluginContext) {
    t := time.NewTicker(time.Minute)
    defer t.Stop()
    for {
        select {
        case <-p.stop:
            return
        case <-t.C:
            ctx.Log.Printf("tick")
        }
    }
}

func (p *myPlugin) OnUnload() {
    close(p.stop) // 停止后台任务；hook 会被管理器自动注销
}
```

### 2.2 注册插件

在 `pluginmgr.go` 的 `pluginLoadAll()` 中追加一行：

```go
registerPlugin("myplugin", "我的插件描述", func() Plugin { return newMyPlugin() })
```

启动后自动建 `plugin_status` 表记录，默认启用；管理 API 可停用/改配置。

### 2.3 PluginContext 能力一览

| 成员 | 类型 | 说明 |
|---|---|---|
| `ctx.Name` | string | 插件名 |
| `ctx.Conf` | json.RawMessage | plugin_status.config 中的 JSON |
| `ctx.GetConf(&dst)` | helper | 解析配置到结构体，空/失败返回 false |
| `ctx.Hooks` | *HookRegistry | 全局 hook 注册表 |
| `ctx.Log` | *log.Logger | 带 `[plugin:<name>]` 前缀的日志 |

数据库直接用全局 `db`（SQLCipher SQLite）；插件建议自建表（如 `myplugin_xxx`），不要改其他模块的表。

### 2.4 Hook 事件

| Hook 常量 | 字符串值 | e.Device | e.Group | e.Packet |
|---|---|---|---|---|
| `HookVoiceBeforeForward` | `voice.before_forward` | 发话设备 | 群组 | 原始 NRL2 帧 |
| `HookMsgBeforeForward` | `msg.before_forward` | 发送设备 | nil | 原始 NRL2 帧 |
| `HookCtlBeforeForward` | `ctl.before_forward` | nil | 群组 | 原始 NRL2 帧 |
| `HookDeviceOnline` | `device.online` | 上线设备 | 群组 | nil |
| `HookDeviceOffline` | `device.offline` | 离线设备 | 群组 | nil |
| `HookGroupJoin/Leave` | `group.join/leave` | 设备 | 群组 | nil（预留） |
| `HookServerTick` | `server.tick` | nil | nil | nil（30s 一次） |

**关键规则**：

1. hook 回调运行在派发 goroutine（异步派发时为独立 goroutine），**不要阻塞**；
2. `e.Packet` 是拷贝副本，可以安全持有，但注意内存（语音帧每 60ms 一帧）；
3. 回调 panic 会被 recover 并记日志，但别依赖这个兜底；
4. 同一 hook 内按 `priority` 升序执行，数值小的先执行。

### 2.5 语音类插件注意事项

- 语音热路径每秒可达 50 帧 × 设备数，回调里做重活（转码/写盘/网络请求）请自行开缓冲队列：
  回调只 `select { case ch <- frame: default: /* 丢弃 */ }`，消费 goroutine 慢慢处理；
- 需要向群组注入语音请参考 `plugin_bridge.go`（桥接插件）的注入路径，或直接使用群组连接池
  `gp.connPool.snapshotList()` + `globelconn.WriteToUDP`；
- 编码/解码复用 `codec.go` 的池化接口（`GetPooledCodec` / `DecodeToPCM`），不要每帧 new。

## 3. 桥接插件（plugin_bridge.go）专用说明

桥接插件是"向外部平台收发语音"的参考实现，核心机制：

- **虚拟设备身份**：每条桥以 `bridge_callsign-bridge_ssid`（DevModel=255）为源呼号，防环过滤即"丢弃来源呼号==自身桥接呼号的帧"；
- **注入本地**：对端语音 → 构造 NRL2 帧（`encodeNRL21`，DevModel=255）→ 直接写入群组连接池设备（与外部服务器互联同构）；
- **出站采样**：`voice.before_forward` hook → 过滤桥接呼号 → 取帧 DATA（G.711）→ `encodeNRL21` 打包 → UDP 发对端；
- **状态表**：`bridge_status`（name, connected, rx_packets, tx_packets, last_error, last_active），管理 API 读取展示。

配置结构见 `doc/plugin-frontend-api.md` §3.2。

## 4. 外部插件协议（非 Go / 独立进程）

### 4.1 注册

```
POST /api/v1/plugins/external/register
Header: X-Plugin-Token: <PluginToken 配置项>
Content-Type: application/json

{
  "name": "my-platform",             // 唯一
  "display": "我的平台",
  "hooks": ["voice.before_forward", "device.online"],
  "callback_url": "http://127.0.0.1:9100/hook"
}
```

- 注册成功后主服务将订阅的 hook 事件转发到 `callback_url`；
- 连续 3 次回调失败（超时 2s 计失败）自动摘除，需重新注册；
- 语音帧以 base64 G.711 下发，仅在 `hooks` 中声明 `voice.before_forward` 才会收到音频。

### 4.2 回调负载

```json
{
  "hook": "voice.before_forward",
  "ts": 1764280000,
  "device": { "callsign": "BG7XXX", "ssid": 1, "group_id": 5 },
  "audio": "base64..."   // 仅语音 hook 且订阅了音频
}
```

### 4.3 查询外部插件状态

```
GET /api/v1/plugins/external/status
Header: x-token: <admin token>
```

## 5. 生命周期与运维

```
pluginLoadAll()
   │  按 name 排序逐个加载
   ├─ pluginGetStatus(name) ── enabled=false ──▶ 跳过
   └─ pluginStart(name)
        ├─ factory() 构造实例
        ├─ 组装 PluginContext（配置/日志/HookRegistry）
        ├─ OnLoad(ctx) ── err ──▶ 记日志，不入运行表（不阻断其他插件）
        └─ runningPlugins[name] = inst

管理 API:
  enable  ─▶ pluginSetStatus(true)  + pluginStart
  disable ─▶ pluginSetStatus(false) + pluginStop（UnregisterAll hooks + OnUnload）
  config/update ─▶ 落库 + 若运行中则 stop+start 热重启
```

- 单插件 OnLoad panic 会被捕获并摘除，不影响主服务与其他插件；
- 改配置通过 config/update 热重启对应插件，无需重启进程；
- 主程序重启后按 plugin_status 自动恢复启用状态。

## 6. 开发检查清单

- [ ] `Name()` 全局唯一，用小写字母+连字符
- [ ] OnLoad 里后台任务必须可被 OnUnload 停止（stop channel / context）
- [ ] hook 回调非阻塞，重活丢队列
- [ ] 语音类插件处理帧率上限，防止内存膨胀
- [ ] 配置通过 ctx.GetConf 解析，容忍空配置
- [ ] 自建表用插件名前缀，启动时 CREATE TABLE IF NOT EXISTS
- [ ] 注册进 pluginLoadAll()，写一行 registerPlugin
- [ ] 编译验证：`go build ./...`（Windows 本机 CGO 缺 libopus 时用桩文件验证，CI/Linux 环境完整编译）
