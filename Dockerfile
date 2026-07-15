FROM node:24-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend-build
ARG GOPROXY=https://goproxy.cn,direct
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ENV GOPROXY=${GOPROXY}
WORKDIR /src
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories \
    && apk add --no-cache build-base
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 go build -trimpath -o /out/bookshelf ./cmd/server

FROM alpine:3.22 AS runtime
ARG BOOKSHELF_UID=10001
ARG BOOKSHELF_GID=10001
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g "${BOOKSHELF_GID}" bookshelf \
    && adduser -S -D -H -u "${BOOKSHELF_UID}" -G bookshelf bookshelf
WORKDIR /app
COPY --from=backend-build /out/bookshelf /app/bookshelf
COPY --from=frontend-build /src/frontend/dist /app/web
RUN mkdir -p /app/data \
    && chown bookshelf:bookshelf /app/data \
    && chmod 0750 /app/data
ENV APP_ENV=production \
    APP_PORT=8080 \
    DATA_DIR=/app/data \
    WEB_DIR=/app/web
USER bookshelf
EXPOSE 8080
ENTRYPOINT ["/app/bookshelf"]
