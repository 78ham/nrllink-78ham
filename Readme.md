# NRL Link - 通过网络连接无线电

NRL Link 是一个基于 Go 语言的 UDP 语音转发服务器，配合 NRL 系列硬件盒子（NRL2100/2200/2300/3188/2600等）和 Vue3 Web 前端，实现无线电设备（模拟中继、手台、公网台、手机App等）的跨地域互联互通。

**原作**: BH4RPN (BG4VKI) | **78ham 二次开发** | **模块名**: `udphub` | **Go版本**: 1.21+ | **许可证**: MIT

---

## 目录

- [架构总览](#架构总览)
- [项目结构](#项目结构)
- [快速开始](#快速开始)
- [核心通信模块](#核心通信模块)
- [NRL2 协议详解](#nrl2-协议详解)
- [HTTP API 完整参考](#http-api-完整参考)
- [数据模型](#数据模型)
- [数据库表结构](#数据库表结构)
- [配置文件](#配置文件)
- [业务流程](#业务流程)
- [第三方集成](#第三方集成)
- [部署](#部署)

---

## 架构总览

```
┌──────────────┐    UDP 60050    ┌──────────────────────┐
│ NRL硬件盒子   │ ──────────────▶ │                      │
│ (NRL2100/2200)│                 │   Go 服务端           │
└──────────────┘                 │   (nrllink-78ham)   │
                                  │                      │
┌──────────────┐                  │  ┌─────────────────┐ │
│ ESP32 终端   │ ──────────────▶  │  │  UDP Hub        │ │
└──────────────┘                  │  │  (语音/信令转发)  │ │
                                  │  └───┬─────────┬───┘ │
┌──────────────┐                  │      │         │     │
│ 73HAM App    │ ──────────────▶  │  ┌───▼──┐ ┌───▼───┐ │
│ (Android)    │                  │  │ Web  │ │ APRS  │ │
└──────────────┘                  │  │ 管理  │ │ 上报  │ │
                                  │  └───┬──┘ └───────┘ │
┌──────────────┐                  │      │              │
│ Web 浏览器   │ ── HTTPS ────▶  │  ┌───▼──────────┐  │
│ (Vue3 前端)  │                  │  │ WebSocket    │  │
└──────────────┘                  │  │ (实时通话推送) │  │
                                  │  └──────────────┘  │
                                  └──────────────────────┘
                                            │
                          ┌─────────────────┼─────────────────┐
                          ▼                 ▼                  ▼
                   其他 NRL 服务器    APRS-IS 网络     WeChat/OpenAI
```

---

## 项目结构

```
nrllink-78ham/
├── main.go                  # 入口：初始化→启动各服务→UDP主循环→优雅关闭
├── config.go                # 配置加载(YAML)、数据库初始化、Schema迁移
├── http.go                  # HTTP服务器、所有路由注册(65+接口)、请求工具函数
├── routes.go                # 前端路由配置的存取
│
├── decode.go                # NRL2协议编解码(48字节包头、11种类型)
├── g711.go                  # G.711 A-law编解码(查表法)
├── udphub.go                # UDP核心：设备管理、语音转发、抢占算法、全网互通
├── tcpclient.go             # TCP客户端(断线重连,用于APRS)
├── tcphub.go                # 占位文件
│
├── websocket.go             # 简单WebSocket回声服务
├── calls_ws.go              # WebSocket实时通话中心(房间状态/音频流广播/会议混音)
│
├── device.go                # 设备HTTP接口(列表/查询/修改/AT/1W/2W参数)
├── deviceDB.go              # 设备持久层(在线监控/EEPROM读写/组切换)
│
├── users.go                 # 用户HTTP接口(登录/注销/CRUD/角色/权限)
├── usersDB.go               # 用户持久层(bcrypt密码/JWT角色/头像)
├── userinfo.go              # 用户初始化(3个私有房间、设备列表)
│
├── group.go                 # 群组HTTP接口(列表/CRUD)
├── groupDB.go               # 群组持久层(连接池管理/PCM混音引擎)
│
├── servers.go               # 服务器HTTP接口(CRUD)
├── serversDB.go             # 服务器持久层(UDP连接管理/心跳)
│
├── relay.go                 # 中继频率HTTP接口(CRUD)
├── relayDB.go               # 中继频率持久层
│
├── homepage.go              # 首页CMS HTTP接口(板块/公告/图片)
├── homepageDB.go            # 首页CMS持久层
│
├── billing.go               # 计费(套餐/微信支付Native/订单/延期)
│
├── weixin.go                # 微信公众号(消息/菜单/绑定/小程序登录)
├── weixinsdk.go             # 微信SDK(AES加解密/XML解析/签名验证)
├── weixinSendMessage.go     # 微信模板消息(登录成功/失败/绑定确认)
├── weixinUserInfo.go        # 微信用户数据层
│
├── aprs.go                  # APRS客户端(位置上报/状态上报/自发现)
├── aprsget.go               # APRS服务器发现(aprs.tv API查询)
├── serverList.go            # 平台服务器同步(nrlptt.com上报/同步)
│
├── chatgpt.go               # OpenAI集成(GPT对话/上下文管理)
├── control.go               # 设备EEPROM参数结构体与解码(1W/2W/Moto模块)
│
├── token.go                 # JWT令牌(生成/验证/HMAC-SHA256)
├── tools.go                 # 工具函数(响应常量/权限检查/IP地理位置)
│
├── log.go                   # 通话日志(文件轮转/10分钟滚动)
├── operatorlog.go           # 操作日志HTTP接口
├── operatorlogDB.go         # 操作日志持久层
│
├── go.mod / go.sum          # Go依赖管理
├── Makefile                 # OpenWrt风格构建
├── Dockerfile               # 多阶段Docker构建
├── .goreleaser.yaml         # 多平台发布配置
├── udphub.yaml              # 默认配置文件
├── udphub.service           # systemd服务文件
│
└── doc/
    ├── 系统架构.md           # 579行完整系统架构文档
    ├── 系统架构.html         # HTML渲染版
    └── svg/                  # Mermaid架构图SVG(9张)
```

---

## 快速开始

### Docker

```bash
docker pull 78ham/nrllink:latest
docker run -d \
  -p 80:80 \
  -p 60050:60050/udp \
  -v /data:/nrllink/data \
  -v /conf:/nrllink/conf \
  78ham/nrllink:latest
```

### 直接运行

```bash
# 编译
go build -o nrllink .

# 运行（配置文件 udphub.yaml 须与二进制同目录）
./nrllink

# 指定配置文件
./nrllink -c /path/to/udphub.yaml

# 输出解析后的配置（用于调试）
./nrllink -c udphub.yaml -o json
./nrllink -c udphub.yaml -o yaml
```

### systemd

```bash
cp udphub.service /usr/lib/systemd/system/
systemctl daemon-reload
systemctl enable udphub.service
systemctl start udphub.service
```

---

## 核心通信模块

### main.go - 启动流程

```
main()
  ├── conf.init()                              # 加载udphub.yaml
  ├── initTokenKey()                           # 初始化JWT签名密钥
  ├── dbip = ipdb.City()                       # 加载IP地理位置数据库
  ├── getDB()                                  # 打开SQLite数据库(支持NRL_DBFILE环境变量; PRAGMA integrity_check)
  ├── execDDL()                                # 无条件、幂等、全量建表(schema唯一来源)
  ├── updatedb()                               # 增量Schema迁移检查与执行(每条仅一次)
  ├── ensureBootstrap()                        # 启动引导(首次启动创建默认管理员/记录meta元数据)
  ├── chatgptInit()                            # 初始化OpenAI客户端
  ├── initPublicGroup()                        # 初始化房间0(公共大厅)+房间999(全网互通)
  ├── initAllUserList()                        # 加载所有用户(创建3个私有房间)
  ├── initAllDevList()                         # 加载所有设备(加入对应群组)
  ├── initHomepageTables()                     # 初始化首页CMS表
  │
  ├── go jsonhttp.init()                       # 启动HTTP服务器
  ├── go callWSHub.run()                       # 启动WebSocket通话Hub
  ├── go saveLog()                             # 启动日志保存
  ├── go cronGetWxToken()                      # 启动微信Token定时刷新(每小时)
  ├── go checkdeviceOnline()                   # 启动设备在线监控(每5s)
  ├── go aprs.OnLoad()                         # 启动APRS上报
  │
  ├── go findNRL()                             # 启动APRS网络服务器发现(每60s)
  ├── go startPlatformServerSync()             # 启动官网平台服务器同步
  │
  ├── udpServer()                              # 启动UDP服务(主循环阻塞)
  └── signal.Notify()                          # 监听SIGINT/SIGTERM优雅关闭
```

### config.go - 配置管理

**核心变量**:
| 变量 | 类型 | 说明 |
|------|------|------|
| `conf` | `*config` | 全局配置单例 |
| `db` | `*sql.DB` | SQLite数据库句柄 |
| `PlatformList` | `[]Platformitem` | 互联服务器列表(APRS发现+官网同步) |

**核心结构体 - `config`**:

```go
type config struct {
    System      SystemConfig     // 端口(60050)、日志路径、License路径、DB/IP文件路径
    Web         WebConfig        // 静态文件路径、HTTP端口(9000)、TokenKey、ICP备案号、SSL证书
    PlatformList []Platformitem  // 配置文件中的平台服务器列表(初始值)
    SystemInfo   SystemInfo      // 平台名称(火链)、简称、LogoURL、语言
    OpenAI      OpenAIConfig     // BaseURL、APIKEY、Engine(Azure兼容)
    APRS        APRSConfig       // 服务器地址、自地址、呼号、坐标、高度
    WeiXin      WeiXinConfig     // 公众号/小程序AppID/AppSecret、模板消息ID、菜单配置
    Billing     BillingConfig    // 启用开关、过期重检间隔、微信支付配置
}
```

**`config.init()`**:
1. 获取可执行文件所在目录
2. 读取 `udphub.yaml` (或 `-c` 指定的路径)
3. 解析YAML到结构体
4. 支持 `-o json` / `-o yaml` 输出解析结果并退出
5. 将配置中的 `PlatformList` 赋值给全局变量

**`execDDL()`** - 全量建表:
- 每次启动无条件执行，幂等（CREATE TABLE IF NOT EXISTS 等）
- schema 的唯一来源（仓库不再包含 `db/sqlite.sql` / `db/update.sql`）

**`updatedb()`** - 增量Schema迁移系统:
- 通过 `schema_migrations` 表追踪已执行的DDL
- 支持的迁移操作: `ALTER TABLE ADD COLUMN`、`CREATE TABLE`、`DELETE`、`DROP INDEX`、`CREATE UNIQUE INDEX`
- 每条语句仅执行一次（如：`users.must_change_pwd` 独立列、存量 `routes='MUST_CHANGE_PWD'` 数据迁移）

**`ensureBootstrap()`** - 启动引导:
- 依赖 `meta` 表（key/value）记录 `bootstrapped_at`、`default_admin_id`
- 首次启动且系统无管理员时创建默认管理员（呼号默认 `NOCALL`，可配置），随机密码打印到 stdout，带强制改密标记，同时写入首条操作日志

---

## NRL2 协议详解

### 包结构 (48字节固定头部 + 可变数据)

```
  0                   1                   2                   3
  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                    Version (4B) "NRL2"                         |  0-3
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |         Length (2B)           |        DMRID (3B)              |  4-8
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                      Password (11B)                            |  9-19
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |    Type (1B)  |  Status (1B)  |      Count (2B)               |  20-23
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                    CallSign (6B)                | SSID (1B)    |  24-30
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 | DevModel(1B) |            Reserved/Extended (14B)              |  31-45
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |              CRC (2B)         |       DATA (variable) ...      |  46+
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**字段详解**:

| 偏移 | 长度 | 字段 | 说明 |
|------|------|------|------|
| 0-3 | 4 | Version | 固定 `"NRL2"` 协议标识 |
| 4-5 | 2 | Length | 报文总长度(头部+数据)，大端序 uint16 |
| 6-8 | 3 | DMRID | 设备DMR标识(3字节) |
| 9-19 | 11 | Password | 设备访问密码 |
| 20 | 1 | Type | 数据类型(见下方类型表) |
| 21 | 1 | Status | 设备状态位(保留字段) |
| 22-23 | 2 | Count | 报文计数器 |
| 24-29 | 6 | CallSign | 呼号(不足6位补0) |
| 30 | 1 | SSID | 1-99=硬件, 100-199=软件, 200-255=服务器 |
| 31 | 1 | DevModel | 设备型号(0保留,1-99=硬件,100-199=软件,200-255=服务器) |
| 32-45 | 14 | Reserved | 保留/扩展(200/255设备含OrigCallsign+OrigSSID+OrigIP) |
| 46-47 | 2 | CRC | 校验码 |
| 48+ | - | DATA | 上层协议数据 |

### 扩展内容 (DevModel=200/255 或 Type=9)

| 偏移 | 长度 | 字段 | 说明 |
|------|------|------|------|
| 32-37 | 6 | OrigCallsign | 原始呼号(服务器转发的源呼号) |
| 38 | 1 | OrigSSID | 原始SSID |
| 39-42 | 4 | OrigIP | 原始IPv4地址 |

### 包类型

| Type | 值 | 名称 | 说明 |
|------|-----|------|------|
| 0 | 0x00 | Reserved | 保留控制指令 |
| 1 | 0x01 | G.711 Voice | G.711 A-law语音, 160字节/帧, 8kHz采样率, 20ms |
| 2 | 0x02 | Heartbeat | 心跳, 每2秒一次 |
| 3 | 0x03 | Config | 设备配置(子类型1=查询,2=返回EEPROM,3=修改EEPROM,4=重启) |
| 4 | 0x04 | Reserved | 保留 |
| 5 | 0x05 | Text/Message | 文本/位置/JSON/XML/HTML/图片/视频/音频消息 |
| 6 | 0x06 | Control | 设备间控制指令 |
| 7 | 0x07 | Group/Op | 设备操作(子类型1=切换群组,2=下载群组列表) |
| 8 | 0x08 | Opus 16K | Opus编码语音, 16kHz采样率, 20ms帧 |
| 9 | 0x09 | Server Voice | 服务器互联语音(后续版本废弃,合并到Type 1) |
| 10 | 0x0A | Device Control 2 | 设备控制类型2 |
| 11 | 0x0B | AT Passthrough | AT指令透传(查询/写入) |
| 12 | 0x0C | Codec2 700C | Codec2 超低码率语音 (700 bps) |
| 13 | 0x0D | Codec2 1300 | Codec2 超低码率语音 (1300 bps) |
| 14 | 0x0E | Codec2 1600 | Codec2 超低码率语音 (1600 bps) |
| 15 | 0x0F | Codec2 2400 | Codec2 超低码率语音 (2400 bps) |
| 16 | 0x10 | Codec2 3200 | Codec2 超低码率语音 (3200 bps) |

> 注: Type 12 在早期版本曾用作 COM 串口透传，后续版本已重新分配给 Codec2 700C。COM 透传功能迁移至更高 Type 值。

### 协议版本演进

NRL2 协议在开发过程中经历了以下关键变更：

| 版本 | 变更 | 说明 |
|------|------|------|
| **v1** | G.711 A-law / μ-law 双编码 | 早期同时支持两种 PCM 压扩算法 |
| **v2** | 仅保留 A-law | 去掉 μ-law，统一编码，简化硬件实现 |
| **v3** | 语音帧 500→160 字节 | **低带宽优化**：帧大小从 500B (62.5ms) 缩减为 160B (20ms)，带宽降低 68%，延迟从 62.5ms 降至 20ms，同时与标准 VoIP G.711 帧对齐 |

### 文本消息子类型 (Type=5)

文本消息通过 `[type]` 前缀标识媒体类型:

| 前缀 | 说明 | MIME Type |
|------|------|-----------|
| `[text]` (默认) | 纯文本 | text/plain |
| `[loc]` | 位置消息(经纬度) | application/location |
| `[json]` | JSON数据 | application/json |
| `[xml]` | XML数据 | application/xml |
| `[html]` | HTML内容 | text/html |
| `[bin]` | 二进制(OSS链接) | application/octet-stream |
| `[img]` | 图片(OSS链接) | image/jpeg |
| `[video]` | 视频(OSS链接) | video/mp4 |
| `[audio]` | 音频(OSS链接) | audio/mp3 |

### G.711语音规格

```
编码格式: G.711 A-law (PCM)
采样率:   8kHz
声道数:   1 (单声道)
帧大小:   20ms (160 samples / 160字节)
码率:     64 kbps
注:       早期版本为 500字节/帧(62.5ms), v3起缩减为标准 160字节/帧
```

### Opus语音规格 (Type=8)

```
编码格式: Opus
采样率:   16kHz
声道数:   1 (单声道)
帧大小:   20ms (320 samples @16k)
码率:     32-40 kbps (VBR)
应用类型: VOIP
复杂度:   10
```

### Codec2 超低码率语音 (Type 12-16)

Codec2 是专为业余无线电数字语音设计的开源声码器，码率极低（700-3200 bps），可在极窄带宽信道（如短波 SSB、APRS 链路、LoRa 等）上传输清晰语音。

**NRL2 Type 分配**:

| Type | Codec2 模式 | 语音码率 | 每帧字节数 | 每帧PCM样本 | 帧时长 | 典型应用 |
|------|------------|---------|-----------|------------|--------|---------|
| 12 | 700C | 700 bps | 9 | 320 | 40ms | 短波/极窄带/远距离HF |
| 13 | 1300 | 1300 bps | 8 | 320 | 40ms | 窄带VHF/数传链路 |
| 14 | 1600 | 1600 bps | 8 | 320 | 40ms | 替代DMR/D-STAR窄带 |
| 15 | 2400 | 2400 bps | 6 | 160 | 20ms | 标准数字语音质量 |
| 16 | 3200 | 3200 bps | 8 | 160 | 20ms | 接近G.711质量 |

```
规格:
  编码格式: Codec2 (基于正弦模型)
  采样率:   8kHz
  声道数:   1 (单声道)
  帧时长:   20ms (2400/3200) / 40ms (700C/1300/1600)
  库依赖:   libcodec2 (CGo 原生 / Emscripten WASM)
```

**带宽对比**:

| 编码 | 码率 | 相对带宽 | 延时 |
|------|------|---------|------|
| G.711 | 64,000 bps | 100% | 20ms |
| Opus | 32,000 bps | 50% | 20ms |
| Codec2 3200 | 3,200 bps | 5% | 20ms |
| Codec2 2400 | 2,400 bps | 3.8% | 20ms |
| Codec2 1600 | 1,600 bps | 2.5% | 40ms |
| Codec2 1300 | 1,300 bps | 2.0% | 40ms |
| Codec2 700C | 700 bps | 1.1% | 40ms |

### 编解码架构 (VoiceCodec 接口)

服务端使用统一的 `VoiceCodec` 接口抽象所有语音编码，消除散落在各处的 `alaw2linear` 硬编码：

```go
type VoiceCodec interface {
    Type() byte                          // NRL2 Type 值
    DecodeToPCM(frame []byte) ([]int16, error)  // 解码为线性 PCM
    EncodeFromPCM(pcm []int16) ([]byte, error)  // PCM 编码为语音帧
    SampleRate() int                     // 采样率
    FrameSamples() int                   // 每帧 PCM 样本数
}
```

**CodecCaps 能力位图**:

```go
const (
    CodecCapG711   byte = 1 << 0  // G.711 A-law
    CodecCapOpus   byte = 1 << 1  // Opus 16kHz
    CodecCapCodec2 byte = 1 << 2  // Codec2 全系列
)
```

设备通过 NRL2 报文扩展字段 `SupportedCodecs` 位图宣告自身支持的编码能力，服务器根据目标设备的 CodecCaps 自动决策是否需要转码。

### 跨编码转码 (transcode.go)

当不同编码的设备在同一群组内通联时，服务器自动执行转码：

```
发送端语音帧 → 解码(VoiceCodec.DecodeToPCM) → 线性PCM
    → 重采样(ResamplePCM, 如需) → 编码(VoiceCodec.EncodeFromPCM) → 接收端语音帧
```

**转码矩阵**:

| 发送端 ↓ / 接收端 → | G.711 | Opus | Codec2 |
|---------------------|-------|------|--------|
| G.711 | 直通 | 8k→16k 重采样 | 8k保持, PCM→Codec2 |
| Opus | 16k→8k 重采样 | 直通 | 16k→8k 重采样 |
| Codec2 | PCM→G.711 | 8k→16k 重采样 | 同模式直通/异模式重编 |

**Intra-frame 缓存**: 同一帧数据的同 src→dst 转码只执行一次，后续接收者复用缓存结果，每帧结束时清空。

---

## HTTP API 完整参考

### 认证体系

所有管理API需在请求头携带 `X-Token: <JWT_TOKEN>`。

**响应格式**:
```json
{"code": 20000, "data": {"items": [...], "total": 100}}
{"code": 20001, "message": "错误信息"}
{"code": 50008, "message": "令牌错误"}
```

### 状态码

| code | 含义 |
|------|------|
| 20000 | 操作成功 |
| 20001 | 操作失败/参数错误/权限不足 |
| 50008 | 令牌错误(过期/格式/签名/超时) |

---

### 平台信息接口

**`GET /platform/info`**
> 获取平台基本信息
```json
{"code":20000,"data":{"name":"火链","logo_url":"","version":"","icp":"皖ICP备2022007119号","mail":"","callsign":"BA4RN","language":"zh"}}
```

**`POST /platform/list`**
> 获取互联服务器列表(含在线/峰值)
```json
// Response
{"code":20000,"data":{"items":[{"name":"nrlptt主站","host":"...","port":"60050","online":5,"total":20}...]}}
```

**`POST /platform/totalstats`**
> 获取全局统计数据
```json
{"code":20000,"data":{"dev_number":100,"online_dev_number":5,"user_number":1000,"voice_time":3600,"traffic":1048576,"packet_number":50000,"session_number":200,"msg_number":100,"lost_percent":2,"platform_dev_online":0,"platform_dev_total":0,"platform_server_total":0,"platform_box_total":0,"platform_app_total":0,"platform_mp_total":0}}
```

**`GET /health`**
> 健康检查
```json
{"status":"ok","service":"nrllink-udphub"}
```

---

### 用户认证接口

**`POST /user/login`**
> 用户登录
```json
// Request
{"username":"13900001111","password":"123456"}
// Response
{"code":20000,"data":{"token":"eyJhbG...","default_admin":false}}
```
- 支持按手机号或呼号登录
- 密码使用bcrypt加密验证
- 记录登录IP和时间
- 失败3次发送微信告警

**`GET /user/info`**
> 获取当前用户信息(含菜单路由和角色)
```
Query: ?token=<JWT>
```
```json
{"code":20000,"data":{"roles":["ham"],"name":"张三","callsign":"BH4XXX","avatar":"","introduction":"","routes":[...],"billing_enabled":false,"default_admin":false,...}}
```
> `default_admin` 字段标识当前用户是否为系统默认管理员（前端据此展示顶部黄色警告条）

**`POST /user/logout`**
> 用户登出(下线SSID=100的设备)

**`POST /user/create`**
> 管理员创建用户
```json
// Request
{"name":"张三","callsign":"BH4XXX","phone":"13900001111","password":"...","roles":"ham","dmrid":"4600001","mdcid":"ABCD"}
```
> 自动使用bcrypt加密密码, 创建用户对象, 初始化3个私有房间

**`POST /user/update`**
> 管理员更新用户
> 支持修改 MDCID(4位hex)、DMRID、角色、状态等

**`POST /user/profile/update`**
> 用户自助修改资料(DMRID、MDCID、头像、密码)

**`POST /user/update/avatar`**
> 更新用户头像URL

**`POST /user/password`**
> 用户自助修改密码

**`POST /user/alllist`**
> 列出所有用户(admin/ham角色可用，支持queryToWhere过滤器)

**`POST /user/list`**
> 列出所有用户(所有已认证用户可用)

**`POST /user/userlistbyrole`**
> 按角色查询用户列表 `?role=ham`

**`GET /user/detail`**
> 获取当前用户的全部详细信息(含设备列表、群组列表)

**`GET /user/mdcid`**
> 按MDC ID查找呼号 `?id=ABCD`

**`GET /user/dmrid`**
> 按DMR ID查找呼号 `?id=4600001`

**`POST /user/delete`**
> 管理员删除用户；默认管理员可删除（删除后清除其标记并记录操作日志），仅禁止删除系统中最后一个管理员（禁用/降级最后一个管理员同样被拦截）
> API别名: `/api/v1/user/delete`（另有 `/api/v1/user/password`、`/api/v1/user/create` 别名对应上述同名接口）

---

### 角色与路由接口

**`GET /roles/list`**
> 获取角色列表(非admin用户看不到admin角色)

**`POST /roles/create`**
> 创建新角色
```json
{"name_key":"operator","name":"操作员","description":"操作员角色","routes":"[...]"}
```
**`PUT /roles/create?key=operator`**
> 更新角色

**`DELETE /roles/create?key=operator`**
> 删除角色(不能删除admin)

**`GET /roles/routes`**
> 获取前端路由配置(原始JSON)

**`POST /roles/updateroutes`**
> 更新前端路由配置(admin only, 最少10字符)

---

### 设备管理接口

**`POST /device/db/list`**
> 查询设备列表(支持40+过滤条件, queryToWhere)
```json
// Request
{"callsign":"BH4","is_online":1,"page":1,"limit":20,"sort":"-id"}
// Response
{"code":20000,"data":{"total":50,"items":[{...}]}}
```
> 支持的过滤字段: id, callsign, ssid, group_id, status, is_online, is_deleted, name, dmrid, 等

**`POST /device/list`**
> 获取所有设备的实时在线列表(从内存devMap)
> 非admin用户看不到其他设备的DeviceParm

**`GET /device/get`**
> 获取单个设备详情 `?callsign=BH4XXX&ssid=1`

**`POST /device/mydevlist`**
> 获取当前用户自己的设备列表

**`GET /device/qth2`**
> 获取指定设备的QTH位置 `?callsignssid=BH4XXX-1`

**`GET /device/qths`**
> 获取所有设备的位置信息(新格式 `map[string]qth`)
> 旧格式 `map[string]string` 在 `/device/qth`

**`POST /device/update`**
> 更新设备信息(名称、DMRID、群组、优先级、型号、RF类型、频道名)
> 验证所有权或admin角色, 记录操作日志, 同步更新群组

**`POST /device/delete`**
> 删除设备
> 验证所有权或admin角色, 从DB和内存中删除, 从群组连接池移除

**`POST /device/changegroupnrl`**
> 切换设备所在群组
> 验证呼叫者, 从旧组移除, 加入新组

**`GET /room/list`**
> 获取当前用户的设备房间列表

---

### 设备远程控制接口

**`POST /device/at`**
> 发送AT指令到设备
```json
{"callsign":"BH4XXX","ssid":1,"atcommand":"AT+CALL"}
```
> 通过UDP发送Type=11报文, 等待200ms响应

**`POST /device/query`**
> 查询设备EEPROM参数
> 发送Type=3 Subtype=1报文, 等待300ms响应
> 返回完整的512字节EEPROM数据解析结果

**`POST /device/change`**
> 修改设备参数(表单式)
```json
{
  "callsign":"BH4XXX","ssid":1,
  "dcd_select":"2", "ptt_enable":"1", "ptt_level_reversed":"0",
  "add_tail_voice":"20", "remove_tail_voice":"50",
  "ptt_resistive":"0", "monitor":"0", "key_func":"0",
  "realy_status":"0", "allow_relay_control":"1", "voice_bitrate":"0",
  "local_ipaddr":"192.168.1.190", "gateway":"192.168.1.1",
  "netmask":"255.255.255.0", "dns_ipaddr":"114.114.114.114",
  "dest_domainname":"js.nrlptt.com",
  "newcallsignssid":"", "one_uv_power":"1", "moto_channel":"1"
}
```
> 支持的参数:
> - **dcd_select**: 0=PTT DISABLE, 1=MANUAL, 2=SQL_LO, 3=SQL_HI, 4=VOX
> - **ptt_enable**: 0=禁用PTT, 1=启用PTT
> - **ptt_level_reversed**: PTT电平反转(NRL2100=0, NRL2300=1)
> - **add_tail_voice**: 加尾音(步进5ms, 最小20=100ms)
> - **remove_tail_voice**: 消尾音(步进5ms)
> - **ptt_resistive**: PTT电阻 0=OFF 1=EN
> - **monitor**: 监听输出 0=OFF 1=EN
> - **key_func**: 自定义按键 0=Relay 1=MANUAL PTT
> - **realy_status**: 继电器掉电状态 0=断开 1=吸合
> - **allow_relay_control**: 允许继电器控制
> - **voice_bitrate**: H=原码率 M=码率/2
> - **IP参数**: local_ipaddr, gateway, netmask, dns_ipaddr (验证IPv4格式)
> - **dest_domainname**: 目标域名/IP
> - **newcallsignssid**: 修改设备呼号和SSID
> - **one_uv_power**: 内置UV模块电源开关
> - **moto_channel**: Moto 3188/3688信道选择

**`POST /device/change1w`**
> 修改1W无线模块参数
```json
{
  "callsign":"BH4XXX","ssid":1,
  "one_band":"1", "one_dtmf":"1",
  "one_recive_freq":"438.500", "one_transmit_freq":"433.500",
  "one_recive_cxcss":"88.5", "one_transmit_cxcss":"88.5",
  "one_sql_level":"3", "one_volume":"5", "one_mic_sensitivity":"4",
  "one_mic_encryption":"0"
}
```
> 参数范围: SQL 0-9, Volume 1-9, MIC灵敏度 1-8, 加密 0-8

**`POST /device/change2w`**
> 修改2W无线模块参数
```json
{
  "callsign":"BH4XXX","ssid":1,
  "two_recive_freq":"438.500", "two_transmit_freq":"433.500",
  "two_recive_cxcss":"88.5", "two_transmit_cxcss":"88.5",
  "two_volume":"5", "two_save_power":"0",
  "two_sql_level":"3", "two_mic_level":"4", "two_tot_level":"60",
  "flag1":"0", "flag2":"0"
}
```
> 参数范围: Volume 1-9, 省电 0=开 1=关, SQL 0-9, TOT (发射限时)

**`GET /device/qthmap`**
> 返回所有设备的GeoJSON地图数据

---

### 群组管理接口

**`POST /group/list`**
> 获取公共群组列表 + 当前用户的私有房间(1-3)
> 支持 queryToWhere 过滤(按group_id, name, type等)
```json
// Response
{"code":20000,"data":{"total":10,"items":[
  {"id":0,"name":"公共大厅","type":0,"online_dev_number":5,"total_dev_number":10,"callsign":"default",...}
]}}
```

**`POST /group/list/mini`**
> 获取迷你群组列表(公共 + 用户私有), 较少字段

**`POST /group/device/list`**
> 获取某群组内的设备列表(分离在线/离线)

**`GET /group/get`**
> 获取单个群组详情(含设备列表) `?group_id=0`

**`GET /group/listnrl`**
> 获取NRL格式群组列表(纯文本CSV)
```
id,name
0,公共大厅
1000,频道1
...
```

**`POST /group/create`**
> 管理员创建公共群组
```json
{
  "name":"测试群组","type":0,"password":"",
  "ower_callsign":"BH4XXX","ower_id":1,
  "allow_callsign_ssid_list":["BH4XXX-1"],
  "devlist":[1,2,3],"note":"测试"
}
```
> 群组类型: 0=公共房间, 1=中继互联, 2=设备互联, 4=数模互联, 5=俱乐部, 6=车友会, 7=会议组(启动混音), 8=私人房间

**`POST /group/update`**
> 管理员更新群组
> 支持修改名称、类型、密码、白名单、备注
> 类型从其他改为7时自动启动mixPCM混音
> 类型从7改为其他时自动停止mixPCM

**`POST /group/delete`**
> 管理员删除公共群组
> 自动停止mixPCM(如果正在运行), 从内存map中删除

---



### 站点设置接口

**GET /platform/site-settings**
> 获取全部站点可配置项（无需登录，登录页消费）
`json
{"code":20000,"data":{"items":[
  {"key":"platform_name","value":"NRL Test","updated_at":"2026-08-29 12:00:00"},
  {"key":"icp","value":"粤ICP备00000000号","updated_at":"..."},
  {"key":"tech_support","value":"技术支持：NRLLink","updated_at":"..."},
  {"key":"copyright","value":"Copyright (c) 2026 NRLLink","updated_at":"..."}
]}}
`

**POST /platform/site-settings/update**
> 更新站点设置（仅管理员）
`json
// 单个更新
{"key":"icp","value":"新备案号"}
// 批量更新
{"settings":{"icp":"新备案号","copyright":"(c) 2026 新版权"}}
`

### 服务器登记接口

**POST /server/register**
> 普通用户登记自己的服务器（需登录）
`json
{"name":"MyServer","ip_addr":"1.2.3.4","udp_port":"60051","server_type":2,"note":"备注"}
`
> 归属自动从 Token 中提取（ower_callsign/ower_id），状态默认 2（等待管理员审核）

---

### 服务器管理接口

**`POST /server/list`**
> 查询服务器列表
```json
{"code":20000,"data":{"items":[{...Server结构...}]}}
```

**`POST /server/create`**
> 管理员添加服务器
```json
{
  "name":"主服务器","server_type":0,"join_key":"...",
  "cpu_type":0,"mem_size":4096,"input_rate":100,"output_rate":100,
  "netcard":"eth0","ip_type":0,"ip_addr":"192.168.1.100",
  "udp_port":"60050","dns_name":"js.nrlptt.com",
  "status":1,"note":""
}
```
> status=1时自动Start()创建UDP连接和设备心跳

**`POST /server/update`**
> 管理员更新服务器
> status变更时自动Start()或Stop()

**`POST /server/delete`**
> 管理员删除服务器

---

### 中继频率接口

**`POST /relay/list`**
> 查询中继频率列表
```json
{"code":20000,"data":{"items":[{"id":1,"name":"南京0.7m","up_freq":"433.500","down_freq":"438.500","send_ctss":"88.5","recive_ctss":"88.5","ower_callsign":"BH4XXX","status":1,...}]}}
```

**`POST /relay/create`**
> 添加中继频率(任何已登录用户, 自动设置为当前用户呼号)

**`POST /relay/update`**
> 更新中继频率(admin或创建者本人)

**`POST /relay/delete`**
> 删除中继频率(admin only)

---

### 用户注册接口

**`POST /user/reg/create`** (multipart/form-data)
> 用户自助注册(上传执照图片)
```
FormData:
  callsign: "BH4XXX"
  name: "张三"
  phone: "13900001111"
  password: "123456"
  address: "南京市"
  mail: "test@test.com"
  license: <file>  // 操作证/电台执照图片
```
> 限制: 10MB, 自动按年月分目录存储, bcrypt加密密码
> 检查手机号唯一性

**`POST /user/reg/list`**
> 管理员查询注册列表

**`POST /user/reg/image/get`**
> 管理员查看注册上传的图片(返回Base64编码)
```json
{"path":"/path/to/license.jpg"}
// Response: {"code":20000,"message":"Image data retrieved successfully","data":"base64..."}
```

**`POST /user/reg/add`**
> 管理员审批通过: 从registers表读取→创建users记录→设置status=2
> 新用户自动获得 "ham" 角色

**`POST /user/reg/update`**
> 管理员更新注册信息(审核/拒绝)

**`POST /user/reg/delete`**
> 管理员删除注册记录

---

### 首页CMS接口

**`GET /api/homepage/sections`**
> 获取启用的首页板块(status=1, 按sort_order排序)
```json
{"code":20000,"data":{"items":[{"section_key":"hero","title":"欢迎","type":0,"content":"...","extra":"{}",...}]}}
```

**`GET /api/homepage/announcements`**
> 获取公告列表(已发布, 置顶优先)
```
Query: ?type=1&pinned=true&page=1&limit=10
```

**`GET /api/admin/homepage/sections`**
> 管理员获取所有板块(含禁用)

**`POST /api/admin/homepage/sections/update`**
> 管理员创建/更新板块(UPSERT by section_key)

**`POST /api/admin/homepage/sections/delete`**
> 管理员删除板块

**`POST /api/admin/homepage/announcements/create`**
> 管理员创建公告

**`POST /api/admin/homepage/announcements/update`**
> 管理员更新公告

**`POST /api/admin/homepage/announcements/delete`**
> 管理员删除公告

**`POST /api/admin/homepage/images/upload`** (multipart/form-data)
> 管理员上传图片(限制10MB, 支持jpg/jpeg/png/gif/webp/svg)

**`GET /api/admin/homepage/images/list`**
> 管理员查看已上传图片列表

**`POST /api/admin/homepage/images/delete`**
> 管理员删除图片

---

### 计费接口

**`GET /billing/info`**
> 获取计费状态和套餐列表
```json
{"code":20000,"data":{"enabled":false,"user":{"expire_time":"...","package_type":0},"packages":[...]}}
```

**`GET /billing/packages/list`**
> 获取套餐列表(admin可见禁用套餐)

**`POST /billing/packages/create`**
> 管理员创建套餐
```json
{"name":"月度套餐","months":1,"unit_price_cents":1000,"price_cents":1000,"status":1}
```

**`POST /billing/packages/update`**
> 管理员更新套餐

**`POST /billing/packages/delete`**
> 管理员删除套餐(status=0软删除)

**`POST /billing/order/create`**
> 创建支付订单(生成微信Native支付二维码)
```json
// Request
{"package_id":1}
// Response
{"code":20000,"data":{"out_trade_no":"NRL1xxxxx","code_url":"weixin://wxpay/bizpayurl?pr=...","amount_cents":1000,"status":"NOTPAY"}}
```
> 订单号格式: `NRL{userID}{timestamp}`

**`POST /billing/order/query`**
> 查询订单状态(自动刷新微信支付状态)
```json
{"out_trade_no":"NRL1xxxxx"}
```

**`POST /billing/wechat/notify`**
> 微信支付异步通知回调
> AEAD_AES_256_GCM解密→验签→标记已支付→延长用户过期时间

---

### 微信接口

**`POST /api/msg/weixin`**
> 微信公众号消息/事件回调(echostr验证/订阅/取消订阅/菜单点击/文本消息)

**`POST /weixinreturn/msgstatus`**
> 微信模板消息状态回执

**`POST /weixin/phonecode`**
> CRM手机绑定码校验

**`POST /weixin/mpphonecode`**
> 小程序手机绑定(校验access key+绑定码→发送成功模板消息)

**`POST /weixin/wxmsg`**
> 查询微信消息记录

**`GET /api/getwxmsg`**
> 获取微信媒体文件内容(图片/视频)
```
Query: ?media_type=image&media_id=xxx
```

**`POST /api/weixin/wxlogin/teacher`**
> 小程序code换session登录
```json
{"code":"wx_code_xxx","appid":"...","secret":"..."}
```

---

### 操作日志接口

**`POST /operatorlog/list`**
> 管理员查询操作日志
> 支持按 operator_id, event_type, daterange 过滤
```json
{"code":20000,"data":{"total":100,"items":[{"id":1,"timestamp":"2025-...","content":"BH4XXX","event_type":"更新设备成功","operator":"管理员-BA4RN","operator_id":1}]}}
```

---

### WebSocket接口

**`GET /ws`**
> 简单WebSocket回声服务(转大写)

**`GET /ws/calls`**
> WebSocket实时通话流
```
Query: ?token=<JWT>
```
> 协议消息:
> - `{"action":"set_subscriptions","room_keys":["public:0","private:BH4XXX:1"]}` 
> - `{"action":"subscribe","room_keys":["public:1000"]}`
> - `{"action":"unsubscribe","room_keys":["public:1000"]}`
> - `{"action":"ping"}`
>
> 服务端推送:
> - `{"type":"snapshot","rooms":[...],"recent_calls":[...],"subscriptions":[...],"connected_clients":5,"online_devices":10}`
> - `{"type":"room_state","room":{...}}` 
> - `{"type":"recent_calls","recent_calls":[...]}`
> - `{"type":"stats","total_subs":5,"connected_clients":3,"online_devices":10}`
>
> Binary消息: G.711音频帧(160字节/帧, 每20ms), 支持多房间订阅+客户端混音

---

### 静态文件服务

**`GET /`** 
> 静态文件服务器, 根路径映射到 `conf.Web.Path`

---

## 数据模型

### 核心结构体

#### deviceInfo (设备)

```go
type deviceInfo struct {
    ID                int           // 设备ID(自增)
    Name              string        // 设备名称
    DMRID             uint32        // DMR设备ID(3字节)
    Password          string        // 设备密码
    Gird              string        // 网格(Grid Square)
    DevType           byte          // 设备类型(0=未知,1=中继,2=热点,3=APP,4=WEB)
    DevModel          byte          // 设备型号(1-99=硬件,100-199=软件,200-255=服务器)
    VoiceServerIP     string        // 语音服务器IP
    VoiceServerPort   string        // 语音服务器端口
    CallSign          string        // 呼号
    SSID              byte          // SSID(0=保留,1-99=硬件,100-199=软件,200-255=服务器)
    Priority          int           // 优先级(默认100)
    CallSignSSID      string        // 呼号-SSID复合键
    GroupID           int           // 当前群组ID
    GroupPassword     string        // 群组密码
    QTH               string        // 地理位置
    Status            int           // 0=正常,1=禁收,2=禁发,3=禁收发,4=透明转发
    RFType            int           // RF类型(0=无射频,1=1W,2=2W,3=Moto,4=Yaesu,5=ICOM,6=其他)
    ISCerted          bool          // 是否已认证
    Traffic           int           // 总流量(字节)
    udpAddr           *net.UDPAddr  // 设备UDP地址
    udpSocket         *net.UDPConn  // UDP Socket
    CreateTime        string        // 创建时间
    UpdateTime        string        // 更新时间
    OnlineTime        string        // 上线时间
    ISOnline          bool          // 是否在线
    ChanName          []string      // 频道名称列表
    LastPacketTime    time.Time     // 最后收包时间
    AccountExpired    bool          // 账户是否过期
    VoiceTime         int           // 累计通话时间
    DeviceParm        *control      // 设备EEPROM参数(512字节)
    LastATcommand     *ATcommand    // 最近AT指令
    pcmBuf            []byte        // G.711 PCM音频缓冲区
    pcmBuffer         []int         // 线性PCM缓冲区(混音用)
    speaking          bool          // 是否正在发言
}
```

#### userinfo (用户)

```go
type userinfo struct {
    PID            string              // 用户PID
    ID             int                 // 用户ID
    Name           string              // 姓名
    CallSign       string              // 呼号(唯一)
    MDCID          string              // MDC ID(4位hex)
    DMRID          string              // DMR ID
    Gird           string              // 网格
    Phone          string              // 手机号(唯一)
    Password       string              // bcrypt密码
    Birthday       string              // 生日
    Sex            int                 // 性别
    Address        string              // 地址
    Mail           string              // 邮箱
    DevList        map[int]*deviceInfo // 设备列表
    Groups         map[int]*group      // 群组列表(1=房间1,2=房间2,3=房间3)
    Introduction   string              // 自我介绍
    Avatar         string              // 头像URL
    Roles          []string            // 角色列表
    UpdateTime     string              // 更新时间
    CreateTime     string              // 创建时间
    Routes         string              // 自定义路由
    Status         int                 // 状态(1=正常)
    LastLoginTime  string              // 最后登录时间
    LastLoginIP    string              // 最后登录IP
    ExpireTime     string              // 过期时间(计费)
    LoginErrTimes  int                 // 登录错误次数
    AlarmMsg       bool                // 是否接收告警模板消息
    NickName       string              // 昵称
    OpenID         string              // 微信OpenID
    TalkDuration   time.Duration       // 累计通话时长
    TalkTimes      int                 // 累计通话次数
}
```

#### group (群组/房间)

```go
type group struct {
    ID              int                 // 群组ID(0=大厅,999=全网互通,1-3=私有房间)
    Name            string              // 群组名称
    Type            int                 // 类型(0=公共,1=中继互联,2=设备互联,4=数模,5=俱乐部,6=车友会,7=会议,8=私人)
    AllowCALLSSIDList []string          // 白名单(呼号-SSID列表)
    DevList         []int               // 设备ID列表
    Password        string              // 群组密码
    OwerID          int                 // 所有者ID
    OwerCallsign    string              // 所有者呼号
    CreateTime      string              // 创建时间
    UpdateTime      string              // 更新时间
    Note            string              // 备注
    connPool        *currentConnPool    // 当前连接池
    devMap          map[int]*deviceInfo // 设备映射
    OnlineDevNumber int                 // 在线设备数
    TotalDevNumber  int                 // 总设备数
    Recorder        int                 // 录音标记
    Timer           *time.Timer         // 定时器
    ticker          *time.Ticker        // 混音ticker(20ms)
    mixStop         chan struct{}       // 混音停止信号
}
```

#### currentConnPool (连接池)

```go
type currentConnPool struct {
    mu                   sync.RWMutex
    UDPAddr              *net.UDPAddr         // 当前语音占用者地址
    lastVoiceTime        time.Time            // 最后语音时间
    lastOwnerPacketTime  time.Time            // 最后占有者包时间
    lastCtlTime          time.Time            // 最后控制时间
    lastPriority         int                  // 最后优先级
    devConnMap           map[string]*deviceInfo // UDP地址→设备映射
    devConnList          []*deviceInfo        // 设备列表快照
}
```

#### control (设备EEPROM)

```go
type control struct {
    DCDSelect         byte   // eeprom[0]  DCD选择
    PTTEnable         byte   // eeprom[1]  PTT启用
    PTTLevelReversed  byte   // eeprom[2]  PTT电平反转
    AddTailVoice      uint16 // eeprom[3-4] 加尾音
    RemoveTailVoice   uint16 // eeprom[5-6] 消尾音
    PTTresistive      byte   // eeprom[7]  PTT电阻
    Monitor           byte   // eeprom[8]  监听
    KeyFunc           byte   // eeprom[9]  自定义按键
    RealyStatus       byte   // eeprom[10] 继电器状态
    AllowRealyControl byte   // eeprom[11] 继电器控制
    VoiceBitrate      byte   // eeprom[12] 语音码率
    DMRID             string // eeprom[16-25] DMRID(只读)
    Password          string // eeprom[26-30] 密码(只读)
    InitSign          byte   // eeprom[31] 初始化标记
    LocalIPaddr       string // eeprom[32-35] 本地IP
    Gateway           string // eeprom[36-39] 网关
    NetMask           string // eeprom[40-43] 子网掩码
    DNSIP             string // eeprom[44-47] DNS
    DestPort          uint16 // eeprom[48-49] 目标端口
    LoaclPort         uint16 // eeprom[50-51] 本地端口
    SSID              byte   // eeprom[64] SSID
    CallSign          string // eeprom[65-71] 呼号
    DestDomainName    string // eeprom[80-127] 目标域名
    // 1W模块参数 (eeprom[128-163])
    OneGBWBand        byte   // 宽带/窄带
    OneGBWDTMF        byte   // DTMF
    OneReciveFreq     string // 接收频率
    OneTransmitFreq   string // 发射频率
    OneReciveCXCSS    string // 接收CTCSS/DCS
    OneTransmitCXCSS  string // 发射CTCSS/DCS
    OneSQLLevel       int    // SQL等级
    OneVolume         int    // 音量(1-9)
    OneMICSensitivity int    // MIC灵敏度(1-8)
    OneMICEncryption  int    // MIC加密(0-8)
    OneUVPower        byte   // UV模块电源
    MotoChannel       byte   // Moto信道
    // 2W模块参数 (eeprom[192-244])
    TwoReciveFreq     string // 接收频率
    TwoTransmitFreq   string // 发射频率
    TwoReciveCXCSS    string // 接收CTCSS/DCS
    TwoTransmitCXCSS  string // 发射CTCSS/DCS
    FLAG1             string // 标志1
    FLAG2             string // 标志2
    TwoVolume         int    // 音量(1-9)
    TwoSavePower      int    // 省电(0=开启,1=关闭)
    TwoSQLLevel       int    // SQL等级
    TwoMICLevel       int    // MIC等级
    TwoTOTLevel       int    // TOT发射限时
}
```

### 查询过滤器 (queryToWhere)

```go
type query struct {
    ID, User, Callsign, SSID, CountryName, RegionName, ISPDomain string
    AppID, GroupID, DeviceID, AreaID, QueryType, PhoneDistinct    string
    QueryString, OperatorID, Schname, Name, IP, NamePhone, Phone  string
    Date, Role, Month, Daterange, UpdateTime, FollowTime, CurrentArea string
    Area, Type, EventType, Count, Limit, Page, Sort, Status, NotStatus string
    IsOnline, IsDeleted, Path string
}
```

支持的 SQL 映射:
- `id`, `operator_id`, `current_area`, `area_id`, `is_deleted`, `phone` → 精确匹配
- `callsign`, `group_id`, `role`, `date`, `type`, `status`, `notstatus` → 条件匹配
- `daterange` → BETWEEN
- `month` → LIKE
- `update_time` → 范围查询
- `name`, `country_name`, `region_name`, `isp_domain`, `ip`, `event_type`, `schname` → LIKE 模糊匹配
- `name_phone` → (name LIKE OR phone LIKE) 联合查询
- `sort` → ORDER BY (+id/-id/+name/-name等)

---

## 数据库表结构

### devices - 设备表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增ID |
| name | TEXT | 设备名称 |
| dmrid | TEXT | DMR ID |
| callsign | TEXT | 呼号 |
| ssid | INTEGER | SSID |
| password | TEXT | 设备密码 |
| grid | TEXT | 网格 |
| dev_type | INTEGER | 设备类型(0-4) |
| dev_model | INTEGER | 设备型号(0-255) |
| group_id | INTEGER | 所在群组ID |
| status | INTEGER | 状态 |
| is_certed | BLOB | 是否认证 |
| chan_name | TEXT | 频道名(逗号分隔) |
| online_time | TEXT | 上线时间 |
| create_time | TEXT | 创建时间 |
| update_time | TEXT | 更新时间 |
| note | TEXT | 备注 |

### users - 用户表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增ID |
| name | TEXT | 姓名 |
| callsign | TEXT | 呼号(唯一索引) |
| grid | TEXT | 网格 |
| phone | TEXT | 手机号(唯一索引) |
| password | TEXT | bcrypt加密密码 |
| birthday | TEXT | 生日 |
| sex | BLOB | 性别 |
| avatar | TEXT | 头像URL |
| address | TEXT | 地址 |
| roles | TEXT | 角色(逗号分隔) |
| introduction | TEXT | 自我介绍 |
| alarm_msg | BLOB | 告警消息开关 |
| status | INTEGER | 状态(1=正常) |
| last_login_time | TEXT | 最后登录时间 |
| login_err_times | INTEGER | 登录错误次数 |
| create_time | TEXT | 创建时间 |
| update_time | TEXT | 更新时间 |
| openid | TEXT | 微信OpenID |
| nickname | TEXT | 昵称 |
| pid | TEXT | PID |
| last_login_ip | TEXT | 最后登录IP |
| expire_time | TEXT | 过期时间(计费) |
| must_change_pwd | INTEGER | 强制改密标记(1=下次登录必须修改密码) |

### public_groups - 公共群组表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 群组ID |
| name | TEXT | 群组名称(唯一索引) |
| type | INTEGER | 类型(0-8,100) |
| callsign | TEXT | 所有者呼号 |
| password | TEXT | 群组密码 |
| allow_callsign_ssid | TEXT | 白名单(逗号分隔) |
| ower_id | INTEGER | 所有者ID |
| devlist | TEXT | 设备列表(逗号分隔) |
| create_time | TEXT | 创建时间 |
| update_time | TEXT | 更新时间 |
| note | TEXT | 备注 |

### servers - 服务器表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 服务器ID |
| name | TEXT | 服务器名 |
| server_type | INTEGER | 类型 |
| join_key | TEXT | 接入密钥 |
| cpu_type | TEXT | CPU类型 |
| mem_size | TEXT | 内存大小 |
| input_rate | INTEGER | 入口带宽 |
| output_rate | INTEGER | 出口带宽 |
| netcard | TEXT | 网卡 |
| ip_type | INTEGER | IP类型(0=静态,1=动态,2=NAT) |
| ip_addr | TEXT | IP地址 |
| dns_name | TEXT | 域名 |
| udp_port | INTEGER | UDP端口 |
| ower_id | TEXT | 所有者ID |
| ower_callsign | TEXT | 所有者呼号 |
| status | INTEGER | 状态(1=启动,2=关闭) |
| create_time | TEXT | 创建时间 |
| update_time | TEXT | 更新时间 |
| note | TEXT | 备注 |

### relay - 中继频率表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 频点ID |
| name | TEXT | 频点名称 |
| up_freq | TEXT | 上行频率 |
| down_freq | TEXT | 下行频率 |
| send_ctss | TEXT | 发射CTCSS |
| recive_ctss | TEXT | 接收CTCSS |
| ower_callsign | TEXT | 创建者呼号 |
| status | INTEGER | 状态(1=启用) |
| create_time | TEXT | 创建时间 |
| update_time | TEXT | 更新时间 |
| note | TEXT | 备注 |

### billing_packages - 计费套餐表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 套餐ID |
| name | TEXT | 套餐名 |
| months | INTEGER | 月数 |
| unit_price_cents | INTEGER | 单价(分) |
| price_cents | INTEGER | 总价(分) |
| status | INTEGER | 状态(1=启用,0=禁用) |
| note | TEXT | 备注 |

### billing_orders - 计费订单表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 订单ID |
| out_trade_no | TEXT UNIQUE | 订单号 |
| user_id | INTEGER | 用户ID |
| callsign | TEXT | 呼号 |
| package_id | INTEGER | 套餐ID |
| months | INTEGER | 月数 |
| amount_cents | INTEGER | 金额(分) |
| status | TEXT | 状态(NOTPAY/SUCCESS) |
| prepay_id | TEXT | 预支付ID |
| code_url | TEXT | 二维码URL |
| transaction_id | TEXT | 微信交易号 |
| payer_openid | TEXT | 付款人OpenID |
| paid_at | TEXT | 支付时间 |
| expire_before | TEXT | 支付前过期时间 |
| expire_after | TEXT | 支付后过期时间 |
| raw_notify | TEXT | 原始回调数据 |

### roles - 角色表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 角色ID |
| name_key | TEXT | 角色键 |
| name | TEXT | 角色名 |
| description | TEXT | 描述 |
| routess | TEXT | 路由配置 |

### operator_log - 操作日志表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 日志ID |
| timestamp | TEXT | 时间戳 |
| content | TEXT | 操作内容 |
| event_type | TEXT | 事件类型 |
| operator | TEXT | 操作员 |
| operator_id | INTEGER | 操作员ID |

### registers - 注册表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 注册ID |
| callsign | TEXT UNIQUE | 呼号 |
| name | TEXT | 姓名 |
| phone | TEXT | 手机号 |
| address | TEXT | 地址 |
| mail | TEXT | 邮箱 |
| birthday | TEXT | 生日 |
| sex | INTEGER | 性别 |
| password | TEXT | 加密密码 |
| op_cert_path | TEXT | 操作证路径 |
| license_path | TEXT | 执照路径 |
| status | INTEGER | 状态(1=未审核,2=审核通过) |
| note | TEXT | 备注 |

### homepage_sections - 首页板块表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 板块ID |
| section_key | TEXT UNIQUE | 板块键(hero/features/about等) |
| title | TEXT | 标题 |
| subtitle | TEXT | 副标题 |
| content | TEXT | 内容 |
| type | INTEGER | 类型 |
| sort_order | INTEGER | 排序 |
| status | INTEGER | 状态(1=启用) |
| extra | TEXT | 扩展JSON |

### homepage_announcements - 公告表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 公告ID |
| title | TEXT | 标题 |
| summary | TEXT | 摘要 |
| content | TEXT | 内容 |
| cover_url | TEXT | 封面图URL |
| type | INTEGER | 类型 |
| is_pinned | INTEGER | 是否置顶 |
| is_published | INTEGER | 是否发布 |
| publish_time | DATETIME | 发布时间 |

### homepage_images - 图片表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 图片ID |
| filename | TEXT | 原始文件名 |
| url_path | TEXT | 访问路径 |
| file_size | INTEGER | 文件大小 |
| width | INTEGER | 宽度 |
| height | INTEGER | 高度 |
| alt_text | TEXT | ALT文本 |
| category | TEXT | 分类 |


### site_settings - 站点设置表

| 字段 | 类型 | 说明 |
|------|------|------|
| key | TEXT PK | 设置键名（platform_name/logo_url/icp/tech_support/copyright/login_slogan/contact_mail/contact_callsign/language/cs_qr_url） |
| value | TEXT NOT NULL | 设置值 |
| updated_at | TEXT | 最后更新时间 |

> 首次启动由 initSiteSettingTable() 幂等建表。读取回退 YAML 默认值，修改只改数据库。

### meta - 元数据表（引导状态）

| 字段 | 类型 | 说明 |
|------|------|------|
| key | TEXT PK | 元数据键(`bootstrapped_at` 引导完成时间 / `default_admin_id` 默认管理员用户ID) |
| value | TEXT | 元数据值 |

---

## 配置文件

### udphub.yaml 详解

```yaml
system:
  port: 60050              # UDP语音端口
  logpath: /nrllink/logs   # 日志路径
  licensepath: /nrllink/licenses  # 执照上传路径
  dbfile: /nrllink/udphub.sqlite3 # SQLite数据库文件路径
  ipfile: /nrllink/udphub.ipdb    # IP地理位置库文件路径
  calllogpath: /nrllink/calllogs  # 通话日志路径

web:
  path: /nrllink/www        # 前端静态文件路径
  port: 9000                # HTTP端口(内部,对外通过nginx 443)
  tokenkey: "nrl1234"       # JWT签名密钥(重启不掉线)
  icp: "皖ICP备2022007119号" # ICP备案号
  sslcrt: ""                # SSL证书路径(可选直连HTTPS)
  sslkey: ""                # SSL密钥路径

systeminfo:
  platformname: "NRL无线电网络互联系统"  # 平台全名
  nameshorthand: "火链"                  # 平台简称
  logourl: ""                             # Logo URL
  language: "zh"                          # 默认语言

openai:
  baseurl: ""       # OpenAI Base URL(空=默认, 填写=自定义/Azure)
  apikey: ""        # API Key
  engine: ""        # Azure引擎名(空=标准API, 填写=Azure模式)

aprs:
  serverhost: asia.aprs2.net    # APRS-IS服务器
  serverport: "14580"           # APRS-IS端口
  selfaddress: js.nrlptt.com   # 本服务器公网地址
  selfport: "60050"             # 本服务器语音端口
  callsign: "BA4RN"             # 运营者呼号
  ssid: "10"                    # APRS SSID
  passcode: 0                   # APRS认证码(0=自动计算)
  latitude: 32.0615513          # 纬度(十进制度)
  longitude: 118.2511           # 经度(十进制度)
  altitude: "50"                # 高度

weixin:
  mpappid: ""              # 公众号AppID
  mpappsecret: ""          # 公众号AppSecret
  phonecodeurl: ""         # 手机绑定URL
  avatarurl: ""            # 默认头像URL
  servertoken: ""          # 服务器Token
  serverurl: ""            # 服务器URL
  accesstoken: ""          # AccessToken(自动获取)
  accesskey: ""            # AccessKey
  appid: ""                # 小程序AppID
  appsecret: ""            # 小程序AppSecret
  encodingaeskey: ""       # 消息加解密密钥
  weixinwelcome: "欢迎关注" # 欢迎语
  defaultkeywords: "默认回复" # 默认回复
  weixinmenu: ""           # 自定义菜单JSON
  alarmmodeid: ""          # 告警模板ID
  # 模板消息ID(自动获取)
  typephonecodeid: ""      # 绑定码模板ID
  typeloginsuccessid: ""   # 登录成功模板ID
  typeloginfailid: ""      # 登录失败模板ID
  # 关键词回复映射
  clickeventmap: {}        # 菜单事件→回复
  keywordsmap: {}          # 关键词→回复

billing:
  enabled: false                        # 计费开关
  accountexpirerechecksecs: 300         # 过期重检间隔(秒)
  packageunitpricecents: 0              # 默认单价
  notifyurl: ""                         # 支付回调URL
  wechatpay:
    mchid: ""         # 商户号
    mchcertserial: "" # 商户证书序列号
    mchapiv3key: ""   # APIv3密钥
    mchprivatekeypath: "" # 商户私钥路径

bootstrap:                        # 首次启动引导配置(可选)
  defaultadmincallsign: "NOCALL"  # 首次启动创建的默认管理员呼号(默认NOCALL)

platformlist:    # 配置文件中的初始服务器列表(启动后会被APRS/官网同步覆盖)
  - name: nrlptt主站
    host: ...
    port: "60050"
  - name: 北京阳光
    host: ...
    port: "60050"
  # ...
```

---

## 业务流程

### UDP语音转发流程

```
1. 设备发送UDP包(格式: NRL2协议)
   ↓
2. udpProcess() 读取包 → newNRL21packet() 解码
   ↓
3. 获取或创建deviceInfo(自动注册未知设备)
   ↓
4. 检查账户是否过期(计费模块)
   ↓
5. 调用 NRL21parser() 按类型分发:
   ├── Type 1 (G.711语音):
   │   ├── 检查设备Status(禁发则丢弃)
   │   ├── 记录通话日志到logbuffer
   │   ├── 更新语音时长
   │   ├── forwardVoice(): 1台=回声/2台=全双工/3+=抢占
   │   ├── FullNetOutput(): 999房间→所有平台服务器
   │   └── callWSHub.publishVoiceFrame(): 推送WebSocket
   │
   ├── Type 2 (心跳):
   │   ├── 更新设备在线状态
   │   ├── 查询IP地理位置(getQTH)
   │   ├── 首次上线自动查询设备参数
   │   ├── 平台服务器转发: 替换源呼号+SSID+IP
   │   └── 回发心跳包
   │
   ├── Type 3 (配置):
   │   └── decodeControlPacket(): 解析512字节EEPROM
   │
   ├── Type 5 (文本):
   │   └── forwardMsg(): 转发给群组内所有设备
   │
   ├── Type 6/10 (控制):
   │   └── forwardCtl(): 转发控制指令
   │
   ├── Type 7 (群组操作):
   │   ├── subtype 1: changeDevGroup()
   │   └── subtype 2: getGroupListForDevice() 下载CSV列表
   │
   ├── Type 8 (Opus):
   │   └── 同Type 1处理
   │
   ├── Type 9 (服务器互联语音):
   │   └── forwardServerVoice(): 替换200/255设备协议头
   │
   ├── Type 11 (AT透传):
   │   └── decodeATPacket(): 解析AT命令响应
   │
   └── Type 12 (COM透传):
       └── forwardCOM(): 转发给非200设备
```

### 语音抢占算法 (Talk-Grab)

```
发言权获取:
  1. 当前无发言者 → 立即获取
  2. 当前有发言者 → 比较优先级
     - 自己优先级 > 当前发言者 → 抢占成功
     - 自己优先级 = 当前发言者 → 比较最后Owner包时间(>500ms=抢占)

发言权释放:
  - PTT松开后200ms自动释放
  - 语音停止1秒后重置抢占权重

特殊处理:
  - echo模式(1设备): 直接回声(含G.711音频,用于自检)
  - 全双工模式(2设备): 互传对方音频
  - 会议模式(3+设备/Type=7房间): 标准抢占
```

### PCM混音引擎 (Type 7 会议组)

```
mixPCM() 每20ms执行:
  1. 收集所有设备的pcmBuf(160线性PCM样本)
  2. 跳过缓冲区不足的设备
  3. 说话人数=1: 直通(Bypass), 音质无损
  4. 说话人数=2: 
     - A听B的原始G.711
     - B听A的原始G.711
     - 其他听众=混合音频
  5. 说话人数≥3: 标准混音
     - 计算全局混合音频
     - 每位说话者: 全局混合 - 自身 = 自己听到的(不含自己)
     - 非说话者: 全局混合
  6. 限幅(±32767) → Linear2Alaw → 广播
```

### 服务器互联 (全网互通)

```
999房间跨服务器转发:
  1. NRL21replace200and255dev() 替换协议头
     - Type字段不变
     - CallSign+SSID替换为服务器代理呼号(SSID=200)
     - 扩展位写入原始呼号/SSID/IP
  2. 遍历 PlatformList (APRS发现+官网同步)
  3. FullNetOutput() → 对每个平台服务器发送UDP
  4. 接收端: NRL21parser()识别Type 1/8/9
     看到 dev.SSID==200 → 从扩展位提取原始呼号转发

平台发现:
  1. APRS机制: 
     - 上报位置到 APRS-IS (TCP 14580)
     - 定时查询 aprs.tv API 获取活跃服务器
  2. 官网机制:
     - POST nrlptt.com/api/platform-servers/report (每3分钟上报)
     - GET nrlptt.com/api/platform-servers (每5分钟同步)

自身过滤: SelfAddress=自身公网IP, 发现后不给自己发
去重: 按 host:port 去重
```

### 设备在线监控

```
checkdeviceOnline() 每5秒:
  1. 遍历所有公共群组和私有房间的连接池
  2. 检查每个设备的LastPacketTime
  3. >6秒无心跳 → 从连接池移除(removeDevice)
  4. 重建devConnList(rebuildListLocked)
  5. 更新在线设备计数(OnlineDevNumber/onlineDevMap)
  6. 触发平台服务器状态上报
```

---

## 第三方集成

### APRS

```
APRS位置上报流程:
  TCP连接 → 呼号认证 → 立即上报位置+状态
  → 1分钟后再次上报 → 5分钟后再次上报
  → 每10分钟保活

APRS服务器发现流程:
  每60秒 → GET aprs.tv/api/findnrl?dest=NRLSRV&duration=60
  → 解析活跃服务器列表 → 去重 → 过滤自身 → 更新PlatformList

APRS统计查询:
  每60秒 → GET aprs.tv/api/findnrltotal?duration=60
  → 获取全网 NRLSRV/NRLBOX/NRLAPP/NRLMP 总数
```

### 微信集成

```
微信公众号:
  - echostr URL验证
  - 关注/取消关注事件处理
  - 自定义菜单(POST微信API)
  - 菜单CLICK事件(查通话统计/获取绑定码)
  - 文本消息(GPT自动回复/关键词匹配)
  - 模板消息(登录成功/失败/绑定确认)
  - AccessToken自动刷新(每60分钟)

小程序:
  - jscode2session换取OpenID/SessionKey
  - 手机号绑定(获取绑定码→模板消息通知)
  - HAM用户登录(unionid匹配)
```

### OpenAI / ChatGPT

```
ChatGPT集成:
  - 支持标准OpenAI API和Azure API
  - 用户级上下文管理(sync.Map, 最多5轮对话)
  - 触发关键词: "复位"/"结束会话"/"重新开始"/"reset" → 清空上下文
  - 首次对话注入预设帮助系统提示词
  - 在微信公众号文本消息中触发
```

---

## 部署

### 数据库初始化与启动顺序

启动时数据库处理按以下顺序执行：

```
getDB()              # 打开数据库（父目录/文件不存在时自动创建；支持 NRL_DBFILE 环境变量）
  → execDDL()        # 无条件、幂等、全量建表（schema 唯一来源）
  → updatedb()       # 增量迁移（每条语句仅执行一次）
  → ensureBootstrap()# 启动引导（首次启动创建默认管理员）
```

- **首次部署自动从零建库**：发行包不再自带 `udphub.sqlite3`；`DBfile` 父目录不存在时自动创建；
  数据库文件不存在时日志打印"检测到首次部署，将自动创建数据库"。
- 连接串追加 `_busy_timeout=5000`；连接池上限 1；未启用 WAL。
- 环境变量 `NRL_DBFILE` 可覆盖配置文件中的 `DBfile`：

```bash
docker run -d \
  -p 80:80 -p 60050:60050/udp \
  -e NRL_DBFILE=/nrllink/data/udphub.sqlite3 \
  -v /data:/nrllink/data -v /conf:/nrllink/conf \
  78ham/nrllink:latest
```

### 首次启动与默认管理员流程

1. 启动服务，关注启动日志中打印的 **16 位随机管理员密码**（容器部署用 `docker logs <容器>` 获取）
2. 用呼号 `NOCALL`（或 `Bootstrap.DefaultAdminCallsign` 配置的值）+ 随机密码登录 Web 管理后台
3. 系统强制要求修改密码（`must_change_pwd` 标记）
4. 在【用户管理】页面创建自己的管理员账号
5. 删除默认管理员账号（顶部黄色警告条随之消失；删除动作会清除默认标记并记录操作日志）

> 存量已有管理员的数据库升级后**不会**被创建默认管理员（仅补写 `meta` 引导元数据）。
> 默认管理员登录期间前端顶部常驻黄色警告条，不可关闭，删除默认账号后消失。
> 行为变化提示：`must_change_pwd` 此前因查询遗漏实际恒为 false（失效），本次修复后真实生效，
> 存量带旧标记（`routes='MUST_CHANGE_PWD'`）的用户升级后将被真正强制改密。

### DBfile 路径建议（重要）

- 建议将 `DBfile` 配置到**持久卷内**，如 `/nrllink/data/udphub.sqlite3`。
- 仓库默认配置的路径 `/nrllink/udphub.sqlite3` **不在**常见 Docker volume 挂载点（`/nrllink/data`）内，
  换容器/重建容器会**丢失整个数据库**。请修改 `udphub.yaml` 中的 `dbfile`，或通过 `NRL_DBFILE` 环境变量指定到持久卷。
- 建议定期备份该数据库文件。

### 数据库损坏处理策略（坏库策略）

- 打开数据库后执行 `PRAGMA integrity_check`，数据库文件损坏时**明确报错退出**，
  **不会自动删除或重建**（防数据丢失，需运维介入）。
- 手工补救示例（如修复存量强制改密旧标记）：

```bash
sqlite3 udphub.sqlite3 "UPDATE users SET must_change_pwd=1, routes='' WHERE routes='MUST_CHANGE_PWD';"
```

- 确认无法修复时，可手工移除损坏文件后重启，服务将按首次部署流程从零建库（原数据丢失，需从备份恢复）。


### 端口说明

| 端口 | 协议 | 用途 |
|------|------|------|
| 80 | TCP | Web管理后台(内部, 经nginx代理) |
| 443 | TCP | Web访问入口(nginx HTTPS反代→80) |
| 60050 | UDP | 设备语音/控制/心跳信令 |

### nginx 反向代理配置示例

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### 多平台构建 (GoReleaser)

```bash
goreleaser release --snapshot --clean
```

构建目标:
- Linux amd64/arm64
- Windows amd64/arm64
- macOS amd64/arm64 (Darwin)
- Docker镜像: `ghcr.io/78ham/nrllink`
- 系统包: deb/rpm/apk

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 语言 | Go 1.24+ |
| Web框架 | gorilla/mux |
| WebSocket | gorilla/websocket |
| 数据库 | SQLite3 (mattn/go-sqlite3) |
| 认证 | JWT (golang-jwt/jwt/v5) |
| 密码 | bcrypt (golang.org/x/crypto) |
| 配置 | YAML (gopkg.in/yaml.v3) |
| JSON | json-iterator (高性能) |
| IP地理 | ipdb-go (ipip.net) |
| AI | go-openai |
| 容器化 | Docker + Alpine |
| 打包 | GoReleaser (多平台+系统包) |

---

## 服务器列表

| 用途 | 地址 |
|------|------|
| Web 管理 | https://nrlptt.com |
| 语音端口 | UDP 60050 |
| APRS 位置 | 搜索 NRLSRV |

---

## 相关项目

| 项目 | 说明 |
|------|------|
| nrllink-web-78ham | Vue3 Web管理前端 |
| 78ham-Android | Android客户端(73HAM) |
| 78ham-Desktop | 桌面客户端 |
| 78ham-ardptt | Arduino PTT控制器 |

## 许可证

MIT License

Copyright (c) 2024 BH4RPN
