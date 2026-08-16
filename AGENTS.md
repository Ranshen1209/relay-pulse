# Sakrylle Status Agent Guide

> 本文件面向仓库内所有「智能体 / 机器人 / 助手」，仅供 AI 使用和维护。人类贡献者一般无需阅读或修改。

## 0. 最高优先级

- 与维护者互动时，始终使用**简体中文**。
- 不提交真实 API Key、数据库密码、生产 `.env` 等敏感信息。
- 不直接修改 `internal/api/frontend/` 下的嵌入产物；前端源码在 `frontend/`，Go embed 所需的 `internal/api/frontend/dist/` 由构建脚本同步，且已被 `.gitignore` 忽略。
- 不随意新增顶层文档；确有必要时先与用户确认。
- 面向人类的回复、README 导航、Issue/PR 描述中，不主动推荐或暴露 `AGENTS.md` 等内部 AI 运维指南，除非用户明确询问。
- 遇到仓库中已有未提交改动时，视为用户或其他工具的改动；除非用户明确要求，不要回滚。

## 1. 文档策略

面向人类读者，项目方重点维护的核心文档只有：

- `README.md`：入口、快速开始、本地开发。
- `QUICKSTART.md`：快速部署与常见问题。
- `docs/user/config.md`：配置与环境变量说明。
- `CONTRIBUTING.md`：贡献流程与规范。

`archive/` 中的文档均为历史文档。引用时必须标注：「历史文档，仅供参考，以当前核心文档和代码实现为准」。

评论、Issue、PR 描述和代码内用户可见文案优先使用中文；如需英文，应保证含义与中文一致，并以中文为主。外部工具或脚本只能输出英文时，在说明中补充简短中文解释。

## 2. 仓库与生产事实

