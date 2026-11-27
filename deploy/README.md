# 自托管服务端部署指南（Self-Hosting Guide）

`vita-server` 是纯 Go + 嵌入式 SQLite（零 CGO）的单二进制同步服务，运行时内存占用仅约 15 MB，可在任意常见设备上稳定运行：

- **运行环境要求**：Linux（云服务器 VPS / 树莓派 / 家庭主机 / NAS）、macOS、Windows 或 Docker 环境，无需安装外部数据库。
- **数据存储**：全部数据保存在单一 SQLite WAL 数据库文件中（默认 `./data/vita-server.db`），备份只需复制该目录。

---

## 方式 A：Docker / Docker Compose（推荐 Linux 服务器与 NAS）

在项目根目录或包含 `deploy/docker-compose.yml` 的目录执行：

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

验证健康检查：`curl http://127.0.0.1:8420/healthz`

---

## 方式 B：直接运行二进制（Linux / macOS / Windows）

从 [GitHub Releases](https://github.com/ilinmu/Vita/releases) 下载对应平台压缩包（或自行编译 `CGO_ENABLED=0 go build -o vita-server ./cmd/vita-server`）：

- **Linux / macOS**：
  ```bash
  mkdir -p ./data
  chmod +x ./vita-server
  ./vita-server -addr :8420 -data ./data
  ```
- **Windows (PowerShell)**：
  ```powershell
  mkdir data
  .\vita-server.exe -addr :8420 -data .\data
  ```

---

## 方式 C：Linux `systemd` 常驻守护

1. 将 `vita-server` 放入 `/opt/vita/vita-server`，数据目录设为 `/opt/vita/data`。
2. 复制并启用 [systemd/vita-server.service.example](systemd/vita-server.service.example)：
   ```bash
   sudo cp deploy/systemd/vita-server.service.example /etc/systemd/system/vita-server.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now vita-server
   ```

---

## 方式 D：macOS `launchd` 开机自启

复制并编辑 [macos/com.ilinmu.vita-server.plist.example](macos/com.ilinmu.vita-server.plist.example) 中的路径：

```bash
cp deploy/macos/com.ilinmu.vita-server.plist.example ~/Library/LaunchAgents/com.ilinmu.vita-server.plist
launchctl load -w ~/Library/LaunchAgents/com.ilinmu.vita-server.plist
```

---

## 环境变量与安全设置

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `VITA_ADDR` | `:8420` | HTTP 监听地址 |
| `VITA_DATA_DIR` | `./data` | SQLite 数据库与 `jwt.secret` 存放目录 |
| `VITA_ALLOW_PUBLIC_REGISTER` | `true` | 设为 `false` 后关闭开放注册；**持有家庭邀请码的家人依然可以注册加入** |
| `VITA_JWT_SECRET` | 自动生成 | 若留空，首次启动会自动生成随机密钥并写入 `<data>/jwt.secret` |

### 推荐上线流程
1. 首次启动保持 `VITA_ALLOW_PUBLIC_REGISTER=true`，在第一台手机上注册主账号并记下 **6 位家庭邀请码**。
2. 将 `VITA_ALLOW_PUBLIC_REGISTER` 设为 `false` 并重启服务端，防止陌生人注册；第二台手机注册时填入该邀请码即可加入同一家庭空间。

---

## 外网访问与双地址同步策略

Vita 客户端支持同时填写两个同步地址，并在断网时离线暂存：
- **局域网主地址（LAN URL）**：例如 `http://192.168.1.50:8420` 或 `http://nas.local:8420`（在家 Wi-Fi 下 1.5 秒内优先直连）。
- **外网备用地址（Public URL）**：可选任意常见方式暴露 HTTPS 端点，出门在外自动切换：
  - **Caddy / Nginx 反向代理 + HTTPS**（有公网 IP / 云服务器 VPS 时推荐）
  - **Cloudflare Tunnel / frp**（无公网 IP 的家庭主机 / NAS 推荐）
  - **Tailscale / WireGuard 私有组网**（使用私有组网地址填入即可）
  - **仅局域网同步**：留空外网地址，外出离线记录，回家连上 Wi-Fi 后自动同步合并。
