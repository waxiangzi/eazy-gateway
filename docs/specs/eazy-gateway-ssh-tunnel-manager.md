# eazy-gateway：基于 Web 的 SSH 隧道管理器

> 回溯性需求 spec。本文档由项目的原始计划整理而来，并以当前代码库的**实际实现**为准，作为项目的正式需求记录。凡计划与实现不一致处，以实现为准并显式标注。

## Problem Statement

作为一名频繁使用 SSH 隧道的运维/开发者，我过去依赖命令行的 `autossh` 来建立并维持 SSH 隧道（本地转发、远程转发、SOCKS5 动态代理）。这种方式存在几个痛点：

- 隧道配置散落在 shell 脚本或命令历史里，难以集中查看和管理。
- SSH 私钥以明文散落在文件系统，缺乏统一的加密存储。
- 连接断开后虽有 `autossh` 重连，但没有可视化的状态、流量和健康信息。
- 想临时启停某条隧道，要记住进程或端口，操作繁琐。
- 缺少一个简单、需要登录鉴权的图形界面，让我在浏览器里管理这一切。

## Solution

一个用 Go 编写、内嵌 Vue 3 单页应用的单管理员 Web 服务，把 SSH 隧道的全生命周期管理集中到一个浏览器控制台里，同时保留命令行操作能力：

- 集中管理 **SSH 主机（Host）** 配置与 **隧道（Tunnel）** 配置，二者解耦——隧道通过 ID 引用主机。
- 支持四种隧道类型：本地转发 `local`（-L）、远程转发 `remote`（-R）、动态 SOCKS5 代理 `dynamic`（-D）、以及 HTTP-to-SOCKS5 代理 `httpToSocks5`。
- SSH 连接自带健康检查 + 指数退避自动重连，行为对标 `autossh` 但更稳健。
- SSH 私钥以 AES-GCM 加密存储，密钥由管理员密码派生。
- 单管理员密码登录（无用户名、无多用户），会话基于 cookie 或 Bearer token。
- 实时流量统计与按隧道的流量趋势图。
- 打包为单个静态二进制（通过 `go:embed` 内嵌前端），附带 systemd 服务与体积极小的 Docker 镜像。
- 二进制同时充当 CLI 客户端，通过本地 Unix socket 控制正在运行的服务。
- 默认监听端口 `8022`，可通过 `--port` flag 或 `PORT` 环境变量配置。

## User Stories

### 部署与运行
1. As a 运维者, I want 用单个静态二进制启动服务, so that 我无需安装运行时依赖即可部署。
2. As a 运维者, I want 服务默认监听 8022 端口, so that 我有一个可预期的默认行为。
3. As a 运维者, I want 通过 `--port` flag 覆盖监听端口, so that 我能避开端口冲突。
4. As a 运维者, I want 通过 `PORT` 环境变量设置端口, so that 我能在容器/编排环境中用环境变量注入配置。
5. As a 运维者, I want 通过 `--data` flag 或 `DATA_DIRECTORY` 环境变量指定数据目录, so that 我能把持久化数据放到我选择的位置。
6. As a 运维者, I want 用 `--install` 一键安装为用户级 systemd 服务, so that 我不必手写 unit 文件。
7. As a 运维者, I want 服务在收到 SIGTERM/SIGINT 时优雅关闭, so that SSH 连接被清理、流量计数被保存，不丢数据。
8. As a 运维者, I want 用 `--serve` 让服务后台守护化运行, so that 我不必自己管理进程后台化。
9. As a 运维者, I want 一个 `/health` 健康检查端点, so that 我能接入负载均衡或存活探针。
10. As a 运维者, I want 端口被占用时得到清晰的错误提示, so that 我能快速定位启动失败原因。
11. As a 运维者, I want 用 Docker 镜像运行服务, so that 我能在容器平台上部署。
12. As a 运维者, I want Docker 运行时镜像足够小（基于 distroless）, so that 分发和拉取都很快。

### 认证与安全
13. As a 管理员, I want 用单个密码登录控制台, so that 未授权者无法访问。
14. As a 管理员, I want 首次启动时系统自动生成随机管理员密码并落盘到 `data/initial-password.txt`, so that 我有一个安全的初始凭据。
15. As a 管理员, I want 修改管理员密码, so that 我能定期轮换凭据。
16. As a 管理员, I want 修改密码后所有既有会话失效, so that 泄露的会话立即作废。
17. As a 管理员, I want 会话通过 cookie 维持, so that 我在浏览器里保持登录状态。
18. As a API 使用者, I want 通过 `Authorization: Bearer <token>` 头鉴权, so that 我能用脚本调用受保护的 API。
19. As a 管理员, I want 在 TLS 反代后启用 `--secure-cookies`, so that 会话 cookie 带上 Secure 标志。
20. As a 系统, I want 对同一 IP 的登录失败进行限速（多次失败后临时封锁）, so that 暴力破解被遏制。
21. As a 管理员, I want SSH 私钥以加密形式存储, so that 即使数据泄露，私钥也不以明文暴露。
22. As a 管理员, I want 用 CLI 重置管理员密码, so that 我在忘记密码时仍能恢复访问。

