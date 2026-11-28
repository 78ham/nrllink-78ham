# 插件管理 - 前端对接文档

> 适用：Vue3 管理后台 | 基础路径：`/api/v1` | 鉴权：请求头 `x-token`（登录接口获得）| 权限：**admin** 角色
>
> 响应统一格式：`{"code": 20000, "data": {...}}`；`code != 20000` 即失败，`message` 在 `data.message` 或顶层 `data` 中（与现有接口风格一致）。

---

## 1. 接口清单

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/plugins` | 插件列表（含运行状态） |
| POST | `/api/v1/plugins/enable` | 启用插件 |
| POST | `/api/v1/plugins/disable` | 禁用插件 |
| GET | `/api/v1/plugins/config?name=<name>` | 读取插件配置 |
| POST | `/api/v1/plugins/config/update` | 保存插件配置（运行中插件自动重启生效） |

## 2. 接口详情

### 2.1 插件列表

```
GET /api/v1/plugins
Header: x-token: <token>
```

响应：

```json
{
  "code": 20000,
  "data": {
    "total": 2,
    "items": [
      {
        "name": "aprs",
        "description": "APRS-IS 服务器定位上报与在线统计广播",
        "enabled": true,
        "running": true
      },
      {
        "name": "bridge",
        "description": "外部平台语音桥接（其他 NRL 服务器/自建平台桥入）",
        "enabled": false,
        "running": false
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | string | 插件唯一标识，启停/配置都用它 |
| `description` | string | 描述（后端注册时写死） |
| `enabled` | bool | 是否启用（持久化在 plugin_status 表） |
| `running` | bool | 当前是否运行中（enabled 且加载成功） |

前端建议展示：卡片列表，`enabled` 显示开关，`running` 显示状态点（绿=运行中，灰=已停止）。

### 2.2 启用 / 禁用插件

```
POST /api/v1/plugins/enable
POST /api/v1/plugins/disable
Content-Type: application/json

{ "name": "bridge" }
```

响应：`{"code": 20000, "data": {"message": "操作成功"}}`

失败（如插件不存在）：`{"code": 20001, "data": {"message": "未知插件: xxx"}}`

> 禁用 = 注销 hook + 调用插件 OnUnload（释放连接/定时器）；启用 = 从注册表重新实例化并 OnLoad。无需重启主服务。

### 2.3 读取插件配置

```
GET /api/v1/plugins/config?name=bridge
```

响应：

```json
{
  "code": 20000,
  "data": {
    "name": "bridge",
    "config": {
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
  }
}
```

> `config` 为该插件自己的 JSON 对象，结构由插件自定义；未配置过时返回 `{}`。

### 2.4 保存插件配置

```
POST /api/v1/plugins/config/update
Content-Type: application/json

{
  "name": "bridge",
  "config": { ...完整配置对象... }
}
```

响应：`{"code": 20000, "data": {"message": "操作成功"}}`

行为说明：

- 保存后如果插件正在运行，**自动重启该插件**加载新配置（前端可提示"配置已生效"）；
- 插件未运行则仅落库，下次启用时生效；
- `config` 必须是合法 JSON 对象，否则返回参数错误。

## 3. 页面设计建议

### 3.1 插件管理页（/admin/plugins）

```
┌─────────────────────────────────────────────┐
│ 插件管理                                       │
├─────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────┐ │
│ │ ● APRS 上报          [开关: ON]          │ │
│ │   APRS-IS 服务器定位上报与在线统计广播      │ │
│ │   状态: 运行中                            │ │
│ │   [配置]                                 │ │
│ └─────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────┐ │
│ │ ○ 语音桥接           [开关: OFF]         │ │
│ │   外部平台语音桥接（其他 NRL 服务器桥入）   │ │
│ │   状态: 已停止                            │ │
│ │   [配置]                                 │ │
│ └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

- 开关切换调 enable/disable，操作后刷新列表；
- 「配置」按钮弹出 Dialog：JSON 编辑器（textarea + 格式化校验即可，后续可换 schema 表单）；
- 保存时调 config/update，成功后关闭 Dialog 并刷新列表（running 状态可能变化）。

### 3.2 桥接插件配置表单（bridge 插件的 config 结构化表单）

桥接插件的 `config.bridges` 是数组，建议渲染为可增删的表格/卡片：

| 字段 | 控件 | 校验 |
|---|---|---|
| `name` | 文本 | 必填，唯一 |
| `type` | 下拉：nrl / bm / custom | 必填，当前仅 nrl 可用 |
| `remote_host` | 文本 | 必填，IP 或域名 |
| `remote_port` | 数字 | 1-65535 |
| `local_group_id` | 下拉（群组列表接口 /api/v1/groups） | 必填 |
| `bridge_callsign` | 文本 | 3-6 位呼号字符 |
| `bridge_ssid` | 数字 | 1-255 |
| `codec` | 下拉：1=G.711 | 默认 1 |
| `tx_enabled` / `rx_enabled` | 开关 | 默认双开 |
| `max_duration_sec` | 数字 | ≥30，0=不限 |
| `hook_priority` | 数字 | 默认 50 |

## 4. 调用示例（axios 风格）

```js
// 列表
const { data } = await axios.get('/api/v1/plugins', { headers: { 'x-token': token } })
const plugins = data.data.items

// 禁用
await axios.post('/api/v1/plugins/disable', { name: 'bridge' }, { headers: { 'x-token': token } })

// 读配置
const cfg = await axios.get('/api/v1/plugins/config?name=bridge', { headers: { 'x-token': token } })
// cfg.data.data.config => { bridges: [...] }

// 存配置
await axios.post('/api/v1/plugins/config/update', {
  name: 'bridge',
  config: cfg.data.data.config   // 修改后的对象
}, { headers: { 'x-token': token } })
```

## 5. 注意事项

1. **非 admin 用户调用返回 `20001 权限不足`**，前端按现有权限路由隐藏菜单即可。
2. 插件配置为整对象覆盖式保存，编辑时先 GET 再改再 POST，不要只传增量。
3. `running` 与 `enabled` 可能短暂不一致（加载失败时 enabled=true 但 running=false），前端以 running 显示运行状态、以 enabled 显示开关。
4. 轮询建议：插件管理页打开时每 10s 刷新一次列表即可，无需高频轮询。
