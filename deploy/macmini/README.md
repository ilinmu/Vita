# Mac mini 24/7 常驻部署指南

`vita-server` 是纯 Go + SQLite 单二进制服务，几乎零内存占用（~15 MB），非常适合常驻在家里 24 小时开机的 Mac mini 上。

## 1. 准备二进制

从 GitHub Actions / Releases 下载 `vita-server-darwin-arm64`（或在本机执行 `CGO_ENABLED=0 go build -o vita-server ./cmd/vita-server`），放到固定目录（例如 `~/vita/vita-server`）：

```bash
mkdir -p ~/vita/data
chmod +x ~/vita/vita-server
```

## 2. 首次启动并关闭公开注册

1. 先允许注册并启动：`~/vita/vita-server -addr :8420 -data ~/vita/data`
2. 在第一台手机上注册账号并创建家庭空间，记下 **6 位家庭邀请码**。
3. 停止服务（`Ctrl+C`），之后在环境变量中设置 `VITA_ALLOW_PUBLIC_REGISTER=false`（即使关闭公开注册，家人依然可以凭 **邀请码** 注册加入）。

## 3. 配置 macOS `launchd` 开机自启

```bash
cp deploy/macmini/com.ilinmu.vita-server.plist.example ~/Library/LaunchAgents/com.ilinmu.vita-server.plist
# 编辑 plist 中的 YOUR_USER 路径
launchctl load -w ~/Library/LaunchAgents/com.ilinmu.vita-server.plist
```

验证服务：`curl http://127.0.0.1:8420/healthz`

## 4. 手机双地址配置（在家局域网直连 + 出门外网穿透）

- **局域网地址（LAN URL）**：`http://<Mac-mini局域网IP或主机名>.local:8420`（例如 `http://192.168.1.50:8420`）
- **外网地址（Public URL，推荐 Cloudflare Tunnel）**：
  ```bash
  brew install cloudflared
  cloudflared tunnel login
  cloudflared tunnel create vita
  cloudflared tunnel route dns vita vita.yourdomain.com
  cloudflared tunnel run --url http://127.0.0.1:8420 vita
  ```
  在 Vita App 的「家庭与设置 -> 同步服务器」中同时填入局域网地址和 `https://vita.yourdomain.com`。在家时 App 优先毫秒直连 Mac mini；出门用 4G/5G 时 1.5 秒内自动无缝切换到外网域名。
