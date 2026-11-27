# Vita

> **Vita**（拉丁语：生命、体征）—— 一款极简、纯 Go 编写、离线优先的家庭成员生长曲线与发烧体温记录应用（同系列：`Via` · `Toki`）。

一套纯 Go 代码同时编译为 **iOS**、**Android**、**macOS** 客户端，并附带可常驻家里 24 小时开机 **Mac mini** 的轻量 **Go + SQLite** 同步服务端。

---

## 核心特性

- **100% 纯 Go 原生架构**：移动端基于 [`egoist/mygo`](https://github.com/egoist/mygo) GPU Native UI 与 `ui/chart` 原生图表（零 WebView / JS）；服务端基于 Go + `modernc.org/sqlite` 单二进制。
- **生长发育曲线**：
  - 内置 **身高、体重、头围（0–3岁）、自动计算 BMI**，并支持无限扩展 **自定义数值指标**（如脚长、视力等）。
  - 儿童支持一键切换 **中国国家标准（WS/T 423-2022 / WS/T 612）** 与 **WHO 国际标准**，展示 P3–P97 / P15–P85 百分位区间与 P50 中位线；成人自动展示历史变化趋势线。
- **发烧与体温管理**：
  - 按 **发烧病程** 归档，支持 24h / 3 天 / 7 天快捷时间窗。
  - 体温折线图叠加 **37.3℃ / 38.5℃ 警戒线** 与 **退烧药/物理降温时间点标注**。
  - 顶部状态条实时显示「距上次测温」与「距上次用药」时间；记住上次测温部位。
  - *注：仅作个人健康记录工具，不提供医疗诊断或用药剂量建议。*
- **离线优先 & Mac mini 双地址同步**：
  - 手机本地原子持久化存储，断网/半夜秒记；联网后双向增量合并（UUID + 软删除 + LWW）。
  - 支持同时配置 **局域网主地址**（在家 Wi-Fi 毫秒直连 Mac mini）与 **外网备用 HTTPS 地址**（如 Cloudflare Tunnel，出门自动无缝切换，不占手机 VPN 槽位）。
  - 账号密码登录 + 6 位家庭邀请码 + 一键关闭公开注册。
- **主题与导出**：浅色 / 深色双主题（默认跟随系统，一键切换夜间模式）；支持 **JSON 备份**、**UTF-8 BOM CSV 表格**、以及生长曲线/发烧曲线 **高清 PNG 导出**。

---

## 本地运行与预览

```bash
# 1. 运行全部单元测试（含 headless UI 渲染测试）
go test -v ./...

# 2. 在 macOS 桌面直接预览 Vita App
go run ./cmd/vita

# 3. 启动 Mac mini 同步服务端（默认监听 :8420）
go run ./cmd/vita-server -addr :8420 -data ./data
```

Mac mini 24/7 `launchd` 常驻与 Cloudflare Tunnel 外网穿透教程见：[deploy/macmini/README.md](deploy/macmini/README.md)。

---

## GitHub Actions 自动打包与发布

- **每次 Push / PR（[.github/workflows/ci.yml](.github/workflows/ci.yml)）**：
  - 自动执行 `go vet` 与 `go test -race`
  - 自动交叉编译 `vita-server`（macOS arm64/amd64、Linux amd64/arm64）
  - 自动打包 **Android `.apk`**（未配签名时使用 debug key，直接可装到手机上使用）
  - 自动打包 **macOS `.dmg` / `.app`**
- **打 Tag 发布（`git tag v0.1.0 && git push origin v0.1.0`，[.github/workflows/release.yml](.github/workflows/release.yml)）**：
  - 自动打包全部平台产物并创建 GitHub Release。

### 可选 Secrets 配置（用于正式签名或 TestFlight 自动上传）

- **Android 正式签名（可选）**：
  - `ANDROID_KEYSTORE_BASE64`、`ANDROID_KEY_ALIAS`、`ANDROID_KEYSTORE_PASSWORD`、`ANDROID_KEY_PASSWORD`
- **iOS 自动打包 `.ipa` 与上传 TestFlight（可选）**：
  - `IOS_CERTIFICATE_BASE64`（导出的 Apple Distribution `.p12` base64）
  - `IOS_CERTIFICATE_PASSWORD`
  - `IOS_PROVISIONING_PROFILE_BASE64`（App Store / Ad-Hoc `.mobileprovision` base64，Bundle ID 需匹配 `com.ilinmu.vita`）
  - `APPLE_TEAM_ID`
  - 若需自动推送到 TestFlight，再补充 App Store Connect API Key：`MYGO_ASC_KEY_ID`、`MYGO_ASC_ISSUER_ID`、`MYGO_ASC_KEY_BASE64`
