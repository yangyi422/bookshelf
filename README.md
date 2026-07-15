# Bookshelf

自托管私人书库。第一阶段包含账号认证、书籍/作者/标签管理、EPUB/PDF 等文件上传下载、基础元数据与封面提取、阅读状态、回收站、OPDS 1.2、一致性备份、离线恢复、manifest、扫描，以及 Docker Compose + Caddy 部署。

## 单容器镜像

GHCR 单容器镜像同时包含 Go 服务和已构建的 Vue 前端。启动后直接打开根路径即可进入 Web 初始化页面：

```bash
docker run -d --name bookshelf \
  -p 8088:8080 \
  -v bookshelf_data:/app/data \
  -e SESSION_SECRET="请替换为至少32字符的随机字符串" \
  ghcr.io/yangyi422/bookshelf:latest
```

访问 `http://localhost:8088/`。首次打开根路径会自动跳转到 `/setup` 完成初始化；刷新 `/setup`、`/login`、`/books/:id` 和 `/system` 等前端路由不会返回 404。健康检查地址为 `http://localhost:8088/api/v1/system/health`。

也可使用只包含一个 `bookshelf` 服务的 Compose 配置：

```bash
SESSION_SECRET="请替换为至少32字符的随机字符串" \
docker compose -f deploy/docker-compose.simple.yml up -d
```

## Docker 本地启动

准备部署环境变量并启动：

```bash
cp .env.example .env
docker compose --env-file .env -f deploy/docker-compose.dev.yml up -d --build
```

开发环境使用纯 HTTP：

- Web：`http://localhost`
- 健康检查：`http://localhost/api/v1/system/health`
- OPDS：`http://localhost/opds`

查看状态：

```bash
docker compose --env-file .env -f deploy/docker-compose.dev.yml ps -a
```

`storage-init` 应显示 `Exited (0)`，`bookshelf`、`frontend` 和 `caddy` 应显示 `healthy`。

首次访问会自动进入 `/setup` 初始化向导。在页面中创建管理员，并选择是否启用 OPDS、访问模式、独立用户名/密码和可选公开地址。OPDS 默认启用，默认模式为推荐的 `https_only`，不提供默认密码。

## 账号

Web 管理员由首次初始化向导创建，登录后可在系统设置中修改密码。OPDS 账号与管理后台账号完全独立，不能用于网页登录。

OPDS 使用独立的 HTTP Basic Auth，不使用 Web 登录 Cookie。管理员可在“系统设置 → OPDS 访问”即时开启/关闭服务、切换 `https_only`/`http_and_https`、修改用户名、重置密码、修改公开地址并执行连通性测试，无需编辑 `.env` 或重启容器。

`http_and_https` 会明文传输凭据和书籍内容，必须在界面确认安全警告后才能保存。OPDS 密码只以安全哈希保存在数据库中，不会返回前端，也不能与管理员密码相同。

环境变量 `OPDS_ENABLED`、`OPDS_ACCESS_MODE`、`OPDS_USERNAME`、`OPDS_PASSWORD` 和 `PUBLIC_BASE_URL` 仅作为已有安装迁移或高级初始回退；数据库中的管理员设置优先。

## 数据持久化

开发 Compose 使用 Docker named volume `bookshelf-dev_bookshelf_data` 保存 `/app/data`，不绑定 WSL 项目目录。数据包括：

- SQLite 数据库 `library.db`
- `books`、`imports`、`cache`
- `trash`、`backups`、`manifests`

`docker compose down` 只移除容器和网络，重新 `up` 后数据仍然存在。不要使用 `down -v`，该参数会删除 named volume。

生产配置使用 `${BOOKSHELF_DATA_DIR:-/opt/bookshelf/data}:/app/data`。开发和生产均由一次性的 root `storage-init` 创建目录及修复所有权，主应用始终以非 root `bookshelf` 用户运行，无需用户手工 `chown`。完整说明见 [部署文档](docs/deployment.md)。

## 第一阶段验证

静态测试：

```bash
cd backend && go test ./...
cd frontend && npm test -- --run
cd frontend && npm run build
docker compose --env-file .env -f deploy/docker-compose.dev.yml config
docker compose --env-file .env -f deploy/docker-compose.prod.yml config
```

端到端测试会验证健康检查、登录、上传、列表、详情、下载、移入回收站、恢复，以及 OPDS 根目录、all 和搜索：

```bash
set -a
. ./.env
set +a
BOOKSHELF_URL=http://localhost make integration-test
```

部署及 named volume 自动验证：

```bash
make docker-verify
```

## 生产启动

设置真实 HTTPS `PUBLIC_BASE_URL` 并替换所有示例凭据后运行：

```bash
docker compose --env-file .env -f deploy/docker-compose.prod.yml up -d --build
```

生产环境要求至少 32 字符的 `SESSION_SECRET` 和有效 HTTPS 域名；管理员及 OPDS 凭据通过初始化向导设置。备份恢复说明见 [docs/backup-and-restore.md](docs/backup-and-restore.md)，OPDS 说明见 [docs/opds.md](docs/opds.md)。
