# 备份与恢复

## 在线备份

`POST /api/v1/system/backups` 使用 SQLite `VACUUM INTO` 创建一致性数据库快照，不直接复制活跃的 `library.db`。随后将以下内容写入 `backups/bookshelf-YYYYMMDD-HHMMSS.tar.gz`：

- `library.db` 一致性快照；
- `books/`；
- `manifests/`；
- `backup.json` 格式和内容清单。

系统计算整个归档的 SHA-256，记录文件大小，并按 `BACKUP_RETENTION_DAYS` 清理过期备份。`GET /api/v1/system/backups/:id/validate` 可重新验证归档校验和。

创建备份前建议先调用 `GET /api/v1/system/manifest` 更新全库 manifest。

## 离线恢复

恢复必须停机执行。脚本会拒绝仍被进程占用的数据库，校验可用的 `.sha256` sidecar，拒绝绝对路径、`..`、符号链接和硬链接，解压到数据目录同一文件系统，执行 SQLite `quick_check`，备份当前数据后原子切换。

```bash
docker compose --env-file .env -f deploy/docker-compose.yml down
sudo DATA_DIR=/opt/bookshelf/data CONFIRM_RESTORE=yes \
  ./scripts/restore.sh /opt/bookshelf/data/backups/bookshelf-YYYYMMDD-HHMMSS.tar.gz
docker compose --env-file .env -f deploy/docker-compose.yml up -d
```

注意：当备份文件位于当前数据目录内部时，停机前应先将 `.tar.gz` 和 `.sha256` 复制到数据目录外，再运行恢复。脚本会在数据目录同级生成 `data-pre-restore-*.tar.gz` 安全备份；确认恢复成功前不要删除它。

## 扫描

`POST /api/v1/system/scan` 默认只检查，不删除或移动未知数据。检查项包括：

- 数据库文件记录缺失；
- SHA-256 不一致；
- metadata 或封面缺失；
- 孤立书籍目录；
- 未跟踪文件；
- 无效格式；
- 回收站目录数量。

扫描报告可通过 `GET /api/v1/system/scan/status` 查看。
