# Codex 项目提示词：自托管私人综合书库

你现在是一名资深全栈工程师和系统架构师。请在当前仓库中实现一个可部署到云服务器的自托管私人综合书库系统。

## 一、项目目标

构建一个长期可维护、数据安全、可通过公网访问的私人书库，核心能力包括：

1. 管理个人合法持有的电子书。
2. 支持 EPUB、PDF 的上传、元数据管理、下载和网页阅读。
3. 支持 MOBI、AZW3、TXT 的上传、管理和下载，第一版不要求网页阅读。
4. 提供 OPDS 1.2 服务，可供外部阅读器订阅、浏览、搜索和下载。
5. 支持用户登录、HTTPS 反向代理、Docker Compose 部署。
6. 支持阅读状态、阅读进度、标签、作者、简介、评分。
7. 支持数据备份、恢复、扫描校验和索引重建。
8. 不依赖 Calibre，不使用 Talebook 的数据库和目录结构。
9. 所有运行数据必须持久化到明确的宿主机目录，禁止使用 `/tmp` 或仅存在于容器内部的数据目录。

项目暂定名：`bookshelf`

---

## 二、技术栈

### 后端

- Go 1.24+
- Gin
- GORM
- SQLite
- SQLite 开启 WAL
- slog 结构化日志
- UUID 或 ULID 作为业务主键
- `encoding/xml` 实现 OPDS 1.2
- 使用 `golang.org/x/crypto/bcrypt` 存储密码哈希

### 前端

- Vue 3
- TypeScript
- Vite
- Pinia
- Vue Router
- Element Plus
- Axios
- epub.js
- PDF.js

### 部署

- Docker
- Docker Compose
- Caddy，作为 HTTPS 反向代理
- 单域名部署，例如 `books.example.com`
- 容器内数据目录固定为 `/app/data`
- 宿主机默认挂载目录为 `/opt/bookshelf/data`

---

## 三、执行原则

1. 先检查当前仓库结构。
2. 如果仓库为空，创建标准 monorepo。
3. 如果已有代码，保留现有可用结构并说明兼容策略。
4. 不要一次性写出大量无法验证的代码。
5. 按阶段实施，每完成一个阶段都必须：
   - 编译后端；
   - 构建前端；
   - 运行测试；
   - 启动 Docker Compose；
   - 输出本阶段完成内容；
   - 输出剩余风险和下一阶段计划。
6. 不得假装命令执行成功。
7. 所有异常必须有明确错误信息和日志。
8. 所有文件路径必须基于配置的数据目录拼接，数据库中只保存相对路径。
9. 书籍删除默认进入回收站，不直接物理删除。
10. 上传、数据库写入和正式文件移动必须保证一致性。

---

## 四、建议目录结构

```text
bookshelf/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── auth/
│   │   ├── book/
│   │   ├── author/
│   │   ├── tag/
│   │   ├── file/
│   │   ├── importjob/
│   │   ├── metadata/
│   │   ├── opds/
│   │   ├── reader/
│   │   ├── backup/
│   │   ├── scanner/
│   │   ├── storage/
│   │   ├── database/
│   │   ├── config/
│   │   └── middleware/
│   ├── migrations/
│   ├── tests/
│   ├── go.mod
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── layouts/
│   │   ├── pages/
│   │   ├── router/
│   │   ├── stores/
│   │   ├── types/
│   │   └── readers/
│   ├── package.json
│   └── Dockerfile
├── deploy/
│   ├── Caddyfile
│   └── docker-compose.yml
├── docs/
│   ├── architecture.md
│   ├── api.md
│   ├── opds.md
│   ├── backup-and-restore.md
│   └── deployment.md
├── scripts/
│   ├── backup.sh
│   ├── restore.sh
│   └── healthcheck.sh
├── .env.example
├── Makefile
└── README.md
```

---

## 五、数据目录规范

容器内数据目录：

```text
/app/data
```

宿主机默认目录：

```text
/opt/bookshelf/data
```

目录结构：

```text
data/
├── library.db
├── books/
│   └── <book-id>/
│       ├── metadata.json
│       ├── cover.jpg
│       └── files/
│           ├── <file-id>.epub
│           └── <file-id>.pdf
├── imports/
├── cache/
├── trash/
├── backups/
└── manifests/
```

要求：