### SSH 主机管理
23. As a 管理员, I want 独立于隧道创建 SSH 主机配置（地址、端口、用户、密钥）, so that 多条隧道可以复用同一台主机的连接信息。
24. As a 管理员, I want 列出、查看、修改、删除主机配置, so that 我能维护我的主机清单。
25. As a 管理员, I want 在保存前测试与某主机的 SSH 连通性, so that 我能在创建隧道前确认配置正确。
26. As a 管理员, I want 删除被隧道引用的主机时被拒绝（返回冲突）, so that 我不会意外破坏正在使用的隧道。

### SSH 密钥管理
27. As a 管理员, I want 首次运行时系统自动生成一对默认 Ed25519 密钥, so that 我开箱即用无需手动准备密钥。
28. As a 管理员, I want 列出已有密钥（名称、公钥）, so that 我能把公钥加到目标服务器的 authorized_keys。
29. As a 管理员, I want 删除密钥, so that 我能清理不再使用的密钥。
30. As a 管理员, I want 删除被主机引用的密钥时被拒绝（返回冲突）, so that 我不会破坏依赖该密钥的主机配置。

### 隧道配置与生命周期
31. As a 管理员, I want 创建一条本地端口转发（-L）隧道, so that 我能把本地端口通过 SSH 转发到远端目标。
32. As a 管理员, I want 创建一条远程端口转发（-R）隧道, so that 我能把 SSH 服务器上的端口转发回本地目标。
33. As a 管理员, I want 创建一条动态 SOCKS5 代理（-D）隧道, so that 我能通过 SSH 连接建立本地 SOCKS5 代理。
34. As a 管理员, I want 创建一条 HTTP-to-SOCKS5 代理隧道, so that 本地 HTTP 代理能把流量转发到上游 SOCKS5。
35. As a 管理员, I want HTTP-to-SOCKS5 隧道支持 CONNECT 隧道与直连 HTTP 中继, so that 它能同时处理 HTTPS 与明文 HTTP 请求。
36. As a 管理员, I want 为 HTTP-to-SOCKS5 隧道配置按域名的路由规则（支持 `*.example.com` 通配）, so that 不同域名可以走不同的上游 SOCKS5。
37. As a 管理员, I want 列出、查看、修改、删除隧道配置, so that 我能维护我的隧道清单。
38. As a 管理员, I want 选择隧道监听在本地回环还是对外地址, so that 我能控制隧道的可访问范围。
39. As a 管理员, I want 手动启动、停止、重启某条隧道, so that 我能按需控制隧道的运行。
40. As a 管理员, I want 删除隧道时若其正在运行则先自动停止, so that 删除操作不会留下悬挂的连接。
41. As a 管理员, I want 隧道被标记为 enabled, so that 服务重启后能自动恢复它。

### 自动重连与稳定性
42. As a 管理员, I want 隧道在 SSH 服务器断开后自动重连, so that 我不必手动介入即可恢复隧道。
43. As a 管理员, I want 重连采用指数退避（1s→2s→…→60s 封顶）, so that 目标不可用时不会造成重连风暴。
44. As a 管理员, I want 系统周期性对连接做 SSH keepalive 健康检查, so that 死连接能被及时发现并触发重连。
45. As a 系统, I want 停止隧道时确保所有转发/中继 goroutine 干净退出, so that 长期运行不发生 goroutine 泄漏。
46. As a 管理员, I want 服务重启后自动恢复上次运行中的隧道, so that 重启不会中断我的隧道服务。

### 可观测性与流量
47. As a 管理员, I want 在仪表盘看到每条隧道的实时连接状态（connected/connecting/disconnected/error）, so that 我一眼掌握全局健康。
48. As a 管理员, I want 看到每条隧道累计的入站/出站字节数, so that 我了解隧道的使用量。
49. As a 管理员, I want 查看某条隧道在过去 N 小时（1–48）的流量趋势图, so that 我能观察流量随时间的变化。
50. As a 管理员, I want 停止隧道时其流量计数被持久化, so that 统计数据不会因停止而丢失。

### CLI 客户端
51. As a 管理员, I want 用 `eazy-gateway list` 在终端列出所有隧道, so that 我无需打开浏览器即可查看。
52. As a 管理员, I want 用 `eazy-gateway start/stop/restart <id>` 控制隧道, so that 我能在脚本或终端里操作。
53. As a 管理员, I want 用 `eazy-gateway status <id>` 查询隧道状态, so that 我能在自动化流程中检查隧道。
54. As a 管理员, I want CLI 在服务未运行时给出清晰提示, so that 我知道要先启动服务。

