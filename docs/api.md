# API（第一阶段进行中）

除健康检查和登录外，下列接口均要求 `bookshelf_session` Cookie。请求和响应使用 JSON，文件上传使用字段名为 `file` 的 `multipart/form-data`。

## 认证

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/change-password`

## 书籍与文件

- `GET /api/v1/books?keyword=&author_id=&tag_id=&format=&reading_status=&sort=&order=&page=1&page_size=20`
- `GET /api/v1/books/trash`
- `POST /api/v1/books`
- `GET /api/v1/books/:id`
- `PUT /api/v1/books/:id`
- `DELETE /api/v1/books/:id`
- `POST /api/v1/books/:id/restore`
- `GET /api/v1/books/:id/cover`
- `POST /api/v1/books/:id/cover`（multipart 字段 `cover`）
- `GET /api/v1/books/:id/files`
- `POST /api/v1/books/:id/files`
- `DELETE /api/v1/books/:id/files/:fileId`
- `GET /api/v1/books/:id/files/:fileId/download`

创建和更新书籍可提交 `title`、`subtitle`、`description`、`language`、`publisher`、`isbn`、`reading_status`、`rating`、`author_ids` 和 `tag_ids`。`rating` 范围为 0–5。

## 导入

- `POST /api/v1/imports`
- `GET /api/v1/imports`
- `GET /api/v1/imports/:id`

导入会解析 EPUB 的标题、作者、语言、出版社、标识符、简介和封面；PDF 在不依赖外部系统库的前提下解析标题、作者和基础页数。解析不到标题时使用文件名降级。

## 作者和标签

- `GET|POST /api/v1/authors`
- `PUT|DELETE /api/v1/authors/:id`
- `GET|POST /api/v1/tags`
- `PUT|DELETE /api/v1/tags/:id`

尚未实现的第一阶段接口不会以空壳成功响应。

## 系统

- `GET /api/v1/system/info`
- `POST|GET /api/v1/system/backups`
- `GET /api/v1/system/backups/:id/validate`
- `POST /api/v1/system/scan`
- `GET /api/v1/system/scan/status`
- `GET /api/v1/system/manifest`
