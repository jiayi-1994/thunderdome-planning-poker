# Docker Compose 部署

包含应用和 PostgreSQL 17，默认使用南京大学 GHCR 应用镜像与 DaoCloud PostgreSQL 镜像。无需编译源码。数据库保存在 Docker 命名卷中，只对外开放应用端口，默认 8080。

## 启动

需要 Linux 容器环境、Docker Engine、Docker Compose v2.20+。初始化脚本需要 `sh`、`sed` 和 `openssl`。

```bash
# 在部署包解压目录执行；只首次生成 .env，重复执行不会更换密钥
sh init-env.sh

# 修改 APP_DOMAIN 为服务器 IP 或实际访问域名，不带 http://、端口或路径
vi .env

docker compose config --quiet
docker compose pull
docker compose up -d
docker compose ps
```

浏览器访问 `http://服务器IP:8080`；本机测试可保持 `APP_DOMAIN=localhost`，访问 `http://localhost:8080`。服务器防火墙需要允许对应端口。

只有 YAML 文件时，手动在同目录创建 `.env` 并填写 `DB_PASSWORD`、`COOKIE_HASHKEY`、`CONFIG_AES_HASHKEY`，三个值分别用 `openssl rand -hex 32` 生成。还可以设置 `APP_DOMAIN` 等选项。所有密码都由部署时生成，包内不包含共用密码。

## 配置

| `.env` 变量 | 用途 |
| --- | --- |
| `APP_DOMAIN` | 浏览器访问的域名或 IPv4，不含协议、端口、路径 |
| `HTTP_PORT` | 宿主机端口，默认 8080 |
| `HTTP_WRITE_TIMEOUT` | 响应写入超时秒数，默认 60；避免慢速网络并行下载较大 JS 时被截断 |
| `BIND_ADDRESS` | 默认 `0.0.0.0`；仅本机反向代理访问可设 `127.0.0.1` |
| `APP_HTTPS` | 通过 HTTPS 反向代理访问时设 `true`，同时启用安全 Cookie 和 WebSocket TLS |
| `ADMIN_EMAIL` | 可选的管理员邮箱 |
| `APP_IMAGE` | 完整应用镜像地址，可用 tag 或 `@sha256:...` |
| `POSTGRES_IMAGE` | 完整 PostgreSQL 17 镜像地址 |

默认国内镜像：

```text
ghcr.nju.edu.cn/jiayi-1994/thunderdome-planning-poker:latest
m.daocloud.io/docker.io/library/postgres:17-bookworm
```

源站和 Harbor 地址可以在 `.env` 替换。源站分别是 `ghcr.io/jiayi-1994/thunderdome-planning-poker:latest` 和 `docker.io/library/postgres:17-bookworm`；私有 Harbor 需先同步镜像，再执行 `docker login 你的Harbor地址`。镜像站可能缓存旧版 `latest` 或临时不可用，部署结果取决于服务器网络。镜像地址映射参考：[南京大学](https://doc.nju.edu.cn/books/e1654/page/ghcr)、[DaoCloud](https://github.com/DaoCloud/public-image-mirror)。

应用为单副本，更新时会短暂中断 WebSocket 连接。默认关闭邮件，启用邮件时在 YAML 应用环境变量中配置 `SMTP_ENABLED: "true"`、SMTP 服务器与凭证。HTTPS 需要已有反向代理和证书；代理需支持 WebSocket，本文件不包含代理服务。

## 检查与更新

```bash
curl --fail http://localhost:8080/healthz
docker compose logs --tail=100 thunderdome db

# 更新镜像并重建发生变化的容器
docker compose pull
docker compose up -d

# 停止服务，保留数据库卷
docker compose down
```

应用使用 scratch 镜像，没有 shell 或 curl，因此不添加无法执行的容器健康命令。数据库健康检查通过后再启动应用；应用就绪可用上面的 HTTP 请求确认。`/healthz` 仅确认 HTTP 服务运行，不代替数据库和业务检查。

## 数据与密钥

备份时同时保存数据库和 `.env`，其中 `CONFIG_AES_HASHKEY` 用于加密 Jira 等配置，不能随意重置。已有数据库的密码变更需要同步修改数据库账户，只改 `.env` 不会改变已有账户密码。不要把 PostgreSQL 17 的旧卷直接挂载给其他主版本。

```bash
umask 077
docker compose exec -T db pg_dump -U thunderdome -d thunderdome -Fc > thunderdome.dump
cp .env thunderdome.env.backup
```

不要执行 `docker compose down -v`，它会删除数据卷。默认 Compose 项目名为 `thunderdome`；更改项目名会使用另一组资源。`.env` 中的密钥不可提交到 Git，也不要删除后重新生成再连接原数据库。

配置语义参考：[Compose 启动顺序](https://docs.docker.com/compose/how-tos/startup-order/)、[变量插值](https://docs.docker.com/reference/compose-file/interpolation/)。