- 每本书独立目录。
- 文件名使用 file ID，不使用原始书名。
- 原始文件名保存在数据库。
- 数据库只保存相对路径。
- 所有文件保存后计算 SHA-256。
- 使用同一 SHA-256 检测重复文件。
- 每本书生成一份 `metadata.json`。
- 提供全库 `manifest.json` 导出能力。
- 启动时检查目录是否可写。
- 数据目录不可写时服务必须拒绝启动。

---

## 六、核心数据模型

请使用 GORM 模型和数据库迁移实现以下实体。

### User

字段：

- id
- username
- password_hash
- display_name
- role
- enabled
- created_at
- updated_at

第一版角色：

- admin
- user

### Book

字段：

- id
- title
- subtitle
- description
- language
- publisher
- isbn
- published_at
- cover_path
- reading_status
- rating
- created_at
- updated_at
- deleted_at

阅读状态：

- unread
- reading
- finished
- paused
- abandoned

### BookFile

字段：

- id
- book_id
- format
- mime_type
- relative_path
- original_name
- file_size
- sha256
- created_at

支持格式：

- epub
- pdf
- mobi
- azw3
- txt

MIME：

- EPUB：`application/epub+zip`
- PDF：`application/pdf`
- MOBI：`application/x-mobipocket-ebook`
- AZW3：`application/vnd.amazon.ebook`
- TXT：`text/plain; charset=utf-8`

### Author

字段：

- id
- name
- sort_name
- created_at
- updated_at

### BookAuthor

字段：

- book_id
- author_id
- position

### Tag

字段：

- id
- name
- created_at

### BookTag

字段：

- book_id
- tag_id

### ReadingProgress

字段：

- user_id
- book_id
- book_file_id
- locator
- percentage
- page
- total_pages
- chapter
- updated_at

EPUB 的 locator 保存 CFI。
PDF 第一版至少保存页码、总页数和百分比。

### ReadingNote

字段：

- id
- user_id
- book_id
- book_file_id
- chapter
- locator
- selected_text
- note
- created_at
- updated_at

### ImportJob

字段：

- id
- original_name
- temp_path
- status
- error_message
- created_at
- updated_at

状态：

- pending
- processing
- success
- failed

### BackupRecord

字段：

- id
- file_path
- file_size
- checksum
- created_at

---

## 七、后端接口

所有管理 API 使用 `/api/v1` 前缀。

### 认证

```text
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/me
POST   /api/v1/auth/change-password
```

要求：

- Web 端使用 Cookie Session。
- Cookie 必须设置 HttpOnly。
- HTTPS 下设置 Secure。
- SameSite 使用 Lax。
- 登录接口需要基本限流。
- 密码使用 bcrypt。
- 第一次启动时，如果没有用户，通过环境变量创建管理员：
  - `ADMIN_USERNAME`
  - `ADMIN_PASSWORD`
- 不允许使用默认弱密码。

### 书籍

```text
GET    /api/v1/books
POST   /api/v1/books
GET    /api/v1/books/:id
PUT    /api/v1/books/:id
DELETE /api/v1/books/:id
POST   /api/v1/books/:id/restore
POST   /api/v1/books/:id/cover
GET    /api/v1/books/:id/files
POST   /api/v1/books/:id/files
DELETE /api/v1/books/:id/files/:fileId
GET    /api/v1/books/:id/files/:fileId/download
```

列表支持：

- keyword
- author_id
- tag_id
- format
- reading_status
- sort
- order
- page
- page_size

搜索范围：

- 标题
- 副标题
- 作者
- ISBN
- 标签

### 上传和导入

```text
POST   /api/v1/imports
GET    /api/v1/imports
GET    /api/v1/imports/:id
POST   /api/v1/imports/:id/retry
```

上传流程：

1. 写入 `/app/data/imports`。
2. 校验文件大小。
3. 校验扩展名和 MIME。
4. 计算 SHA-256。
5. 检测重复文件。
6. 解析元数据。
7. 提取或生成封面。
8. 创建数据库事务。
9. 移动到正式目录。
10. 提交事务。
11. 失败时删除临时文件并回滚数据库。

### 作者和标签

```text
GET    /api/v1/authors
POST   /api/v1/authors
PUT    /api/v1/authors/:id
DELETE /api/v1/authors/:id

GET    /api/v1/tags
POST   /api/v1/tags
PUT    /api/v1/tags/:id
DELETE /api/v1/tags/:id
```

