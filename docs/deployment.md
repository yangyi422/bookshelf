# 部署、升级与持久化

## 生产要求

- Linux 云服务器；
- Docker Engine 和 Docker Compose v2；
- 已解析到服务器公网 IP 的域名；
- 防火墙开放 TCP 80、443；
- 宿主机持久化目录默认 `/opt/bookshelf/data`；
- 公网部署必须使用 HTTPS。

三个容器分别承担后端 API/OPDS、前端静态文件和 Caddy HTTPS。只有 Caddy 暴露端口，后端与前端不直接发布到宿主机。

## 首次部署

```bash
git clone <repository-url> /opt/bookshelf/app
cd /opt/bookshelf/app
cp .env.example .env
```

编辑 `.env`：

- `PUBLIC_BASE_URL` 设置为真实的 `https://books.example.com`；
- 替换管理员密码、Session Secret 和 OPDS 密码；
- `DATA_DIR` 保持 `/app/data`；
- 密码和密钥不得提交到 Git。

准备数据目录。后端容器固定使用 UID/GID 10001：

```bash
sudo install -d -m 0750 -o 10001 -g 10001 /opt/bookshelf/data
docker compose --env-file .env -f deploy/docker-compose.yml config
docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

Caddy 会自动申请和续期证书。初次签发前必须确保 DNS 和 80/443 端口正确。

## 验证

```bash
curl https://books.example.com/api/v1/system/health
ADMIN_USERNAME=admin ADMIN_PASSWORD='...' \
OPDS_USERNAME=reader OPDS_PASSWORD='...' \
BOOKSHELF_URL=https://books.example.com make integration-test
```

端到端测试会登录、上传一个最小 PDF、验证元数据与 OPDS Acquisition Link，最后将测试书籍移入回收站。

## 数据持久化

Compose 明确绑定 `${DATA_HOST_PATH:-/opt/bookshelf/data}:/app/data`。数据库、书籍、导入、缓存、回收站、备份和 manifests 都位于这个宿主机目录。核心数据不使用匿名卷，也不写入 `/tmp` 或容器可写层。

Caddy 的命名卷只保存 TLS 证书和运行配置，不保存书库数据。

重建验证：

```bash
docker compose --env-file .env -f deploy/docker-compose.yml down
docker compose --env-file .env -f deploy/docker-compose.yml up -d
```

重新登录后应仍能看到原有书籍。

## 升级

```bash
BOOKSHELF_COOKIE='当前会话 Cookie' BOOKSHELF_URL=https://books.example.com make backup
git pull --ff-only
docker compose --env-file .env -f deploy/docker-compose.yml build
docker compose --env-file .env -f deploy/docker-compose.yml up -d
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

应用启动时自动执行版本化迁移。升级前必须保留可验证备份，不要手工修改 SQLite 表。

## 日志与停机

```bash
docker compose --env-file .env -f deploy/docker-compose.yml logs -f --tail=200
docker compose --env-file .env -f deploy/docker-compose.yml down
```

Compose 使用 `unless-stopped` 和日志轮转。不要使用 `down -v`，否则会删除 Caddy 证书卷；书库绑定目录不受该命令管理，但仍不建议使用破坏性参数。