### 界面与国际化
55. As a 管理员, I want 一个现代化的 Vue 3 单页控制台（登录、仪表盘、隧道编辑、主机编辑、密钥管理、设置）, so that 我能通过图形界面完成所有管理操作。
56. As a 管理员, I want 前端以中文或英文显示, so that 我能用熟悉的语言操作。
57. As a 管理员, I want 自定义应用显示名称（appName）, so that 品牌名符合我的部署环境。
58. As a 管理员, I want 配置流量趋势图的默认时长（1–24 小时）, so that 图表默认展示我关心的时间窗口。
59. As a 管理员, I want 表单在提交前对输入做校验, so that 我能尽早得到错误反馈。

## Implementation Decisions

### 命名与配置
- 项目、Go module、二进制、systemd 服务、cookie 名、DB 文件名等**统一命名为 `eazy-gateway`**（原 `tun-console`）。Go module 路径为 `github.com/eazy-gateway/eazy-gateway`。
- 默认 HTTP 端口统一为 **8022**，可通过 `--port` flag 或 `PORT` 环境变量覆盖（flag 优先）。数据目录默认 `./data`，可通过 `--data` 或 `DATA_DIRECTORY` 覆盖。
- 配置仅通过命令行 flag 与上述环境变量控制。`config.example.yaml` 仅作参考，**运行时不加载**。

### 架构
- 后端 Go + `golang.org/x/crypto/ssh`；前端 Vue 3 + Pinia + Vue Router（hash 模式）+ Vue I18n。
- 前端通过 `go:embed`（`embed` build tag）内嵌进 Go 二进制；`noembed` tag 时从磁盘的 `web/dist` 提供，便于开发。
- 持久化用 bbolt（纯 Go 嵌入式 KV，单文件，无 CGO），通过本地 `replace` 指向自研的 `bbolt_shim` 兼容模块。选择 bbolt 而非 SQLite 是刻意的架构决策（偏好纯 KV、无 CGO）。
- 数据模型将 **Host（SSH 服务器配置）** 与 **Tunnel（转发规则）** 解耦，隧道通过 `hostId` 引用主机；私钥以文件存于密钥目录（`data/keys/`），**用管理员密码加密静态存储**，DB 中以路径引用。

### 模块划分
- `internal/db`：bbolt 持久化层，覆盖 host、tunnel、key、traffic、settings、admin 六类实体，读用 View、写用 Update 事务。
- `internal/crypto`：bcrypt 密码哈希；AES-GCM 密钥加密，密钥经 **PBKDF2-SHA256（10 万次迭代）** 从密码派生，存储格式为 `base64(salt || nonce || ciphertext)`。
- `internal/ssh`：隧道引擎（`TunnelEngine`）+ 四种转发器 + SOCKS5 客户端。引擎并发安全，`configs`/`keyCache`/`tunnels` 三类共享状态分锁保护；解密后的私钥仅驻留内存（`keyCache`），`Deregister` 时清零。
- `internal/api`：REST handlers（auth、hosts、tunnels、keys、settings、admin）。

### 隧道引擎
- 状态机：`connecting` → `connected`；健康检查失败进入重连（`disconnected`/`connecting` 交替），初次连接失败为 `error`。
- 健康检查间隔 5s，用 SSH 全局 keepalive 请求探活。
- 重连指数退避：初始 1s，倍增至 60s 封顶；仅在 context 被取消（Stop/Shutdown）时退出。
- `httpToSocks5` 类型不建立 SSH 连接，直接以本地监听 + 转发到上游 SOCKS5 的方式运行。
- 所有转发共用 `pipe` 双向中继，原子计数 bytesIn（remote→local）/ bytesOut（local→remote）。

### 认证与会话
- 单管理员，无用户名。密码 bcrypt 哈希存于 DB。
- 会话存于内存（token → 过期时间），有效期 24 小时；支持 cookie（`eazy-gateway-session`）与 `Authorization: Bearer` 两种携带方式。
- 登录限速：同一客户端 IP 在 5 分钟窗口内失败 5 次后临时封锁 5 分钟。
- 修改密码采用**直接重加密**架构：改密时用新密码重新加密所有已存密钥，并作废全部会话。
- 主机 key 验证策略：接受所有主机 key 并记录告警（不做严格 known_hosts 校验）。

### API 契约（关键点）
- 公开：`GET /health`、`GET /api/settings`。
- 认证：`POST /api/login`、`POST /api/logout`、`GET /api/me`。
- 受保护（cookie 或 Bearer）：hosts、tunnels、keys、settings（PUT）、admin/change-password 的全部 CRUD 与操作端点。
- 删除存在引用关系的资源返回 **409**（主机被隧道引用、密钥被主机引用）。
- 隧道流量趋势：`GET /api/tunnels/{id}/traffic/trend?hours=N`，N 取 1–48。
- 设置：`PUT /api/settings` 更新 appName 与 trafficTrendHours（1–24）。

