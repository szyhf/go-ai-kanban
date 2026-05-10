# 移动设备测试指南

本指南介绍如何从手机（iPhone/Android）访问 remote-web 前端进行 UI 测试。使用 [Tailscale](https://tailscale.com) 实现稳定的网络连接和 HTTPS 证书，使用 [Caddy](https://caddyserver.com) 作为反向代理 —— 无需自定义 IP，无需随机 URL，在任何网络环境下均可使用。

**首次搭建时间**：约 15 分钟（一次性操作）。之后只需在两个终端中各执行一条命令即可。

---

## 前提条件

### 1. 在 Mac 上安装 Tailscale

从 https://tailscale.com/download/mac 下载独立应用（推荐）。也可以从 [Mac App Store](https://apps.apple.com/app/tailscale/id1470499037) 安装。

安装后：

1. 打开 Tailscale 应用
2. 点击屏幕右上角菜单栏中的 Tailscale 图标
3. 点击 **Log in** —— 这会打开浏览器窗口进行登录
4. 登录成功后，图标变为活跃状态 —— 你已连接

> 如果你已经安装了 Tailscale，可以跳过此步骤。

### 2. 在手机上安装 Tailscale

- **iPhone**：[App Store — Tailscale](https://apps.apple.com/app/tailscale/id1470499037)
- **Android**：[Play Store — Tailscale](https://play.google.com/store/apps/details?id=com.tailscale.ipn)

使用与 Mac 上 **相同的账号** 登录。

### 3. 在 Mac 上安装 Caddy

```bash
brew install caddy
```

### 4. 验证两台设备均已连接

点击 Mac 菜单栏中的 Tailscale 图标 —— 你应该能看到你的 Mac 显示为已连接。也可以在终端中验证：

```bash
tailscale status
```

你的 Mac 和手机都应该出现在列表中：

```
100.x.x.x   johns-macbook     user@   macOS   -
100.x.x.x   iphone-john      user@   iOS     -
```

> 如果手机显示"offline"，请在手机上打开 Tailscale 应用，确保开关已打开。

### 5. 启用 MagicDNS 和 HTTPS 证书

1. 打开 https://login.tailscale.com/admin/dns
2. 滚动到 **Nameservers** 部分 —— 确保 **MagicDNS** 已启用。如果你看到"Disable MagicDNS..."按钮，说明它已经启用。
3. 滚动到页面底部，找到 **"HTTPS Certificates"** 部分
4. 如果尚未启用，点击 **"Enable HTTPS"**。如果你看到"Disable HTTPS..."按钮，说明它已经启用。

> 启用 HTTPS 意味着你的机器名称和 tailnet DNS 名称将出现在公开的证书注册表中。这是 Let's Encrypt 的工作方式，属于正常行为。

---

## 一次性配置

以下所有命令都会自动检测你的 Tailscale 主机名 —— 无需手动复制粘贴。

### 步骤 1 —— 将主机名保存到 shell 配置文件中

根据你使用的 shell 执行对应命令：

**zsh**（macOS 默认）：
```bash
echo "export TS_HOSTNAME=$(tailscale status --json | python3 -c "import sys,json; print(json.load(sys.stdin)['Self']['DNSName'].rstrip('.'))")" >> ~/.zshrc
source ~/.zshrc
```

**bash**：
```bash
echo "export TS_HOSTNAME=$(tailscale status --json | python3 -c "import sys,json; print(json.load(sys.stdin)['Self']['DNSName'].rstrip('.'))")" >> ~/.bashrc
source ~/.bashrc
```

**fish**：
```bash
set -Ux TS_HOSTNAME (tailscale status --json | python3 -c "import sys,json; print(json.load(sys.stdin)['Self']['DNSName'].rstrip('.'))") 
```

验证是否生效：
```bash
echo "Your hostname: $TS_HOSTNAME"
```

验证 DNS 解析是否正常：

```bash
ping -c 1 $TS_HOSTNAME
```

### 步骤 2 —— 生成 HTTPS 证书

```bash
tailscale cert $TS_HOSTNAME
```

这会在当前目录创建 `$TS_HOSTNAME.crt` 和 `$TS_HOSTNAME.key` 文件。这些是真正的 Let's Encrypt 证书 —— 受所有浏览器和设备信任，无需在手机上额外安装。

> 证书有效期为 90 天。到期后重新运行 `tailscale cert $TS_HOSTNAME` 即可续期。

### 步骤 3 —— 创建 Caddyfile

```bash
cat > Caddyfile << EOF
${TS_HOSTNAME}:3001 {
    tls ${TS_HOSTNAME}.crt ${TS_HOSTNAME}.key
    reverse_proxy 127.0.0.1:3000
}

${TS_HOSTNAME}:8443 {
    tls ${TS_HOSTNAME}.crt ${TS_HOSTNAME}.key
    reverse_proxy 127.0.0.1:8082
}
EOF
```

**功能说明：**
- `https://$TS_HOSTNAME:3001` → 代理到 localhost:3000 上的 remote server
- `https://$TS_HOSTNAME:8443` → 代理到 localhost:8082 上的 relay server

> 我们使用不同的端口（3001 用于应用，8443 用于 relay），以避免与你 Tailscale 主机名上的其他服务产生冲突。

### 步骤 4 —— 创建 GitHub OAuth 应用

每位开发者需要创建自己的 GitHub OAuth 应用，以便从手机登录。该应用只需要 `read:user` 和 `user:email` 权限范围 —— 不需要特殊权限。

1. 访问 https://github.com/settings/applications/new
2. 填写表单：
   - **Application name**：任意名称（例如 `vibe-kanban-mobile-yourname`）
   - **Homepage URL**：运行 `echo "https://$TS_HOSTNAME:3001"` 并粘贴输出结果
   - **Authorization callback URL**：运行 `echo "https://$TS_HOSTNAME:3001/v1/oauth/github/callback"` 并粘贴输出结果
3. 点击 **Register application**
4. 复制下一页面显示的 **Client ID**
5. 点击 **Generate a new client secret** 并立即复制（此密钥不会再次显示）
6. 将这两个值添加到你的 `.env` 文件中：
   ```bash
   # 替换为你自己的值
   GITHUB_OAUTH_CLIENT_ID=your_client_id
   GITHUB_OAUTH_CLIENT_SECRET=your_client_secret
   ```

> `.env.remote` 已在 `.gitignore` 中 —— 你的凭据仅保存在本地。如果该文件已有共享开发环境配置中的这些变量，请替换为你自己的值。

## 运行

有两种模式：**Docker 模式**（简单，无热重载）和 **开发模式**（Vite 热重载，适合前端修改）。根据你的工作流程选择合适的模式。

---

### 选项 A —— Docker 模式（简单）

前端在 Docker 内构建。没有热重载 —— 需要重启 Docker 才能看到前端的变更。适合测试后端变更或在手机上进行最终 QA。

**需要两个终端：**

```bash
# 终端 1 —— Docker 容器栈
VITE_RELAY_API_BASE_URL=https://$TS_HOSTNAME:8443 \
PUBLIC_BASE_URL=https://$TS_HOSTNAME:3001 \
pnpm remote:dev

# 终端 2 —— Caddy
caddy run --config Caddyfile
```

> 首次使用这些环境变量运行时，Docker 会使用 Tailscale URL 重新构建前端。这需要几分钟时间。后续使用相同 URL 运行时会使用缓存。

---

### 选项 B —— 开发模式（Vite 热重载）

前端通过 Vite 在 Docker 外运行，因此编辑 React 组件时可以即时热重载。Caddy 将 API 请求路由到 Docker，其他请求路由到 Vite。

**步骤 1 —— 生成 `Caddyfile.dev`：**

此文件不能直接使用 shell 变量，因此需要生成一次（如果主机名变更，需要重新运行）：

```bash
cat > Caddyfile.dev << EOF
${TS_HOSTNAME}:3001 {
    tls ${TS_HOSTNAME}.crt ${TS_HOSTNAME}.key
    handle /api/* {
        reverse_proxy 127.0.0.1:3000
    }
    handle /v1/* {
        reverse_proxy 127.0.0.1:3000
    }
    handle /shape/* {
        reverse_proxy 127.0.0.1:3000
    }
    handle {
        reverse_proxy localhost:3002 {
            header_up Host localhost:3002
        }
    }
}

${TS_HOSTNAME}:8443 {
    tls ${TS_HOSTNAME}.crt ${TS_HOSTNAME}.key
    reverse_proxy 127.0.0.1:8082
}
EOF
```

**路由规则：**
- `/api/*`、`/v1/*`、`/shape/*` → Docker remote server (`:3000`)
- 其他所有请求 → Vite 开发服务器 (`:3002`)，支持热重载
- `:8443` → Relay 服务器 (`:8082`)

**步骤 2 —— 需要四个终端：**

```bash
# 终端 1 —— Docker 后端（无需构建前端）
PUBLIC_BASE_URL=https://$TS_HOSTNAME:3001 \
pnpm remote:dev

# 终端 2 —— Vite 开发服务器（热重载）
VITE_RELAY_API_BASE_URL=https://$TS_HOSTNAME:8443 \
pnpm --filter @vibe/remote-web dev

# 终端 3 —— Caddy（开发配置）
caddy run --config Caddyfile.dev

# 终端 4（可选） —— 本地桌面客户端
VK_SHARED_API_BASE=https://$TS_HOSTNAME:3001 \
VK_SHARED_RELAY_API_BASE=https://$TS_HOSTNAME:8443 \
pnpm run dev
```

> Vite 绑定到 `localhost:3002`。`Caddyfile.dev` 使用 `localhost`（而非 `127.0.0.1`）来匹配 —— 这可以避免 macOS 上的 IPv6/IPv4 不匹配问题。

---

### 从手机访问

1. 打开 Tailscale 应用，确保已连接（开关为 ON）
2. 打开 Safari（或 Chrome），访问：`https://<your-hostname>:3001`（如果忘记了，运行 `echo "https://$TS_HOSTNAME:3001"`）
3. 使用 GitHub 登录
4. 开始使用

要回到普通的 localhost 开发模式，只需不带环境变量运行 `pnpm remote:dev` 即可 —— 无需清理。

---

## 快速参考

**Docker 模式（2 个终端）：**
```bash
# 终端 1
VITE_RELAY_API_BASE_URL=https://$TS_HOSTNAME:8443 \
PUBLIC_BASE_URL=https://$TS_HOSTNAME:3001 \
pnpm remote:dev

# 终端 2
caddy run --config Caddyfile

# 在手机上访问
echo "https://$TS_HOSTNAME:3001"
```

**开发模式（4 个终端）：**
```bash
# 终端 1 —— Docker 后端
PUBLIC_BASE_URL=https://$TS_HOSTNAME:3001 \
pnpm remote:dev

# 终端 2 —— Vite
VITE_RELAY_API_BASE_URL=https://$TS_HOSTNAME:8443 \
pnpm --filter @vibe/remote-web dev

# 终端 3 —— Caddy
caddy run --config Caddyfile.dev

# 终端 4（可选） —— 桌面客户端
VK_SHARED_API_BASE=https://$TS_HOSTNAME:3001 \
VK_SHARED_RELAY_API_BASE=https://$TS_HOSTNAME:8443 \
pnpm run dev

# 在手机上访问
echo "https://$TS_HOSTNAME:3001"
```

---

## 故障排除

| 问题 | 解决方案 |
|---|---|
| `$TS_HOSTNAME` 为空 | 重新运行：`source ~/.zshrc` 或重启终端 |
| 手机无法访问 URL | 在手机上打开 Tailscale 应用 → 确保开关为 ON。在 Mac 上运行 `tailscale status` 验证两台设备均已连接 |
| 手机显示证书警告 | 重新运行 `tailscale cert $TS_HOSTNAME` —— 证书可能已过期（有效期 90 天） |
| `tailscale cert` 报错"does not support getting TLS certs" | 在 Tailscale 管理后台启用 HTTPS 证书：https://login.tailscale.com/admin/dns → 滚动到底部"HTTPS Certificates" → 点击"Enable HTTPS" |
| `tailscale cert` 报错"invalid domain" | 确保 `$TS_HOSTNAME` 包含 tailnet 名称（例如 `johns-macbook.tail99xyz.ts.net`）。重新执行步骤 1 |
| 手机上 OAuth 重定向失败 | 运行 `echo "https://$TS_HOSTNAME:3001/v1/oauth/github/callback"` 并验证与 GitHub 设置中的回调 URL 一致 |
| 首次构建非常慢 | 正常现象 —— Docker 使用新的 `VITE_RELAY_API_BASE_URL` 重新构建前端。后续构建会使用缓存 |
| Relay 功能（终端、日志）在手机上不工作 | 检查命令中的 `VITE_RELAY_API_BASE_URL` 是否与 Caddy relay 配置块匹配（`https://$TS_HOSTNAME:8443`） |
| Caddy 要求输入密码 | 首次运行时正常 —— 它在安装本地 CA 证书。输入你的 macOS 密码即可 |
| `caddy run` 报错"address already in use" | 有另一个 Caddy 实例正在运行。终止它：`pkill caddy`，然后重试 |
| `ping $TS_HOSTNAME` 无法解析 | 在 Tailscale 管理后台启用 MagicDNS：https://login.tailscale.com/admin/dns |
| 开发模式：Vite 页面加载成功但 API 调用失败 | 确保 Docker 正在运行（`pnpm remote:dev`），并且你使用的是 `Caddyfile.dev`（而非 `Caddyfile`） |
| 开发模式：手机上热重载不工作 | Vite HMR 使用 WebSocket —— 验证 Caddy 代理到的是 `localhost:3002`（而非 `127.0.0.1:3002`）。如有需要，重新生成 `Caddyfile.dev` |
| 开发模式：手机上空白页或 502 错误 | Vite 开发服务器可能未运行。检查终端 2 中的 `pnpm --filter @vibe/remote-web dev` 是否正常运行 |
