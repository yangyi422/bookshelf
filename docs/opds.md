# OPDS 1.2

OPDS 由管理员在首次初始化向导或“系统设置 → OPDS 访问”中管理，修改后下一次请求立即生效。

## 访问模式

- `disabled`：所有 `/opds*` 和 `/opensearch.xml` 路由返回 404；
- `https_only`：默认且推荐，HTTP 返回明确的 403，HTTPS 才进入 Basic Auth；
- `http_and_https`：同时允许 HTTP/HTTPS，保存前必须确认界面中的明文传输警告。

OPDS 使用独立用户名和密码，不复用 Web Cookie，账号不能登录管理后台。密码只保存 bcrypt 哈希，不通过 API 返回；新密码不得与当前管理员密码相同。封面和文件下载路由使用同一动态认证中间件，不能绕过认证。

## 公开地址

XML 中绝对链接按以下优先级生成：

1. 管理员保存的 `public_base_url`；
2. 来自可信直接代理的 `X-Forwarded-Proto` 和 `X-Forwarded-Host`；
3. 当前请求的 TLS、scheme 和 Host。

非标准端口会保留，路径会规范化以避免重复斜杠。修改公开地址后新响应立即使用新值。

应用只信任 `TRUSTED_PROXIES` 中直接来源的转发头。默认包含回环和 Docker 常用私网 CIDR，适用于同机 Caddy/Nginx 及 Compose 网络；公网直连请求伪造的 `X-Forwarded-*` 不生效。直接暴露应用端口或使用其他网络时应收紧该高级环境变量。

## 路由

- `/opds`、`/opds/recent`、`/opds/all`
- `/opds/authors`、`/opds/authors/:id`
- `/opds/tags`、`/opds/tags/:id`
- `/opds/formats`、`/opds/formats/:format`
- `/opds/search?q=`
- `/opds/books/:id`
- `/opds/books/:id/cover`
- `/opds/books/:id/files/:fileId`
- `/opensearch.xml`

本地 HTTP 测试需由管理员将模式切换为 `http_and_https` 并确认警告：

```bash
curl -u reader:独立密码 http://localhost/opds
```

生产环境推荐始终使用 `https_only`，由 Caddy/Nginx 终止 TLS并传递可信转发头。
