# Thunderdome Helm 部署包

独立 Chart，无需下载 Helm 子依赖。默认部署 1 个应用实例、PostgreSQL 17、10Gi 持久化卷，并自动生成数据库密码、Cookie 密钥和 Jira 配置加密密钥。应用镜像为 `ghcr.io/jiayi-1994/thunderdome-planning-poker:latest`，支持 Linux amd64 / arm64。

## 1. 快速安装

需要 Kubernetes 1.24+、Helm 3，以及可用的默认 StorageClass。客户端能够访问 Kubernetes API，**集群节点**需要能够拉取镜像。若没有默认 StorageClass，在自己的 values 文件中设置 `postgresql.persistence.storageClass`，或设置 `existingClaim` 使用预建 PVC。

在完整部署包解压目录执行以下命令（Linux shell）：

```bash
# 查看存储类，确认哪一个是 default
kubectl get storageclass

# 国内镜像配置；无需代理时去掉 -f values-cn.yaml
helm upgrade --install thunderdome ./thunderdome-0.1.0.tgz \
  --namespace thunderdome --create-namespace \
  -f values-cn.yaml --wait --timeout 10m

kubectl -n thunderdome get pods,pvc,svc
kubectl -n thunderdome port-forward svc/thunderdome 8080:8080
```

浏览器打开 `http://localhost:8080`。默认关闭邮件发送；需要注册邮件、密码重置等邮件能力时，配置 `app.smtpEnabled: true` 及 SMTP 环境变量。完整 values 可用 `helm show values ./thunderdome-0.1.0.tgz` 查看。

## 2. 镜像代理和固定版本

`values-cn.yaml` 使用以下地址：

| 组件 | 国内镜像地址 |
| --- | --- |
| 应用 | `ghcr.nju.edu.cn/jiayi-1994/thunderdome-planning-poker:latest` |
| PostgreSQL | `m.daocloud.io/docker.io/library/postgres:17-bookworm` |

2026-09-12 已验证以上两个镜像清单可匿名读取；应用镜像清单摘要与 GHCR 一致。公共镜像站可能限流、缓存旧的 `latest` 或临时不可用，正式环境可采用下方 Harbor 配置。DaoCloud 对本应用仓库返回 403，因此国内示例分别使用南京大学 GHCR 镜像与 DaoCloud PostgreSQL 镜像，不能把所有镜像都直接切到 DaoCloud。