### CLI 客户端
- 二进制无参数时作为 HTTP 服务器运行；带子命令时作为 CLI 客户端，经数据目录下的本地 Unix socket（`cli.sock`）与运行中的服务通信。
- 支持子命令：`list`、`start`、`stop`、`restart`、`status`、`reset-password`。

### 部署
- 多阶段 Dockerfile：node 构建前端 → golang 构建二进制 → distroless 运行时镜像，`EXPOSE 8022`。
- `--install` 生成用户级 systemd unit（`~/.config/systemd/user/eazy-gateway.service`），daemon-reload、enable、start 一气呵成。
- 首次启动：创建 `data/eazy-gateway.db`（0600）；若无密钥则在 `data/keys/default` 生成 Ed25519 密钥对（0600）；生成随机管理员密码，**只**写入 `data/initial-password.txt`（0600）——既不打印到 stdout，也不进日志。管理员密码同时是私钥的加密口令，因此它只落盘到这一个 0600 文件。

## Testing Decisions

- **好测试的定义**：只验证外部可观测行为，不绑定内部实现细节。对本项目而言，"外部行为"指编译后二进制的 CLI 输出、HTTP API 的状态码与响应字段、以及数据目录中产生的文件/文件名——而非内部函数签名或私有状态。
- **计划的首选接缝（单一、最高层）**：编译后二进制的黑盒行为。**实际落地时自动化测试落在包级接缝（`httptest` + 临时数据目录），二进制黑盒检查作为发布前验收保留（见末条）**——差异按本文档规则显式标注。该接缝原本要覆盖的行为：
  - `go build`（含 `-tags embed`）成功即验证了命名/import/目录一致性这一整类正确性。
  - `--help` 输出 `--port` 默认为 8022；不设 flag 时监听 8022，`--port N` 与 `PORT=N` 均能覆盖。
  - 首次启动在数据目录生成 `eazy-gateway.db`（而非旧名）。
  - 受保护端点在无凭据时返回鉴权错误；携带正确 cookie/Bearer 时可访问。
- **被测模块**：优先在二进制/HTTP 边界测试，覆盖 auth、hosts、tunnels、keys、settings 各 handler 的外部契约，以及隧道引擎的可观测状态转换（通过 status 端点）。
- **既有测试（prior art）**：`internal/api/keys_test.go` 在 `httptest` 边界覆盖密钥生命周期（删除密钥、末尾密钥删除后自动重建默认密钥、并发删除、被主机引用时拒绝、重加密不动密钥库外的文件）；`cmd/eazy-gateway/main_test.go` 与 `internal/bbolt_shim/bbolt_test.go` 覆盖首次启动/首次落盘产生的文件权限与日志行为。CI 的 `test` job 执行 `gofmt -l`、`go vet` 与两个模块的 `go test -race`，并阻断 `build-go` 与 `release`。
- **发布前验收**：对编译出的二进制做黑盒检查——curl 打 HTTP API 断言状态码与响应字段、shell 检查数据目录中的文件名与权限。这是最靠近真实部署的接缝，与上面的包级测试互为补充。

## Out of Scope

以下项在原始计划中被明确列为 Guardrails，且当前实现仍未包含：
- 多用户支持或 RBAC（保持单管理员）。
- HTTPS/TLS 终止（仅纯 HTTP；TLS 交由外部反向代理，配合 `--secure-cookies`）。
- 通知系统（邮件、Slack、webhook）。
- SSH agent 转发或 HSM 支持。
- OpenAPI/Swagger 生成。
- 外部数据库（MySQL、PostgreSQL）——坚持嵌入式 bbolt。

## Further Notes

- **计划与实现的偏离（实现即事实）**：原计划 Guardrails 曾把"独立 CLI 客户端"与"流量日志/带宽监控/分析"列为不做项，但当前代码**已实现** CLI 客户端与流量统计（累计计数 + 趋势图）。本 spec 以实现为准，将二者纳入范围。
- 原计划的隧道类型为三种（-L/-R/-D），实现中扩展出**第四种** `httpToSocks5`（含域名代理规则），此前的 README 未记录，现已在文档中补齐。
- 加密细节较计划更具体：AES-GCM + PBKDF2-SHA256（10 万次迭代），而非泛称的 "password-derived key"。
- 本 spec 属回溯性文档，用于沉淀已完成项目的需求与决策；仓库根目录暂无 `CONTEXT.md` 或 `docs/adr/`，如后续需要可按 domain-modeling 流程补充。
