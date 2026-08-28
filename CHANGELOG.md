# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- GitHub Actions workflow for automated builds and releases
- GoReleaser configuration for multi-platform builds

### Fixed
- 移除发行包自带 `udphub.sqlite3` 中 2022 年遗留的默认管理员账号
  （callsign=NOCALL, phone=18900000000），该账号密码无从考证，且会导致新部署
  跳过安全引导、无法获得默认管理员
- 启动时自动清除已部署旧数据库中的上述遗留管理员账号（幂等），随后若无任何
  管理员则重建新的默认管理员（随机密码输出到 stdout，首次登录强制改密）