### 阅读进度

```text
GET    /api/v1/books/:id/progress
PUT    /api/v1/books/:id/progress
```

### 阅读笔记

```text
GET    /api/v1/books/:id/notes
POST   /api/v1/books/:id/notes
PUT    /api/v1/books/:id/notes/:noteId
DELETE /api/v1/books/:id/notes/:noteId
```

### 系统

```text
GET    /api/v1/system/info
GET    /api/v1/system/health
POST   /api/v1/system/scan
GET    /api/v1/system/scan/status
POST   /api/v1/system/backups
GET    /api/v1/system/backups
POST   /api/v1/system/backups/:id/restore
GET    /api/v1/system/manifest
```

---

## 八、OPDS 1.2

实现以下接口：

```text
GET /opds
GET /opds/recent
GET /opds/all
GET /opds/authors
GET /opds/authors/:id
GET /opds/tags
GET /opds/tags/:id
GET /opds/formats
GET /opds/formats/:format
GET /opds/search?q=
GET /opds/books/:id
GET /opds/books/:id/cover
GET /opds/books/:id/files/:fileId
GET /opensearch.xml
```

要求：

1. 使用 OPDS 1.2 Atom XML。
2. 使用 `encoding/xml`，禁止字符串拼接 XML。
3. XML 必须合法转义。
4. 根目录使用 Navigation Feed。
5. 书籍列表使用 Acquisition Feed。
6. 每本书输出：
   - id
   - title
   - updated
   - author
   - summary
   - language
   - category
   - cover link
   - thumbnail link
   - acquisition links
7. 同一本书可以输出多个 acquisition link。
8. 所有 URL 使用配置中的公开基础地址：
   - `PUBLIC_BASE_URL`
9. 支持分页。
10. 提供 next、previous、start、self、search 链接。
11. 提供 OpenSearch 描述文档。
12. OPDS 需要独立认证。
13. 第一版优先实现 HTTP Basic Auth。
14. Basic Auth 只能在 HTTPS 下使用。
15. OPDS 用户名和密码可单独配置：
   - `OPDS_USERNAME`
   - `OPDS_PASSWORD`
16. 后续预留 Token 认证扩展点。
17. 为 OPDS XML 编写单元测试和兼容性测试样例。

OPDS Content-Type：

```text
application/atom+xml;profile=opds-catalog;kind=navigation; charset=utf-8
application/atom+xml;profile=opds-catalog;kind=acquisition; charset=utf-8
```

---

## 九、网页端页面

### 登录页

- 用户名
- 密码
- 登录状态和错误提示

### 首页

- 最近阅读
- 最近添加
- 在读
- 未读
- 已读
- 书籍总数
- 存储空间

### 书籍列表

- 封面网格和表格两种视图
- 搜索
- 作者筛选
- 标签筛选
- 格式筛选
- 阅读状态筛选
- 排序
- 分页

### 书籍详情

- 封面
- 标题
- 作者
- 简介
- 标签
- 出版信息
- ISBN
- 阅读状态
- 评分
- 文件格式列表
- 在线阅读
- 下载
- 编辑
- 删除到回收站

### 上传页

- 单文件上传
- 多文件上传
- 上传进度
- 导入状态
- 失败原因
- 重复文件提示

### EPUB 阅读器

使用 epub.js，支持：

- 上一页
- 下一页
- 章节目录
- 字号
- 行距
- 浅色/深色主题
- 保存 CFI
- 保存百分比
- 恢复阅读位置

### PDF 阅读器

使用 PDF.js，支持：

- 翻页
- 页码跳转
- 缩放
- 保存页码
- 保存百分比
- 恢复阅读位置

### 系统设置

- OPDS 地址
- 当前存储路径
- 存储占用
- 创建备份
- 查看备份
- 触发扫描
- 扫描结果
- 修改密码

---

## 十、元数据解析

### EPUB

至少解析：

- title
- author
- language
- publisher
- identifier / ISBN
- description
- cover

注意：

- EPUB 是 ZIP，必须防止 Zip Slip。
- 限制解压总大小。
- 限制单文件大小。
- 不把整个 EPUB 永久解压到磁盘。
- 读取 container.xml 和 OPF。
- 对异常 EPUB 返回清晰错误。

### PDF

第一版至少解析：