代理地址映射参考：[南京大学 GHCR 镜像说明](https://doc.nju.edu.cn/books/e1654/page/ghcr)、[DaoCloud 镜像说明](https://github.com/DaoCloud/public-image-mirror)。也可以配置 `global.imageProxy`，生成 `<代理>/<原始仓库域名>/<原始镜像路径>:<tag>`。单镜像 `proxy: null` 继承全局代理，`proxy: ""` 明确关闭继承。替换 registry 的代理应直接改 `image.repository`。

可追加 `-f values-pinned.yaml` 固定本次应用和数据库镜像摘要，例如：

```bash
helm upgrade --install thunderdome ./thunderdome-0.1.0.tgz \
  -n thunderdome --create-namespace \
  -f values-cn.yaml -f values-pinned.yaml --wait --timeout 10m
```

`image.digest` 优先于 `tag`。只使用 `latest` 时，Pod 每次启动会重新检查镜像；同一配置再次执行 Helm upgrade 不一定重建 Pod。需要拉取更新时执行 `kubectl -n thunderdome rollout restart deployment/thunderdome`。

### Harbor / 私有仓库

先同步两个镜像，再修改 `values-harbor.yaml` 中的地址。可以在有源站网络权限的机器上用 Skopeo 同步全部架构：

```bash
skopeo login harbor.example.com
skopeo copy --all \
  docker://ghcr.io/jiayi-1994/thunderdome-planning-poker:latest \
  docker://harbor.example.com/apps/thunderdome-planning-poker:latest
skopeo copy --all \
  docker://docker.io/library/postgres:17-bookworm \
  docker://harbor.example.com/apps/postgres:17-bookworm
```

私有仓库的拉取凭证 Secret 名称默认为示例中的 `harbor-pull`，需要在 `thunderdome` namespace 预先创建 `kubernetes.io/dockerconfigjson` 类型 Secret。使用 `-f values-harbor.yaml` 安装；公开项目可以删除示例中的 `imagePullSecrets` 项。内网自签证书应在节点容器运行时配置信任。

## 3. 上环境访问

配置文件可以叠加，后面的值覆盖前面的值。

### NodePort

复制并修改 `values-nodeport.yaml` 的 `app.domain` 为浏览器能访问的域名或 IPv4 地址，不带协议、端口、路径。

```bash
helm upgrade --install thunderdome ./thunderdome-0.1.0.tgz \
  -n thunderdome --create-namespace \
  -f values-cn.yaml -f values-nodeport.yaml --wait --timeout 10m
```

默认示例端口为 `30080`，访问 `http://你的域名或IP:30080`。节点防火墙需要允许此端口。

### Ingress / HTTPS

复制并修改 `values-ingress.yaml` 中的域名、IngressClass 和证书 Secret。集群应已有 Ingress Controller；配置 DNS，并在同一 namespace 创建 TLS Secret。

```bash
helm upgrade --install thunderdome ./thunderdome-0.1.0.tgz \
  -n thunderdome --create-namespace \
  -f values-cn.yaml -f values-ingress.yaml --wait --timeout 10m
```

启用 Ingress TLS 会自动开启安全 Cookie 和 `wss://`。如果 TLS 在集群外终止，应设置 `app.https: true`。Ingress 必须支持 WebSocket，示例超时注解适用于 ingress-nginx，其他控制器按其规则配置。子路径部署使用 `app.pathPrefix: /poker`，Ingress 保留该前缀，不做 rewrite。

## 4. 外部 PostgreSQL 和密钥

内置 PostgreSQL 适合单实例部署。使用已有数据库时修改 `values-external-db.yaml`，提前创建空数据库和具备迁移权限的专用账号。应用会自动执行数据库迁移。

在目标 namespace 创建 Opaque Secret `thunderdome-credentials`，必须包含三个键：

| 键 | 内容 |
| --- | --- |
| `DB_PASS` | 真实数据库密码 |
| `COOKIE_HASHKEY` | 随机密钥，至少 32 字符 |
| `CONFIG_AES_HASHKEY` | 随机密钥，至少 32 字符；用于加密 Jira 等配置，务必保留 |

可用 `openssl rand -hex 32` 生成每个密钥。通过组织现有 Secret 管理方式导入；不要把真实密钥提交到 Git。然后追加 `-f values-external-db.yaml` 安装。`sslmode: require` 使用加密连接；需验证证书时改为 `verify-full` 并按数据库 CA 要求配置。

默认由 Chart 创建的 Secret 在升级时通过 Helm lookup 复用，卸载时保留。已有托管 Secret 的值不能直接在 values 中改写，模板会报错，避免数据库密码与已初始化的数据不一致。外部 Secret 修改后应手动重启相关工作负载；数据库密码变更还需要实际更新数据库账户密码。

**GitOps / 离线渲染：** `helm template` 不读取集群中的 Secret，每次可能生成不同密钥。Argo CD、Flux 或先渲染再 apply 的流程应配置 `secrets.existingSecret` 并自行管理稳定密钥。

## 5. 升级、备份与排错

应用使用进程内 WebSocket 状态，强制单副本，更新策略为 Recreate；升级时存在短暂中断。数据库也为单副本，无数据库高可用或自动备份。

升级前备份数据库及 `<release>-secrets` Secret，两者一起保存。使用 Linux shell 的示例：

```bash
umask 077
kubectl -n thunderdome exec thunderdome-postgresql-0 -- \
  pg_dump -U thunderdome -d thunderdome -Fc > thunderdome.dump
kubectl -n thunderdome get secret thunderdome-secrets -o yaml > thunderdome-secrets.yaml
```

持久化卷和托管 Secret 有 `helm.sh/resource-policy: keep`，`helm uninstall` 后仍保留，但**删除 namespace 仍会删除其中资源**。保留资源会成为 Helm 管理之外的资源，重新安装必须保持 release 名称、namespace 和密钥/存储配置一致，并核对原资源的 Helm 所有权；详见 [Helm 保留资源说明](https://helm.sh/docs/howto/charts_tips_and_tricks/)。不要删除密钥后再连接原数据库，也不要给旧数据目录直接更换 PostgreSQL 主版本。

不要在原卷上修改数据库名、用户名或 PostgreSQL 主版本来进行迁移；这些初始化参数仅对空数据目录生效。已建 PVC 的 StorageClass 不能直接修改；扩容需要存储类支持且新容量不能缩小。应用自动执行的数据库迁移可能无法靠 Helm rollback 撤销，回退前需评估数据库兼容性。

```bash
kubectl -n thunderdome get pods,pvc
kubectl -n thunderdome describe pod <异常Pod名称>
kubectl -n thunderdome logs deployment/thunderdome --tail=100
kubectl -n thunderdome logs statefulset/thunderdome-postgresql --tail=100
kubectl -n thunderdome get events --sort-by=.lastTimestamp
```

`ImagePullBackOff` 检查镜像代理、节点网络及拉取凭证；PVC Pending 检查 StorageClass 和容量；数据库权限错误检查卷是否支持 `fsGroup: 999` 及 PostgreSQL 用户写入。应用 `/healthz` 只确认 HTTP 服务运行，不能代替数据库可用性和业务检查。`--wait` 超时后可修正配置并重新执行升级命令。

Chart 没有额外初始化镜像。应用内置等待数据库的逻辑，默认启动探针允许约 10 分钟启动时间。
