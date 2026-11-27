# Vita

> **Vita**（拉丁语：生命、体征）—— 一款极简、纯 Go 编写、离线优先的家庭成员生长曲线与发烧体温记录应用（同系列：`Via` · `Toki`）。

🌐 **项目主页**：https://ilinmu.github.io/Vita/ ｜ 📦 **版本下载**：[GitHub Releases](https://github.com/ilinmu/Vita/releases)

一套纯 Go 代码同时编译为 **iOS**、**Android**、**macOS** 客户端，并附带轻量的 **Go + SQLite** 自托管同步服务端，支持单二进制或 Docker 部署，可运行在 Linux 云服务器、NAS、树莓派、macOS 或 Windows 等任意常用设备上。

---

## 核心特性

- **100% 纯 Go 原生架构**：移动端基于 [`egoist/mygo`](https://github.com/egoist/mygo) GPU Native UI 与 `ui/chart` 原生图表（零 WebView / JS）；服务端基于 Go + `modernc.org/sqlite` 单二进制，无外部数据库依赖。
- **生长发育曲线**：
  - 内置 **身高、体重、头围（0–3岁）、自动计算 BMI**，并支持无限扩展 **自定义数值指标**（如脚长、视力等）。
  - 儿童支持一键切换 **中国国家标准（WS/T 423-2022 / WS/T 612）** 与 **WHO 国际标准**，展示 P3–P97 / P15–P85 百分位区间与 P50 中位线；成人自动展示历史变化趋势线。
- **发烧与体温管理**：
  - 按 **发烧病程** 归档，支持 24h / 3 天 / 7 天快捷时间窗。
  - 体温折线图叠加 **37.3℃ / 38.5℃ 警戒线** 与 **退烧药/物理降温时间点标注**。
  - 顶部状态条实时显示「距上次测温」与「距上次用药」时间；记住上次测温部位。
  - *注：仅作个人健康记录工具，不提供医疗诊断或用药剂量建议。*
- **离线优先 & 双地址增量同步**：
  - 手机本地原子持久化存储，断网/半夜秒记；联网后双向增量合并（UUID + 软删除 + LWW）。
  - 支持同时配置 **局域网主地址**（在家 Wi-Fi 优先直连家庭主机/NAS）与 **外网备用 HTTPS 地址**（反向代理或内网穿透，外出自动无缝切换）。
  - 账号密码登录 + 6 位家庭邀请码 + 一键关闭公开注册（关闭后仍凭邀请码加入）。
- **主题与导出**：浅色 / 深色双主题（默认跟随系统，一键切换夜间模式）；支持 **JSON 备份**、**UTF-8 BOM CSV 表格**、以及生长曲线/发烧曲线 **高清 PNG 导出**。

---

## 本地运行与预览

```bash
# 1. 运行全部单元测试（含 headless UI 渲染测试）
go test -v ./...

# 2. 在桌面直接预览 Vita App
go run ./cmd/vita

# 3. 启动自托管同步服务端（默认监听 :8420）
go run ./cmd/vita-server -addr :8420 -data ./data
```

完整的自托管部署指南（Docker / Docker Compose、Linux `systemd`、macOS `launchd`、Windows 与外网 HTTPS 方案）请查看：[deploy/README.md](deploy/README.md)。

---

## GitHub Actions 自动打包、Pages 与发布

- **CI（[.github/workflows/ci.yml](.github/workflows/ci.yml)）**：每次推送到 `main` 或发起 PR，自动运行 `go vet` + `go test -race`，并构建 `vita-server`（Linux / macOS / Windows）、Android `.apk` 与 macOS 应用。
- **Releases（[.github/workflows/release.yml](.github/workflows/release.yml)）**：推送 `v*` 标签（如 `git tag v0.1.0 && git push origin v0.1.0`）自动打包各平台产物并发布到 [Releases](https://github.com/ilinmu/Vita/releases)。
- **Pages（[.github/workflows/pages.yml](.github/workflows/pages.yml)）**：自动构建并发布项目介绍页至 https://ilinmu.github.io/Vita/。

### 可选 Secrets 配置（用于正式签名或 TestFlight 自动上传）

- **Android 正式签名（可选）**：
  - `ANDROID_KEYSTORE_BASE64`、`ANDROID_KEY_ALIAS`、`ANDROID_KEYSTORE_PASSWORD`、`ANDROID_KEY_PASSWORD`
- **iOS 自动打包 `.ipa` 与上传 TestFlight（可选）**：
  - `IOS_CERTIFICATE_BASE64`、`IOS_CERTIFICATE_PASSWORD`、`IOS_PROVISIONING_PROFILE_BASE64`、`APPLE_TEAM_ID`
  - 若需自动推送到 TestFlight，再补充 App Store Connect API Key：`MYGO_ASC_KEY_ID`、`MYGO_ASC_ISSUER_ID`、`MYGO_ASC_KEY_BASE64`