- 文件名推导标题
- PDF 元数据中的 title
- PDF 元数据中的 author
- 页数
- 首页缩略图或默认封面

如果 PDF 封面生成依赖额外系统库，请：

1. 明确依赖；
2. 写入 Dockerfile；
3. 提供降级策略；
4. 依赖不可用时仍允许导入。

### MOBI / AZW3 / TXT

第一版只做：

- 上传
- 格式识别
- 文件管理
- OPDS 下载
- 手工编辑元数据

不要求在线阅读和深度元数据解析。

---

## 十一、安全要求

1. 所有上传接口限制文件大小。
2. 默认最大单文件大小通过环境变量配置：
   - `MAX_UPLOAD_SIZE_MB`
3. 严格限制允许的扩展名。
4. 不能信任客户端传入的 MIME。
5. 防止目录穿越。
6. 防止 Zip Slip。
7. 所有下载只能读取数据库记录对应的受控路径。
8. 不允许通过参数直接读取任意本地文件。
9. 登录、上传、OPDS 接口加入基础限流。
10. CORS 默认只允许同源。
11. 生产环境禁止调试模式。
12. 日志中不得打印密码、Session、完整 Authorization。
13. 错误响应不得暴露绝对文件路径。
14. 数据库启用外键。
15. 管理操作记录审计日志。
16. 下载接口设置正确的 Content-Disposition。
17. EPUB 和 PDF 资源响应支持 Range 请求。
18. 增加安全响应头。
19. Caddy 自动申请 HTTPS 证书。
20. README 明确说明公网部署前必须配置域名和 HTTPS。

---

## 十二、备份、恢复和扫描

### 备份

备份必须包含：

- SQLite 数据库一致性快照
- books 目录
- manifests
- 关键配置清单

不得直接在数据库活跃写入期间粗暴复制 SQLite 主文件。

优先使用：

- SQLite Backup API
- 或 `VACUUM INTO`

备份文件格式：

```text
backups/bookshelf-YYYYMMDD-HHMMSS.tar.gz
```

生成 SHA-256 校验值。

### 恢复

恢复前：

1. 校验备份；
2. 进入维护模式；
3. 备份当前数据；
4. 解压到临时目录；
5. 校验数据库和文件；
6. 原子替换；
7. 恢复失败时回滚。

### 扫描和重建

扫描功能需要检查：

- 数据库记录存在但文件缺失；
- 文件存在但数据库无记录；
- SHA-256 不一致；
- metadata.json 缺失；
- cover 缺失；
- 孤立目录；
- 无效格式；
- 回收站数据。

提供：

- 只检查模式；
- 自动修复安全问题；
- 输出扫描报告；
- 不自动删除未知文件。

---

## 十三、配置项

创建 `.env.example`：

```env
APP_ENV=production
APP_PORT=8080
DATA_DIR=/app/data
PUBLIC_BASE_URL=https://books.example.com
TZ=Asia/Shanghai

ADMIN_USERNAME=admin
ADMIN_PASSWORD=change-me-immediately

SESSION_SECRET=replace-with-a-long-random-string
SESSION_TTL_HOURS=168

OPDS_ENABLED=true
OPDS_USERNAME=reader
OPDS_PASSWORD=replace-with-a-strong-password

MAX_UPLOAD_SIZE_MB=500
SQLITE_BUSY_TIMEOUT_MS=5000
BACKUP_RETENTION_DAYS=30
LOG_LEVEL=info
```

要求：

- 启动时校验关键配置。
- 生产环境检测弱密码。
- Session Secret 长度不足时拒绝启动。
- 不在镜像中写死密钥。

---

## 十四、Docker Compose

实现：

- bookshelf 后端
- frontend 静态站点，或由后端统一托管构建产物
- Caddy
- 健康检查
- 自动重启
- 明确数据挂载
- 明确日志策略

宿主机挂载：

```yaml
volumes:
  - /opt/bookshelf/data:/app/data
```

禁止：

- `/tmp:/app/data`
- 匿名 volume 保存核心数据
- 书籍只存在于容器可写层

提供 Caddyfile，使：

```text
https://books.example.com
```

代理到应用。

提供以下命令：

```bash
make dev
make test
make build
make docker-build
make docker-up
make docker-down
make backup
make restore
make scan
```

---

## 十五、测试要求

### 后端单元测试

至少覆盖：

