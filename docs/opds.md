# OPDS 1.2

OPDS 服务默认入口为：

```text
https://books.example.com/opds
```

它使用独立的 `OPDS_USERNAME` 和 `OPDS_PASSWORD` HTTP Basic Auth，不复用 Web Cookie。服务拒绝非 HTTPS 请求；反向代理必须正确传递 `X-Forwarded-Proto: https`。不要将应用容器端口直接暴露到公网。

已实现的目录：

- `/opds`：Navigation Feed
- `/opds/recent`、`/opds/all`
- `/opds/authors`、`/opds/authors/:id`
- `/opds/tags`、`/opds/tags/:id`
- `/opds/formats`、`/opds/formats/:format`
- `/opds/search?q=`
- `/opds/books/:id`
- `/opds/books/:id/cover`
- `/opds/books/:id/files/:fileId`
- `/opensearch.xml`

Feed 使用 `encoding/xml` 输出，并包含 self、start、search、next、previous、封面、缩略图和 acquisition link。所有绝对链接均基于 `PUBLIC_BASE_URL`。

本地直连调试需要模拟 HTTPS 代理头：

```bash
curl -u reader:password -H 'X-Forwarded-Proto: https' http://localhost:8080/opds
```

生产环境只应通过 Caddy 提供 OPDS。
