# 部署、升级与持久化

## 部署结构

后端 API/OPDS、前端静态文件和 Caddy 分别运行在独立容器中，只有 Caddy 暴露 80/443。两套 Compose 都包含一次性的 `storage-init`：它使用后端镜像、以 root 运行，创建所需目录并将 `/app/data` 递归设为镜像内 `bookshelf` 用户所有；只有它以退出码 0 完成后，后端才会启动。

后端镜像最终以非 root `bookshelf` 用户运行。其 UID/GID 只由 `backend/Dockerfile` 中的 `BOOKSHELF_UID`、`BOOKSHELF_GID` ARG 维护，默认均为 `10001`。`storage-init` 通过同一镜像中的用户名解析实际身份，因此 Compose 不重复硬编码数值，也不需要用户手工 `chown`。目录权限为 `0750`，未使用 `chmod 777`。

## 开发环境

开发环境使用 Docker named volume，不沿用仓库或 WSL 中的宿主机 `data` 目录：

```bash
cp .env.example .env
docker compose --env-file .env -f deploy/docker-compose.dev.yml config
docker compose --env-file .env -f deploy/docker-compose.dev.yml up -d --build
```

开发访问地址为 `http://localhost`。开发 Compose 使用独立的 `deploy/Caddyfile.dev`，站点地址显式写为 `http://localhost`，因此不会启用自动 HTTPS，也不会发生 HTTP 到 HTTPS 跳转。生产环境继续使用原有 Caddyfile 和 HTTPS 域名配置。

Compose 将 `APP_ENV` 覆盖为 `development`、将 `DATA_DIR` 覆盖为 `/app/data`，并将应用生成链接使用的 `PUBLIC_BASE_URL` 覆盖为 `http://localhost`。named volume 名为 `bookshelf-dev_bookshelf_data`（最终名称以 Compose 项目名为准）。常规 `down` 和容器重建不会删除它；不要使用 `down -v`。

首次访问 `http://localhost` 会进入初始化向导，管理员账号和 OPDS 设置在页面中创建。OPDS 默认使用 `https_only`；本地 HTTP 测试应在系统设置中选择 `http_and_https` 并确认安全警告。所有后续修改即时生效，不需要编辑 `.env` 或重启。

## 生产环境

要求 Linux、Docker Engine、Docker Compose v2、已解析到服务器的域名，以及开放的 TCP 80/443。首次部署：

```bash
git clone <repository-url> /opt/bookshelf/app
cd /opt/bookshelf/app
cp .env.example .env
```

编辑 `.env`：

- `PUBLIC_BASE_URL` 设置为真实 HTTPS 域名；
- 替换 Session Secret；管理员和 OPDS 密码由首次初始化向导设置；
- `DATA_DIR` 保持 `/app/data`；
- `BOOKSHELF_DATA_DIR` 可省略以使用 `/opt/bookshelf/data`，也可设置为另一个绝对路径；
- 密码和密钥不得提交到 Git。

不需要预先创建数据子目录，也不需要手工修改 UID/GID 或执行 `chown`：

```bash
docker compose --env-file .env -f deploy/docker-compose.prod.yml config
docker compose --env-file .env -f deploy/docker-compose.prod.yml up -d --build
```

生产配置严格使用 `${BOOKSHELF_DATA_DIR:-/opt/bookshelf/data}:/app/data`。数据库、书籍、导入、缓存、回收站、备份和 manifests 均位于该目录；Caddy 的 named volume 只保存 TLS 证书和运行配置。

## 部署验证

以下命令以开发配置为例；生产环境将文件名替换为 `deploy/docker-compose.prod.yml`。

可用一条命令自动执行本节全部检查（会构建并重建开发后端，但不会删除 named volume）：

```bash
make docker-verify
```

确认 Compose 解析、初始化服务成功退出，且所有长期服务状态正常：

```bash
docker compose --env-file .env -f deploy/docker-compose.dev.yml config --quiet
docker compose --env-file .env -f deploy/docker-compose.dev.yml ps -a storage-init
docker compose --env-file .env -f deploy/docker-compose.dev.yml ps
```

`storage-init` 应显示 `Exited (0)`，`bookshelf` 最终应显示 `healthy`。再以主容器的非 root 用户验证目录可写：

```bash
docker compose --env-file .env -f deploy/docker-compose.dev.yml exec bookshelf sh -c \
  'test "$(id -u)" != 0 && test -w /app/data && touch /app/data/.write-test && rm /app/data/.write-test'
curl --fail http://localhost/api/v1/system/health
```

验证 named volume 在容器重建后仍保留数据：

```bash
docker compose --env-file .env -f deploy/docker-compose.dev.yml exec bookshelf \
  sh -c 'printf persistent > /app/data/.volume-persistence-test'
docker compose --env-file .env -f deploy/docker-compose.dev.yml down
docker compose --env-file .env -f deploy/docker-compose.dev.yml up -d --build
docker compose --env-file .env -f deploy/docker-compose.dev.yml exec bookshelf \
  sh -c 'test "$(cat /app/data/.volume-persistence-test)" = persistent && rm /app/data/.volume-persistence-test'
```

## 升级

```bash
BOOKSHELF_COOKIE='当前会话 Cookie' BOOKSHELF_URL=https://books.example.com make backup
git pull --ff-only
docker compose --env-file .env -f deploy/docker-compose.prod.yml up -d --build
docker compose --env-file .env -f deploy/docker-compose.prod.yml ps
```

升级启动时 `storage-init` 会再次安全地确认目录存在并修复所有权，然后应用自动执行版本化迁移。升级前必须保留可验证备份，不要手工修改 SQLite 表。

## 日志与停机

```bash
docker compose --env-file .env -f deploy/docker-compose.prod.yml logs -f --tail=200
docker compose --env-file .env -f deploy/docker-compose.prod.yml down
```

Compose 使用 `unless-stopped` 和日志轮转。不要使用 `down -v`，否则会删除 Compose 管理的 named volume。
