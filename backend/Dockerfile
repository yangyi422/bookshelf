FROM golang:1.24-alpine AS build
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -o /out/bookshelf ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -S -g 10001 bookshelf && adduser -S -D -H -u 10001 -G bookshelf bookshelf
WORKDIR /app
COPY --from=build /out/bookshelf /app/bookshelf
RUN mkdir -p /app/data && chown bookshelf:bookshelf /app/data
USER bookshelf
EXPOSE 8080
ENTRYPOINT ["/app/bookshelf"]
