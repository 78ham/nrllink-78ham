# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed（变更）— 容器部署重构
- Dockerfile 改为**纯后端镜像**，不再打包 `www/`（`www/` 是开发用测试前端）；
  runtime 补装 `wget` 以支持 compose 健康检查
- 镜像内置一份**中性默认配置** `docker/udphub.default.yaml`（平台名为通用值、**APRS 默认关闭**，
  避免沿用他人呼号上报）；自定义配置挂到 `/nrllink/conf/udphub.yaml` 即可优先加载（`start.sh` 判断）
- 新增环境变量 `NRL_TOKEN_KEY` 覆盖 JWT 签名密钥（原先只能改配置文件）
- `docker-compose.yml` 重构：端口/镜像全部走环境变量（`API_BIND`/`API_PORT`/`UDP_PORT`/`NRL_IMAGE`），
  去掉自定义网络名冲突风险，上传图片独立卷 `nrllink-uploads`
- `install.sh` 重写：支持服务器一行安装（自动下载编排文件）、自动生成 `.env` 与随机 `TOKEN_KEY`、支持 `--build`
- 完整部署（前端容器 + 后端容器）以 nrllink-web 仓库为入口；本仓库 compose 用于单独跑后端
- 文档：Readme 部署章节、端口表、部署架构图 `doc/svg/06-部署架构.mmd` 同步更新

### Fixed（修复）
- **微信相关接口全部 404**：`http.go` 的 `msghttp()` 里用 `mux.NewRouter()` 注册了 7 条路由
  却从未挂载到 server 上，等于死代码；改为统一用 `http.HandleFunc` 注册，并移除不再使用的
  `gorilla/mux` 依赖（`go.mod` / `go.sum` 已 tidy）
- **首页 CMS 接口前后端路径不一致**：前端调用 `/api/homepage/*`、`/api/admin/homepage/*`，
  后端只注册了 `/homepage/*`；补齐 `/api` 与 `/api/admin` 两套别名

### Added（新增）
- GitHub Actions workflow for automated builds and releases
- GoReleaser configuration for multi-platform builds
- 首次部署自动从零建库：发行包不再自带 `.sqlite3`；`DBfile` 父目录不存在时自动创建；
  数据库文件不存在时日志打印"检测到首次部署，将自动创建数据库"
- 启动引导 `ensureBootstrap`：新增 `meta` 表（key/value），记录 `bootstrapped_at`、
  `default_admin_id`；首次启动创建默认管理员（呼号默认 `NOCALL`，可用配置项
  `Bootstrap.DefaultAdminCallsign` 覆盖），16 位随机密码打印到 stdout（容器用
  `docker logs` 查看），带强制改密标记，同时写入首条操作日志
- 支持环境变量 `NRL_DBFILE` 覆盖配置文件中的 `DBfile`
- 打开数据库后执行 `PRAGMA integrity_check`，数据库文件损坏时明确报错退出，
  不会自动删除或重建（防数据丢失，需运维介入）
- 登录响应与 `/user/info` 新增 `default_admin` 字段
- 前端：默认管理员登录时顶部常驻黄色警告条（不可关闭，删除默认账号后消失）；
  新增"用户管理"页面（列表/新建/删除）
- 新增 API 别名：`/api/v1/user/password`、`/api/v1/user/create`、`/api/v1/user/delete`

### Changed（变更）
- 数据库初始化启动顺序调整为 `getDB → execDDL（无条件、幂等、全量建表）→
  updatedb（增量迁移）→ ensureBootstrap（引导）`
- `must_change_pwd` 改为 `users` 表独立列，自动迁移存量 `routes='MUST_CHANGE_PWD'` 数据
- 删除用户保护从"任何 admin 一律不可删"改为"仅不可删除系统中最后一个管理员"；
  禁用/降级最后一个管理员同样被拦截；删除默认管理员会清除其标记并记录操作日志
- 存量已有管理员的数据库升级后**不会**被创建默认管理员（仅补写引导元数据）
- SQLite 连接串追加 `_busy_timeout=5000`；连接池上限 1（未启用 WAL）
- NRL2 协议 Type 12 由早期版本的 COM 串口透传重新分配为 Codec2 700C 语音；
  服务端删除了与之冲突的旧 `case 12`（COM 透传）分支，`forwardCOM` 已停用并保留注释说明

### Fixed（修复）
- 移除发行包自带 `udphub.sqlite3` 中 2022 年遗留的默认管理员账号
  （callsign=NOCALL, phone=18900000000），该账号密码无从考证，且会导致新部署
  跳过安全引导、无法获得默认管理员
- 启动时自动清除已部署旧数据库中的上述遗留管理员账号（幂等），随后若无任何
  管理员则重建新的默认管理员（随机密码输出到 stdout，首次登录强制改密）
- 修复 `must_change_pwd` 判定因查询遗漏字段而恒为 false（失效）的问题，现真实生效。
  **行为变化：此前该判定实际处于失效状态，本次修复后存量带旧标记
  （`routes='MUST_CHANGE_PWD'`）的用户升级后将被真正强制改密**
- 修复前端强制改密接口因参数缺失必然失败的问题

### Removed（移除）
- 仓库不再包含 `udphub.sqlite3` 与 `db/sqlite.sql`、`db/update.sql`
  （schema 唯一来源为 `init.go` 的 `execDDL()`）
- `start.sh` 不再复制自带库