- 本仓库是 [prehisle/relay-pulse](https://github.com/prehisle/relay-pulse) 的 Sakrylle Status fork。
- 上游：`prehisle/relay-pulse`。
- Fork：`Ranshen1209/sakrylle-status`。
- Sakrylle 定制分支：`theme/sakrylle`。
- 定制镜像发布源：`ghcr.io/ranshen1209/relay-pulse`。
- 生产域名：`status.sakrylle.com`。
- 唯一生产节点：`sakrylle-la`，`154.44.8.202`，Ubuntu 24.04.4 LTS，SSH 用户 `admin`。
- 日常 SSH 入口：`ssh-sakrylle.sakrylle.com:443`，本地别名 `ssh-sakrylle`。
- Stack：`/opt/stack`；生产 Compose：`/opt/stack/docker-compose.yml`。
- 生产容器：`relay-pulse`；网关仓库为 `Ranshen1209/sakrylle-api`，生产分支 `main`，集成分支 `theme/monet-purple`。

本站是 Sakrylle API 网关的伴生监测站。探针打的是网关上真实的 group，成本和排障都与网关强耦合。

香港、东京节点已经退役，不再是生产、升级或回滚目标。历史资料中可能出现 `cliproxyapi-jp`、`64.83.47.108`、`ssh-tokyo`、`hk-server` 和 `sslh`；这些名称只用于标记历史状态，禁止出现在新的运维命令、回滚方案或自动化默认值中。

查看 fork 与上游差异时，优先使用：

```bash
git diff upstream/main
```

## 3. 配置与安全

- 生产 API key 实际值只放在服务器 `/opt/stack/relay-pulse/.env`，权限应为 `600`，通过生产 Compose 的 `env_file` 注入。任何日志、命令输出、提交和回复都不得回显 key 值。
- 只更新示例配置或本地未入库配置；真实值通过服务器环境变量注入。
- `/opt/stack/relay-pulse/config/config.yaml` 支持 fsnotify 热重载；`.env` 不支持热更新，改完必须重建 `relay-pulse`。
- 修改存储相关逻辑时，同时验证 SQLite（生产默认）和 PostgreSQL（通用示例）。
- Sub2API 后台 key 命名约定为 `relay-pulse-<channel>`，每个 key 绑定一个 `group_id`。
- 环境变量名约定为 `MONITOR_SAKRYLLE_<CHANNEL>_API_KEY`，配置中用 `env_var_name` 显式引用。
- 孤立 env 行本身无害；清理或轮换前必须先确认没有仍在使用的 monitor。
- 未获得明确授权，不得写生产数据库、轮换 API key、变更 DNS、重启服务器或重启生产容器。
- 生产操作优先通过 SSH 在服务器本地完成。生产备份、数据库 dump、镜像归档和大文件应在服务器本地生成并落在 `/opt/stack/backups/`；不得把数据库 dump、镜像、归档或其他大文件经本机传往境外服务器。
- 修改 443 stream、sshd、防火墙、Docker 网络或边缘 Nginx 前，必须先建立并保持独立应急会话：

```bash
ssh -p 22 admin@154.44.8.202
```

  确认该会话可持续执行命令后，才能从第二个终端操作 443 入口。

## 4. 洛杉矶生产环境

    /opt/stack/
    ├── docker-compose.yml
    └── relay-pulse/
        ├── .env                    # API keys，权限 600
        └── config/
            ├── config.yaml
            └── templates/          # 探测模板

    Docker volume: stack_relay-pulse-data:/data
    SQLite:        /data/monitor.db

- relay-pulse 监听容器内部 8080，不直接发布宿主机端口。
- 生产共享网络为 Compose 默认的 stack_default。
- Nginx 配置：/opt/stack/nginx/conf.d/sakrylle-status.conf。
- 当前反代链路：

    Public :443
      -> Nginx stream + ssl_preread
         -> SSH: 宿主机 22
         -> TLS: 127.0.0.1:8443
            -> Nginx HTTPS server_name status.sakrylle.com
               -> relay-pulse:8080

- sslh 已停用；不要恢复旧的双协议入口或把旧服务器写进回滚路径。
- 2026-08-16 只读核验到生产 Compose 使用固定镜像 digest，relay-pulse 健康状态为 healthy；该核验不替代每次升级前的现场检查。
- 现场 Compose 当前给 relay-pulse 注入 TZ=Asia/Tokyo；这是容器日志时区，不代表节点位置。若要改为其他时区，需单独评估并授权，不在本次文档更新中隐式修改。

## 5. 镜像发布与生产升级

### 工作流对应关系

| 工作流 | 触发与镜像 | 用途与限制 |
|---|---|---|
| .github/workflows/build-sakrylle-image.yml | theme/sakrylle push/手动；ghcr.io/ranshen1209/relay-pulse:sakrylle、:latest | Sakrylle 定制镜像的直接发布源；当前 workflow 不输出 digest，必须从成功 run 或 registry 现场核对。 |
| .github/workflows/ci-release.yml | main push；逻辑上为 ghcr.io/ranshen1209/sakrylle-status:latest，版本 tag 从可变 latest 派生 | 通用主分支发布，不能直接当作 Sakrylle 生产镜像；当前是否启用需人工确认。 |
| .github/workflows/notifier-docker.yml | 独立 notifier；ghcr.io/ranshen1209/sakrylle-status/notifier:SHA、:latest | 与 relay-pulse 主容器解耦；生产是否使用需查看 notifier 自己的 Compose。 |

最近一次可查的定制镜像成功 run（2026-07-24，run 30068386885）对应 manifest digest 为：
sha256:a37489e63cf28d2d19b68bdab833afd7f471fd8c571206fbe20168375a750e48。
2026-08-16 在服务器现场确认 relay-pulse 正在使用同一 digest；这只是核验快照，不得把 tag 当成永久版本锁。

当前 build-sakrylle-image.yml 没有传入版本 build args，生产 /api/version 在 2026-08-16 返回 version=dev、git_commit=unknown、build_time=unknown。因此 /api/version 只能作为接口健康检查，版本归属必须以 CI run、Compose 固定 digest 和 docker inspect 为准。

仓库内的 docker-compose.yaml、docker-compose.pg.yaml 和 scripts/docker-build.sh 是通用/开发或上游示例，不是 /opt/stack/docker-compose.yml 的生产依据。

### 生产升级步骤

每次升级都要使用 CI 成功产物的明确 digest，不要直接 pull :latest：

1. 确认目标 commit 的 CI 成功，并记录目标镜像 manifest digest。
2. 在服务器本地备份 Compose 和 SQLite；不要把备份拉回本机。

    ssh ssh-sakrylle 'install -d -m 700 /opt/stack/backups && cp -a /opt/stack/docker-compose.yml /opt/stack/backups/docker-compose-before-$(date -u +%Y%m%dT%H%M%SZ).yml'
    ssh ssh-sakrylle 'docker run --rm -v stack_relay-pulse-data:/data -v /opt/stack/backups:/backup alpine sh -c "apk add -q sqlite && sqlite3 /data/monitor.db \".backup /backup/relay-pulse-before-$(date -u +%Y%m%dT%H%M%SZ).db\""'

3. 在服务器 Compose 中把 relay-pulse 的 image 更新为 ghcr.io/ranshen1209/relay-pulse@sha256:DIGEST，同时记录旧 digest 作为回滚点。
4. 在服务器本地执行 cd /opt/stack && docker compose config --quiet；失败即停止，不重建容器。
5. 只拉取并重建 relay-pulse：

    ssh ssh-sakrylle 'cd /opt/stack && docker compose pull relay-pulse && docker compose up -d --no-deps --force-recreate relay-pulse'

6. 检查容器健康、内部接口和公网链路：

    ssh ssh-sakrylle 'cd /opt/stack && docker compose ps relay-pulse'
    ssh ssh-sakrylle 'docker inspect relay-pulse --format "{{.State.Health.Status}} {{.Config.Image}}"'
    curl -fsS https://status.sakrylle.com/health
    curl -fsS https://status.sakrylle.com/api/version
    curl -fsS -I https://status.sakrylle.com/

7. 将新 digest、旧 digest、CI run、健康检查结果和变更时间写入维护记录。回滚时恢复旧 digest，重复第 4--6 步；不要使用浮动 tag 猜测版本。

### 配置与密钥变更

config.yaml 是小体积配置，可在确认备份后通过 scp 更新并观察热重载日志；.env 变更必须在服务器上重建 relay-pulse。这两类操作都不能把 key 值写进 shell 历史或日志。

### 本地开发与校验

    ./scripts/setup-dev.sh
    ./scripts/setup-dev.sh --rebuild-frontend
    make dev
    cd frontend && npm run dev
    go vet ./...
    go test ./...
    npm --prefix frontend run lint
    npm --prefix frontend run test -- --run

当前 Makefile 没有 ci target；推送前运行上面的四项检查。生产升级前还要完成 digest、Compose、健康检查和公网状态页核对。

`ci-release.yml` 中的 `VITE_NOTIFIER_API_URL` 和 notifier 截图配置的代码默认值仍指向上游历史域名，均未从当前生产现场验证。`notifier/README.md` 已改为显式占位符；不要把代码 fallback 当成生产入口，也不要据此写入新的服务器地址。

## 6. 当前生产探针（只读核验快照）

以下事实由 2026-08-16 通过 `ssh-sakrylle`、生产 `config.yaml`、Sub2API PostgreSQL、`/v1/models` 和最近日志核对得到。生产实际加载 3 个 monitor，provider 均为 `sakrylle`。未执行新的真实 GPT 请求，也未改生产配置或数据。

| Channel key | Service | Template / 配置 request model | Sub2API 当前 group / channel | 生产核验 |
|---|---|---|---|---|
| gpt-pro | cx | cx-gpt-mini-chat / gpt-5.4-mini | GPT-Pro，id 14 / OpenAI GPT，rate 0.35 | 配置模型不在 /v1/models；最近返回 503，24h usage log 为 0。gpt-5.4 仅为可见候选，真实请求与 usage 待生产核实。 |
| gpt-pro-special | cx | cx-gpt-mini-chat / gpt-5.4-mini | GPT-Pro-Special，id 3 / OpenAI GPT，rate 0.30 | 配置模型不在 /v1/models；最近返回 503，24h usage log 为 0。gpt-5.4 仅为可见候选，真实请求与 usage 待生产核实。 |
| deepseek-official | dx | dx-flash-openai-chat / deepseek-v4-flash | 当前组名 Deepseek-Anthropic，id 9 / Deepseek Channel，rate 1.0 | /v1/models 可见；最近日志持续 200，24h 有 480 条 usage log。 |

共同事实：生产配置声明 `interval: 3m`、`retry: 3`、`timeout: 30s`，API base URL 为 `https://api.sakrylle.com`，请求路径为 OpenAI-compatible `POST /v1/chat/completions`。`cx-gpt-mini-chat` 模板默认 `retry: 0`、`timeout: 15s`，`dx-flash-openai-chat` 默认 `retry: 0`、`timeout: 30s`；monitor 层值覆盖模板默认值。`retry: 3` 表示最多 4 次尝试，`timeout: 30s` 是整轮尝试共享的总预算，超时不会重试。默认退避为 `retry_base_delay=200ms`、`retry_max_delay=2s`、`retry_jitter=0.2`。

生产 key 的变量名和绑定状态已核对为 active：
MONITOR_SAKRYLLE_GPT_PRO_API_KEY -> group 14、
MONITOR_SAKRYLLE_GPT_PRO_SPECIAL_API_KEY -> group 3、
MONITOR_SAKRYLLE_DEEPSEEK_OFFICIAL_API_KEY -> group 9。
只记录状态，不记录 key 值。服务器 .env 中仍有若干历史 Claude/Grok/Agnes/旧 Deepseek key 行；它们不代表启用 monitor，清理前需单独授权。

/v1/models 结果：GPT 两个 key 返回 gpt-5.4、gpt-5.5、gpt-5.6-* 等模型，但没有 gpt-5.4-mini；DeepSeek key 返回 deepseek-v4-flash 与 deepseek-v4-pro。在 GPT 新模型完成一次真实请求并产生 usage log 前，不得把“可见”写成“已恢复”。

### 历史探针记录

下列调整记录保留用于审计，均不是当前启用列表：

- 2026-05-26（历史状态）：探针节奏由 9m 收紧到 3m。
- 2026-06-04（历史状态）：Sub2API 后台将旧 GPT-Pro (id 3) 重命名为 GPT-Pro-Special，新上线 GPT-Pro (id 14) 号池。relay-pulse 同步将 `gpt-pro` 历史数据迁移至 `gpt-pro-special`，新增 `gpt-pro` 和 `claude-code-awsq` 两个探针。
- 2026-06-13（历史状态）：下架 GPT-Plus (id 4) 探针，清除历史数据。探针数 7 -> 6。下架 Claude-Kiro (id 2) 探针，清除全部历史数据。探针数 6 -> 5。
- 2026-06-14（历史状态）：清空 `monitor.db` 全部历史（不备份），探针扩至 9 个。新增 `claude-kiro` (id 15)、`claude-kiro-special` (id 2，Sub2API 把旧 Claude-Kiro 重命名而来)、`grok` (id 22)、`agnes` (id 23)。Sub2API 同期把 id 6 `Deepseek` 重命名为 `Deepseek-Special`。补回此前仅存服务器、未入库的 `dx-flash-openai-chat.json` 模板。探针数 5 -> 9。
- 2026-06-15（历史状态）：清空 `monitor.db` 全部历史（不备份），下架 `agnes` (id 23) 探针。探针数 9 -> 8。`ag-flash-openai-chat.json` 模板与前端 Agnes 图标保留未删；`.env` 里 `MONITOR_SAKRYLLE_AGNES_API_KEY` 成孤立行。
- 2026-06-15（二，历史状态）：下架 `deepseek` (Deepseek-Special) 探针，仅清该 channel 历史，未动 `deepseek-official`。探针数 8 -> 7。`.env` 里 `MONITOR_SAKRYLLE_DEEPSEEK_API_KEY` 成孤立行；`deepseek-official` 不受影响。
- 2026-06-26（历史状态）：下架 `claude-code-awsq` (id 12) 探针，清除该 channel 历史数据。探针数 7 -> 6。`.env` 里 `MONITOR_SAKRYLLE_CLAUDE_CODE_AWSQ_API_KEY` 成孤立行。
- 2026-07-24（历史状态）：下架 `claude-kiro` 与 `claude-kiro-special`，Deepseek-Official 展示名改为 DeepSeek-Anthropic，清空 `monitor.db` 全部历史（不备份）。探针数 6 -> 4。两个 Claude API key 环境变量成为孤立行。
- 2026-07-24（二，历史状态）：下架 `grok`，清空 `monitor.db` 全部历史（不备份）。探针数 4 -> 3。`.env` 里的 `MONITOR_SAKRYLLE_GROK_API_KEY` 成为孤立行。
- 2026-07-24（三，历史状态）：卡片主标题由服务商名改为通道展示名；`deepseek-official` 的通道展示名由 DeepSeek-Anthropic 改为 DeepSeek。
- 2026-07-24（四，历史状态）：服务商展示名改为 Sakrylle，公开目标网址由 `sub.sakrylle.com` 改为 `ai1.sakrylle.com`；卡片标题仍显示通道名，外链确认弹窗显示服务商名。

## 7. 高风险探针与历史模板

Claude、Grok、Agnes、GPT-Image 和旧 Deepseek-Special 当前都不在生产 monitor 列表。保留模板或前端图标不等于可以重新启用；重新启用必须先核对 group、key、`/v1/models`、真实请求和 usage log。

历史 `gk` / `ag` service code 与前端 Grok/Agnes 图标保留用于兼容定制分支；无对应 monitor 时不会渲染，不应把保留代码误认成已启用探针。

### GPT 候选模型（当前待生产核实）

`cx-gpt-mini-chat` 仍请求 `gpt-5.4-mini`；该模型已确认不在两个 GPT key 的 `/v1/models` 中，最近探针返回 503。`gpt-5.4` 目前只是在 `/v1/models` 可见的候选，必须先获得真实请求授权并确认产生 usage log，才能修改模板或部署。禁止只因模型可见就批量替换。

### claude-kiro-special（历史状态，当前未启用）

历史上不能把 `claude-kiro-special` 改回上游原生 `cc-haiku-arith` 模板。上游模板打 `/v1/messages` 并携带 claude-cli headers；Sakrylle 曾返回 200 且耗时约 15ms，但 model 字段为空、Sub2API `usage_logs` 24 小时内无记录，属于静默假阳性。

历史验证可用的 `cc-haiku-openai-chat` 是 `cx-gpt-mini-chat` 的 fork，走 `POST /v1/chat/completions` 加 Claude model id；Sub2API 执行 OpenAI -> Anthropic 翻译后曾产生真实 usage log。若未来考虑恢复，仍须按当时生产重新核验，不能沿用历史成功结论。

### Grok / Agnes（历史状态，当前未启用）

两者是逆向端点，chat-completions 路径历史上拿不到稳定可读回复：

- Agnes (`agnes-2.0-flash` / `agnes-1.5-flash`) 曾固定 `content=null`、`finish_reason=length`、128 输出 token，且忽略 `max_tokens`。
- Grok (`grok-4.20-0309-non-reasoning`) 曾能返回正文但算术不可靠。

因此历史模板不用算术答案校验，而用 `success_contains: "{{MODEL}}"` 做 envelope/model 回显校验，并另查 usage log。不要把这些历史模板改成普通算术探针；`grok-build-console` 从未被真实验证，不要使用。

### DeepSeek（当前启用）

- `dx-flash-openai-chat.json` 是自建模板，固定走 OpenAI-compatible 路径。
- `max_tokens: 8` 用来抑制 thinking-token 成本；不要未经验证切换协议或提高上限。
- 当前只探 `deepseek-v4-flash`；`deepseek-v4-pro` 虽在 `/v1/models` 可见，但未配置为探针。

### Deepseek-Special（历史状态，当前未启用）

历史 group 的后台账号是 Krill 逆向号（`api.cdn-krill-ai.com/coding`，Anthropic 平台）。Krill 的 `deepseek-v4-flash` 默认带 thinking block，而当时 Sub2API 的 cc 转发器处理不了流里的 thinking block，会返回 `upstream stream ended without response` (502)。

历史排查结论：直连 Krill 的 stream/non-stream 都曾返回 200；经当时 Sub2API 的真实流式路径会 502。后台“测试账号连接”是非流式直连，对这类账号可能假阳性。该结论只用于解释历史下架原因，不证明当前上游实现仍相同。

### GPT-Image（历史状态，当前未启用）

GPT-Image 按调用计费，历史真实探针成本约 `$0.15/call`；仅用 `/v1/models` 又拿不到实质服务信号，因此 2026-05-23 已移除。恢复前必须重新核算成本并获得授权。

## 8. Sakrylle 定制与上游同步

这些定制都在 `theme/sakrylle`，每次 rebase 上游都容易冲突：

- 主题：从上游 4 个主题改为 2 个主题（`default-dark`、`light-cool`），并通过 `prefers-color-scheme` 自动跟随浏览器，不提供手动切换。亮色主题使用 hue 256（Monet 薰衣草），不是上游 hue 210。
- 品牌：`Sakrylle Status` 分布于 React i18n（zh/en/ru/ja 4 个 locale）和 Go SSR meta（`internal/api/meta.go` title/description/JSON-LD/404）。
- Logo：樱花 SVG 替代上游 RP 文字 logo。
- 语言切换：使用文字代码（ZH/EN/RU/JA），无国旗图标。
- 删除页面/组件：`ContactPage`、`OnboardingPage`、`ChangeRequestPage`、`Footer.tsx`、`useOnboarding`、`useChangeRequest`、`utils/share.ts`。rebase 后它们可能回来，需要重新删除。
- 状态视图：桌面端默认使用卡片视图，移动端强制表格；隐藏 Model/Price/Listed-days 列；`MOBILE_ROW_HEIGHT=200`；`HIDDEN_ANNOTATION_IDS` 过滤 `frequency`、`public_service`、`key_type`、`sponsor_*`。
- Channel 解析：`parseChannelType` 对无前缀 channel 返回 `null`，上游返回 `'unknown'`。
- 主题逻辑：`useTheme.ts` 监听浏览器深浅色偏好；截图模式仍强制 `default-dark`。

冲突常客：

- `frontend/src/i18n/locales/{zh,en,ru,ja}.json`
- `internal/api/meta.go`
- `frontend/src/styles/themes/*.css`
- `frontend/src/components/{Header,StatusTable,StatusCard,Footer,RefreshButton,ChannelTypeIcon}.tsx`
- `frontend/src/hooks/useTheme.ts`
- `frontend/src/router.tsx`
- `Dockerfile`

同步流程：

```bash
git fetch upstream
git checkout theme/sakrylle
git rebase upstream/main
go vet ./...
go test ./...
npm --prefix frontend run lint
npm --prefix frontend run test -- --run
```

rebase 后重点检查：已删页面是否回归、主题 hue 是否被上游覆盖、品牌字符串是否完整、状态表隐藏列和注解过滤是否还在。

## 9. 日常只读与故障排查

    # 状态、日志、健康
    ssh ssh-sakrylle 'cd /opt/stack && docker compose ps relay-pulse'
    ssh ssh-sakrylle 'cd /opt/stack && docker compose logs --tail=100 relay-pulse'
    curl -fsS https://status.sakrylle.com/health
    curl -fsS https://status.sakrylle.com/api/version

    # 生产配置和镜像（只读）
    ssh ssh-sakrylle 'sudo stat -c "%a %U:%G %n" /opt/stack/relay-pulse/.env'
    ssh ssh-sakrylle 'docker inspect relay-pulse --format "{{.Config.Image}} {{.State.Health.Status}}"'
    ssh ssh-sakrylle 'cd /opt/stack && docker compose config --quiet'

    # SQLite 查询：只在服务器本地挂载 volume，禁止把数据库复制到本机
    ssh ssh-sakrylle 'docker run --rm -v stack_relay-pulse-data:/data alpine sh -c "apk add -q sqlite && sqlite3 /data/monitor.db .tables"'
    ssh ssh-sakrylle 'docker run --rm -v stack_relay-pulse-data:/data alpine sh -c "apk add -q sqlite && sqlite3 -header -column /data/monitor.db \"SELECT channel,status,http_code,latency,timestamp FROM probe_history ORDER BY id DESC LIMIT 10;\""'

看到红块时按端到端链路核对：relay-pulse 日志 -> 对应 API key 的 Sub2API usage_logs -> group/channel/model；日志 200 但 usage log 为空时，优先怀疑静默假阳性，不要先改公开历史数据。

## 10. adjust_availability.py 处置

`scripts/adjust_availability.py` 是高风险历史入口，不属于常规运维流程。它原先会删除最近 24 小时 `probe_history`，再插入伪造的成功/失败记录，从而改变公开可用率。当前工作树只保留不可连接 SSH 或数据库的告警 stub，危险实现仅由 Git 历史保存；禁止执行，也没有生产运行授权。不得把它改成新 SSH 别名后继续使用。

若将来确有业务需求，重新设计必须先获得单独授权，并至少满足：

1. 默认只读或 dry-run，写入只能由显式、一次性的开关启用。
2. SSH 主机、数据库路径、provider/service/channel/model 和时间窗口均由参数传入，不得硬编码。
3. 执行前在服务器本地完成 Compose 与 SQLite 备份，记录可验证的恢复命令。
4. 写入前做二次人工确认，校验目标 channel/model 的现存记录数量和时间范围，拒绝模糊匹配。
5. 每次读写记录操作者、命令摘要、窗口、行数、前后校验和，形成审计日志。
6. 提供恢复/回滚演练和只读验证；未完成方案评审前不得恢复写入能力。

## 11. API key 轮换

API key 轮换需要维护者明确授权，且不得在本地保存真实值：

1. 在 Sub2API 后台重置 relay-pulse-CHANNEL，只在密码管理器或服务器受控环境记录新值。
2. 修改服务器 /opt/stack/relay-pulse/.env 对应变量并确认权限仍为 600。
3. 因 .env 不热更新，在服务器执行 cd /opt/stack && docker compose up -d --no-deps --force-recreate relay-pulse。
4. 检查容器健康、探针日志和 usage log，再按授权撤销旧 key；不要在回复或日志中打印 key。

## 12. 相关上下文

- 网关仓库：/Users/cervine/Documents/Sakrylle/Sakrylle API；生产分支 main，theme/monet-purple 仅为集成分支。
- 当前 ServerOps 文档：20 Work/ServerOps/Server 配置.md、20 Work/ServerOps/Server 维护.md、20 Work/ServerOps/服务器迁移计划.md（Obsidian vault 内）。
- 这些文档描述的是洛杉矶单节点架构；旧香港、东京和旧边缘代理内容只按历史记录阅读，不作为当前入口。
