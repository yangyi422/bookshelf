# Bookshelf

自托管私人书库。第一阶段功能已实现：认证、书籍/作者/标签、上传下载、元数据、回收站、管理前端、OPDS 1.2、一致性备份、离线恢复、manifest、扫描，以及 Docker Compose + Caddy 部署。

## 本地启动

需要 Go 1.24+、Node.js 20.19+（或 22.12+）及 npm。

```bash
cp .env.example .env
# 开发环境请设置 APP_ENV=development、DATA_DIR=./data，并替换密码和 SESSION_SECRET
make dev
```

健康检查：`http://localhost:8080/api/v1/system/health`。Web 认证接口位于 `/api/v1/auth`。生产部署参见 [docs/deployment.md](docs/deployment.md)。

## 数据持久化

生产容器固定使用 `/app/data`，Compose 明确绑定 `/opt/bookshelf/data:/app/data`。数据库保存为 `library.db`，并初始化 `books`、`imports`、`cache`、`trash`、`backups`、`manifests`。服务启动时若目录不可写会拒绝运行。

首次启动且用户表为空时，必须通过 `ADMIN_USERNAME` 和强 `ADMIN_PASSWORD` 创建管理员。生产环境还要求至少 32 字符的 `SESSION_SECRET`，示例弱密码会被拒绝。

## 验证

```bash
cd backend && go test ./...
cd frontend && npm install && npm run build
```

公网部署前必须将 `PUBLIC_BASE_URL` 设为真实 HTTPS 域名、替换全部密钥，并确保 DNS 指向服务器。备份恢复说明见 [docs/backup-and-restore.md](docs/backup-and-restore.md)，OPDS 说明见 [docs/opds.md](docs/opds.md)。