- 密码哈希与校验
- 登录
- 文件扩展名和 MIME 校验
- SHA-256
- 路径安全
- EPUB Zip Slip 防护
- 数据库模型
- 书籍搜索
- OPDS XML 序列化
- OPDS acquisition link
- 备份元数据
- 扫描逻辑

### 集成测试

至少覆盖：

1. 创建管理员；
2. 登录；
3. 上传 EPUB；
4. 上传 PDF；
5. 创建书籍；
6. 访问书籍列表；
7. 下载文件；
8. 获取 OPDS 根目录；
9. OPDS 搜索；
10. Basic Auth 错误时返回 401；
11. 删除书籍进入回收站；
12. 恢复书籍；
13. 创建备份；
14. 扫描数据库与文件。

### 前端测试

至少覆盖：

- 登录页
- 书籍列表
- 上传流程
- 阅读进度 API
- 路由权限守卫

---

## 十六、第一阶段实施范围

第一阶段只实现可稳定上线的 P0 + P1，不要先做所有高级功能。

必须完成：

1. 项目骨架；
2. 用户登录；
3. SQLite 和迁移；
4. EPUB、PDF、MOBI、AZW3、TXT 上传；
5. 书籍列表；
6. 书籍详情；
7. 元数据手工编辑；
8. EPUB 基础元数据解析；
9. PDF 基础元数据解析；
10. 文件下载；
11. 删除到回收站；
12. OPDS 1.2：
    - 根目录；
    - 最近添加；
    - 全部书籍；
    - 作者；
    - 标签；
    - 格式；
    - 搜索；
    - 封面；
    - 下载；
13. Basic Auth；
14. Docker Compose；
15. Caddy；
16. 数据目录持久化；
17. 基础备份；
18. 基础扫描；
19. README；
20. 测试。

第一阶段暂不实现：

- EPUB 网页阅读；
- PDF 网页阅读；
- 阅读笔记；
- 多用户复杂权限；
- 外部 OPDS 导入；
- OPDS 2.0；
- 自动格式转换；
- 第三方元数据刮削；
- 对象存储；
- 邮件通知。

完成第一阶段后停止继续扩展，输出：

- 已完成项；
- 目录结构；
- 运行命令；
- 默认访问地址；
- OPDS 地址；
- 测试结果；
- 尚未完成项；
- 已知风险；
- 第二阶段建议。

---

## 十七、第二阶段实施范围

第二阶段在第一阶段稳定后实现：

1. epub.js 阅读器；
2. PDF.js 阅读器；
3. 阅读进度；
4. 继续阅读；
5. 阅读状态；
6. 阅读笔记；
7. 书摘；
8. 更完整的备份恢复；
9. 更完善的扫描重建；
10. 前端体验优化。

不得提前进入第二阶段，除非第一阶段所有测试通过。

---

## 十八、验收标准

项目达到以下条件才算第一阶段完成：

1. `go test ./...` 通过。
2. 前端构建通过。
3. Docker Compose 可以从零启动。
4. 首次启动可创建管理员。
5. 登录后可以上传 EPUB、PDF。
6. 上传后文件实际保存到 `/app/data/books`。
7. 重建容器后书籍仍存在。
8. OPDS 阅读器可添加 `/opds` 地址。
9. OPDS 可以浏览和搜索书籍。
10. OPDS 可以下载 EPUB、PDF。
11. 未授权访问 OPDS 返回 401。
12. 数据库和书籍目录可被备份。
13. 系统扫描可以发现缺失文件。
14. 删除书籍后文件进入回收站。
15. README 包含完整部署、升级、备份和恢复说明。
16. 不存在将核心数据写入 `/tmp` 的行为。
17. 不存在仅保存在容器内部的数据。
18. 不依赖 Calibre。
19. 所有关键错误有日志。
20. 不存在明显目录穿越、Zip Slip 和任意文件读取漏洞。

---

## 十九、开始执行

现在开始执行以下步骤：

1. 扫描当前仓库。
2. 输出当前仓库分析。
3. 给出第一阶段实施计划。
4. 创建项目骨架。
5. 优先完成后端数据模型、认证、存储和上传。
6. 再实现前端基础页面。
7. 再实现 OPDS。
8. 再实现 Docker Compose、Caddy、备份和扫描。
9. 执行全部测试。
10. 输出最终结果。

不要只输出方案，必须实际修改仓库并运行验证命令。
