# NRL Link - 通过网络连接无线电

NRL Link 是一个基于 Go 语言的 UDP 语音转发服务器，配合 NRL 系列硬件盒子和 Web 前端，实现无线电设备（模拟中继、手台、公网台等）的互联互通。

## 架构总览

```
NRL硬件盒子 ──UDP 60050──▶ Go 服务端 (nrllink-78ham)
                                │
                    ┌───────────┼───────────┐
                    ▼           ▼           ▼
              WebSocket     HTTP API    APRS/平台同步
                    │           │
                    ▼           ▼
              Vue3 前端 (nrllink-web-78ham)
```

## 服务端模块 (Go)

### 核心通信
| 模块 | 文件 | 说明 |
|------|------|------|
| **主入口** | `main.go` | 初始化配置/DB/令牌，启动 HTTP、WebSocket、日志、APRS、平台同步、UDP 服务 |
| **配置** | `config.go` | YAML 配置 (`udphub.yaml`)，包含系统/Web/平台列表/OpenAI/APRS/微信/计费；SQLite 初始化与版本迁移 |
| **NRL2 协议** | `decode.go` | 48 字节包头协议，11 种包类型：语音、心跳、配置、文本、设备控制、组加入、Opus、服务器互联、AT 透传 |
| **UDP Hub** | `udphub.go` | 核心 UDP 服务：设备管理、语音转发（会议模式/半双工/全双工）、抢占算法、服务器互联 |
| **语音编解码** | `g711.go` | G.711 A-law 编解码（查表法） |

### 实时通信
| 模块 | 文件 | 说明 |
|------|------|------|
| **WebSocket** | `websocket.go`, `calls_ws.go` | 实时通话追踪、房间状态、音频流广播、会议混音、统计广播 |
| **TCP 客户端** | `tcpclient.go` | 断线重连的 TCP 客户端（用于 APRS） |

### HTTP / REST API
| 模块 | 文件 | 说明 |
|------|------|------|
| **HTTP 服务** | `http.go` | Gin/Mux 路由、CORS、静态文件服务 |
| **路由** | `routes.go` | API 路由注册 |
| **工具函数** | `tools.go` | 响应辅助、IP 地理位置 |

### 设备管理
| 模块 | 文件 | 说明 |
|------|------|------|
| **设备 CRUD** | `device.go`, `deviceDB.go` | 设备在线/离线管理、参数查询/修改（NRL2 协议）、AT 透传、IP 参数配置 |
| **设备在线** | `onlinedev.go` | 在线设备跟踪、设备计数器 |

### 用户管理
| 模块 | 文件 | 说明 |
|------|------|------|
| **用户 CRUD** | `users.go`, `usersDB.go` | bcrypt 密码、JWT 认证、角色权限 |
| **用户信息** | `userinfo.go` | 用户详情与设备关联 |
| **注册** | `userReg.go`, `userRegDB.go` | 自助注册、执照上传、管理员审批 |

### 群组管理
| 模块 | 文件 | 说明 |
|------|------|------|
| **群组 CRUD** | `group.go`, `groupDB.go` | 公共群组（会议室），类型：公共/中继互联/设备互联/数模/俱乐部/车友会/会议/私有 |
| **会议混音** | `group.go` | mixPCM 会议音频混音 |
| **设备组管理** | 同 `group.go` | 设备与群组的成员关系 |

### 服务器与中继
| 模块 | 文件 | 说明 |
|------|------|------|
| **服务器互联** | `servers.go`, `serversDB.go` | 对等服务器配置、UDP 心跳 |
| **中继频率** | `relay.go`, `relayDB.go` | 中继频率数据库（名称、上下行频率、CTCSS） |

### 业务功能
| 模块 | 文件 | 说明 |
|------|------|------|
| **计费** | `billing.go` | 套餐管理（月数/价格）、微信支付 Native（二维码）、订单与延期 |
| **APRS** | `aprs.go`, `aprsget.go` | APRS 位置上报、NRL 服务器自动发现 |
| **平台同步** | `serverList.go` | 向中心平台上报状态、同步对等服务器列表 |

### 第三方集成
| 模块 | 文件 | 说明 |
|------|------|------|
| **微信公众号** | `weixin.go`, `weixinsdk.go` | 消息处理、菜单、二维码绑定 |
| **微信模板消息** | `weixinSendMessage.go` | 登录成功/失败、绑定确认通知 |
| **微信用户信息** | `weixinUserInfo.go` | 用户信息拉取、小程序登录 |
| **OpenAI** | `chatgpt.go` | ChatGPT 自动回复（上下文会话记录） |

### 日志
| 模块 | 文件 | 说明 |
|------|------|------|
| **通话日志** | `log.go` | 文件日志，每 10 分钟轮转 |
| **操作日志** | `operatorlog.go`, `operatorlogDB.go` | 管理员审计日志（DB 存储） |

## NRL2 协议

### 包结构
```
  0                   1                   2                   3
  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |    Version    |     Type      |            Length             |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                          Caller ID                            |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                          Group ID                             |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                          Device ID                            |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |     Flags     |    Reserved   |         Sequence Num          |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                         Timestamp                             |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                         Reserved...                           |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 包类型
| 类型 | 值 | 说明 |
|------|-----|------|
| Reserved | 0 | 保留 |
| G.711 Voice | 1 | G.711 A-law 语音 |
| Heartbeat | 2 | 心跳 |
| Config | 3 | 配置 |
| Text Message | 5 | 文本消息 |
| Device Control | 6 | 设备控制指令 |
| Group Join | 7 | 群组加入/切换 |
| Opus 16K | 8 | Opus 16KHz 语音 |
| Server Interconnect | 9 | 服务器互联数据 |
| AT Passthrough | 11 | AT 指令透传 |

## 部署

### Docker
```bash
docker pull hicaoc/nrllink:latest
docker run -d \
  -p 80:80 \
  -p 60050:60050/udp \
  -v /data:/nrllink/data \
  -v /conf:/nrllink/conf \
  hicaoc/nrllink:latest
```

### 直接运行
```bash
# 编译
make build

# 运行
./nrllink

# 或使用 systemd
cp udphub.service /usr/lib/systemd/system/
systemctl daemon-reload
systemctl enable udphub.service
systemctl start udphub.service
```

## 技术栈

- **后端**: Go 1.21+, SQLite, gorilla/mux, gorilla/websocket
- **前端**: Vue 3, Element Plus, SCSS, Vite
- **协议**: NRL2 (自定义 UDP 协议), WebSocket, APRS
- **部署**: Docker, systemd

## 服务器

| 用途 | 地址 |
|------|------|
| Web 管理 | https://nrlptt.com |
| 语音端口 | UDP 60050 |
