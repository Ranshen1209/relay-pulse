# Sakrylle Status Agent Guide

> 本文件面向仓库内所有「智能体 / 机器人 / 助手」，仅供 AI 使用和维护。人类贡献者一般无需阅读或修改。

## 0. 最高优先级

- 与维护者互动时，始终使用**简体中文**。
- 不提交真实 API Key、数据库密码、生产 `.env` 等敏感信息。
- 不直接修改 `internal/api/frontend/` 下的嵌入产物；前端源码在 `frontend/`，Go embed 所需的 `internal/api/frontend/dist/` 由构建脚本同步，且已被 `.gitignore` 忽略。
- 不随意新增顶层文档；确有必要时先与用户确认。
- 面向人类的回复、README 导航、Issue/PR 描述中，不主动推荐或暴露 `AGENTS.md`、`CLAUDE.md`，除非用户明确询问。
- 遇到仓库中已有未提交改动时，视为用户或其他工具的改动；除非用户明确要求，不要回滚。

## 1. 文档策略

面向人类读者，项目方重点维护的核心文档只有：

- `README.md`：入口、快速开始、本地开发。
- `QUICKSTART.md`：快速部署与常见问题。
- `docs/user/config.md`：配置与环境变量说明。
- `CONTRIBUTING.md`：贡献流程与规范。

`archive/` 中的文档均为历史文档。引用时必须标注：「历史文档，仅供参考，以当前核心文档和代码实现为准」。

评论、Issue、PR 描述和代码内用户可见文案优先使用中文；如需英文，应保证含义与中文一致，并以中文为主。外部工具或脚本只能输出英文时，在说明中补充简短中文解释。

## 2. 仓库事实

- 本仓库是 [prehisle/relay-pulse](https://github.com/prehisle/relay-pulse) 的 Sakrylle Status fork。
- 上游：`prehisle/relay-pulse`
- Fork：`Ranshen1209/sakrylle-status`
- 定制分支：`theme/sakrylle`，所有 Sakrylle 定制都在此分支。
- 镜像：`ghcr.io/ranshen1209/relay-pulse:sakrylle`
- 生产域名：`status.sakrylle.com`
- 服务器：`cliproxyapi-jp`（`64.83.47.108`，SSH 别名 `ssh-tokyo`）
- Compose stack：`/opt/stack/`
- 伴生网关：`Ranshen1209/sub2api`，分支 `theme/monet-purple`。

本站是 Sakrylle API 网关的伴生监测站。探针打的是网关上真实的 group，成本和排障都与网关强耦合。

查看 fork 与上游差异时，优先使用：

```bash
git diff upstream/main
```

## 3. 配置与安全

- 生产 API key 实际值放在服务器 `/opt/stack/relay-pulse/.env`，文件权限应为 `600`，通过 compose 的 `env_file:` 注入。
- 只更新 `config.yaml.example` 等示例配置；真实值通过环境变量或本地未入库配置文件注入。
- `config.yaml` 支持 fsnotify 热重载；`.env` 不支持热更新，改完必须重建容器。
- 修改存储相关逻辑时，需同时验证 SQLite（默认）和 PostgreSQL。
- sub2api 后台创建 key 时，命名为 `relay-pulse-<channel>`，并绑定对应 group。
- 环境变量名约定：`MONITOR_SAKRYLLE_<CHANNEL>_API_KEY`。
- `config.yaml` 中使用 `env_var_name:` 显式引用，覆盖自动名 `MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY`。
- 孤立 env 行（无 monitor 引用）无害，下次轮换时清理。

## 4. 生产环境

```text
/opt/stack/
├── docker-compose.yml
└── relay-pulse/
    ├── .env                    # API keys，mode 600，env_file 注入
    ├── config/
    │   ├── config.yaml
    │   └── templates/          # 探测模板
    └── (volume) stack_relay-pulse-data -> /data/monitor.db (SQLite)
```

- 反代链路：`Public 443 -> sslh -> 127.0.0.1:8443 -> Nginx (server_name status.sakrylle.com) -> relay-pulse:8080`
- Nginx 配置：`/opt/stack/nginx/conf.d/sakrylle-status.conf`
- Docker 网络：`stack_default`，与 sub2api 系列共用。

## 5. 工作流速查

### 代码改动

代码改动走 GitHub Actions 构建镜像，再由服务器拉取：

```bash
git push origin theme/sakrylle
gh run list -R Ranshen1209/sakrylle-status --limit=1
ssh ssh-tokyo 'cd /opt/stack && docker compose pull relay-pulse && docker compose up -d --force-recreate relay-pulse'
curl -sI https://status.sakrylle.com/health
```

### 仅配置改动

`config.yaml` 热更新，无需重启：

```bash
scp config/config.yaml ssh-tokyo:/opt/stack/relay-pulse/config/config.yaml
ssh ssh-tokyo 'docker logs --tail=20 relay-pulse 2>&1 | grep -i reload'
```

`.env` 改动必须重建：

```bash
ssh ssh-tokyo 'cd /opt/stack && docker compose up -d --force-recreate relay-pulse'
```

### 本地开发与校验

```bash
./scripts/setup-dev.sh
./scripts/setup-dev.sh --rebuild-frontend
make dev
cd frontend && npm run dev
make ci
```

推前最小校验：

```bash
go build -o /tmp/relay-pulse ./cmd/server
go vet ./...
go run ./cmd/verify/main.go -provider Sakrylle -service cc -v
```

`make ci` 包含 gofmt、vet、go test 和 npm lint。代码变更完成后优先运行。

## 6. 生产探针

生产当前为 4 个探针，全部 `interval: 3m`，全部打 `https://api.sakrylle.com`。每个 group 使用独立 API key，因为 sub2api 的 `api_keys.group_id` 是单值。

| Channel key | Service | Template | Model | Group (sub2api) | Rate |
|---|---|---|---|---|---|
| `gpt-pro` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Pro (id 14, 号池) | 0.5x |
| `gpt-pro-special` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Pro-Special (id 3，旧名 GPT-Pro) | 0.4x |
| `deepseek-official` | `dx` | `dx-flash-openai-chat` | `deepseek-v4-flash` | DeepSeek-Anthropic (id 9，官方直连) | 1.0x |
| `grok` | `gk` | `gk-grok-openai-chat` | `grok-4.20-0309-non-reasoning` | Grok-API (id 22) | 0.001x |

不监测：

- 用户指定不监测所有 Claude group。
- 生图分组：GPT-Image (id 5)、GPT-Image-2-4K (id 11)、GPT-Image-2-Async (id 21)，按调用计费，探针成本过高。
- 已下架：Agnes-API (id 23)、Deepseek-Special (id 6 -> 28)、Claude-Code-AWSQ (id 12)，见历史调整。

`gk`/`ag` 是新 service code，前端 `ServiceIcon.tsx` 已加 Grok/Agnes 品牌图标。Agnes 探针虽已下架，但图标保留；无探测时不渲染，移除会徒增 rebase 冲突。

### Retry / timeout

- 每个 monitor 固定 `retry: 3`，即总共 4 次尝试。
- 每个 monitor 固定 `timeout: 30s`。
- 模板默认 `retry: 0`。
- 配置优先级：`monitor > template > global`，见 `internal/config/lifecycle.go`。
- `internal/monitor/probe.go` 中超时不重试；per-call deadline 与 retry-loop ctx 共享，`timeout` 是所有尝试的总预算。
- 挑选 timeout 时要满足：`timeout > slowest_real_call + sum(backoffs)`。
- 默认退避：`retry_base_delay=200ms`、`retry_max_delay=2s`、`retry_jitter=0.2`。

### 探针历史

- 2026-05-26：探针节奏由 9m 收紧到 3m。
- 2026-06-04：sub2api 后台将旧 GPT-Pro (id 3) 重命名为 GPT-Pro-Special，新上线 GPT-Pro (id 14) 号池。relay-pulse 同步将 `gpt-pro` 历史数据迁移至 `gpt-pro-special`，新增 `gpt-pro` 和 `claude-code-awsq` 两个探针。
- 2026-06-13：下架 GPT-Plus (id 4) 探针，清除历史数据。探针数 7 -> 6。下架 Claude-Kiro (id 2) 探针，清除全部历史数据。探针数 6 -> 5。
- 2026-06-14：清空 monitor.db 全部历史（不备份），探针扩至 9 个。新增 `claude-kiro` (id 15)、`claude-kiro-special` (id 2，sub2api 把旧 Claude-Kiro 重命名而来)、`grok` (id 22)、`agnes` (id 23)。sub2api 同期把 id 6 `Deepseek` 重命名为 `Deepseek-Special`。补回此前仅存服务器、未入库的 `dx-flash-openai-chat.json` 模板。探针数 5 -> 9。
- 2026-06-15：清空 monitor.db 全部历史（不备份），下架 `agnes` (id 23) 探针。探针数 9 -> 8。`ag-flash-openai-chat.json` 模板与前端 Agnes 图标保留未删；`.env` 里 `MONITOR_SAKRYLLE_AGNES_API_KEY` 成孤立行。
- 2026-06-15（二）：下架 `deepseek` (Deepseek-Special) 探针，仅清该 channel 历史，未动 `deepseek-official`。探针数 8 -> 7。`.env` 里 `MONITOR_SAKRYLLE_DEEPSEEK_API_KEY` 成孤立行。`deepseek-official` 不受影响，保留。
- 2026-06-26：下架 `claude-code-awsq` (id 12) 探针，清除该 channel 历史数据。探针数 7 -> 6。`.env` 里 `MONITOR_SAKRYLLE_CLAUDE_CODE_AWSQ_API_KEY` 成孤立行。
- 2026-07-24：下架 `claude-kiro` 与 `claude-kiro-special`，Deepseek-Official 展示名改为 DeepSeek-Anthropic，清空 monitor.db 全部历史（不备份）。探针数 6 -> 4。两个 Claude API key 环境变量成为孤立行。

## 7. 高风险坑位

### claude-kiro-special 历史上必须使用 OpenAI-compatible 模板

不要把 `claude-kiro-special` 改回上游原生 `cc-haiku-arith` 模板。

上游 `cc-haiku-arith` 打 `/v1/messages` 并携带 claude-cli headers。Sakrylle 曾返回 `200` 且耗时约 `15ms`，但 model 字段为空，sub2api `usage_logs` 24 小时内无记录。也就是说探针显示成功，但没有经过真实计费链路，是静默假阳性。

`cc-haiku-openai-chat` 是 `cx-gpt-mini-chat` 的 fork，走 `POST /v1/chat/completions` 加 Claude model id。sub2api 内部执行 OpenAI -> Anthropic 翻译上行，会产生真实 `usage_logs`，约 2s，是真实计费。

### Grok / Agnes 使用 envelope 校验

`grok`/`agnes` 是逆向端点，chat-completions 路径拿不到稳定可读回复：

- Agnes (`agnes-2.0-flash` / `agnes-1.5-flash`)：永远 `content=null`、`finish_reason=length`、固定 128 输出 token，且忽略 `max_tokens`。`prompt_tokens` 固定 218，因为 sub2api 注入大 system prompt。
- Grok (`grok-4.20-0309-non-reasoning`)：能返回正文但算术不可靠，曾出现问 6+7 答 "7"。

这两类模板不发算术题、不校验答案，改用 envelope 校验：`success_contains: "{{MODEL}}"`。`{{MODEL}}` 在 `probe.go` 里会被替换成 request_model，匹配响应里回显的 `"model":"<id>"`。目标是确认 200、正确路由和真实计费（`usage_logs` 有行），不依赖正文。不要把它们换回算术模板。

`grok-build-console` 从未被真实调用，疑似非对话 console 模型，不要使用。Agnes `max_tokens` 调大无用，永远 128。

### Deepseek 探针走 OpenAI 路径

- `dx-flash-openai-chat.json` 是自建模板。
- `max_tokens: 8` 用来抑制 thinking-token 成本，deepseek 的 `thinking` block 不限制会爆。
- Anthropic 路径也能通，sub2api 可双向翻译，但计费语义不同，因此锁定 OpenAI 路径。
- 只探 `v4-flash`。`v4-pro` 与其上游同源，flash 通则 pro 通；pro 单价约 6x，不值得 3m 探一次。

### Deepseek-Special 已下架原因

Deepseek-Special 的 group 后台账号是 Krill 逆向号（`api.cdn-krill-ai.com/coding`，anthropic 平台）。Krill 的 `deepseek-v4-flash` 默认带 `thinking` 推理块，而 sub2api 的 cc 转发器处理不了流里的 `thinking` block，会报 `upstream stream ended without response` (502)。

排查结论：

- 直连 Krill（绕过 sub2api）stream/non-stream 都 200；加 `thinking:{type:"disabled"}` 后只回干净 text，说明 Krill 本身没坏。
- 官方 Deepseek (`api.deepseek.com/anthropic`) 默认不出 thinking，走同一条 sub2api cc 路径稳定 200。
- 经 sub2api 打 Krill 必 502，与 `max_tokens`、prompt、模板无关。

注意：sub2api 后台「测试账号连接」是非流式直连，对 thinking 号会假阳性（显示 active/测试完成），但真实服务路径是流式、必挂。看到账号“绿”不等于服务路径通。未采纳的修复路径是 sub2api 侧给该账号关 thinking，或换非 thinking 号。另注：group 28 有较紧的 RPM 限流，密集 curl 会 429。

### GPT-Image 已下线

GPT-Image 按调用计费，真实探针成本约 `$0.15/call`。仅 `/v1/models` ping 又拿不到实质信号，因此 2026-05-23 已移除。

## 8. Sakrylle 定制与上游同步

这些定制都在 `theme/sakrylle`，每次 rebase 上游都容易冲突：

- 主题：从上游 4 个主题改为 2 个主题（`default-dark`、`light-cool`），并通过 `prefers-color-scheme` 自动跟随浏览器，不提供手动切换。亮色主题使用 hue 256（Monet 薰衣草），不是上游 hue 210。
- 品牌：`Sakrylle Status` 分布于 React i18n（zh/en/ru/ja 4 个 locale）和 Go SSR meta（`internal/api/meta.go` title/description/JSON-LD/404）。
- Logo：樱花 SVG 替代上游 RP 文字 logo。
- 语言切换：使用文字代码（ZH/EN/RU/JA），无国旗图标。
- 删除页面/组件：`ContactPage`、`OnboardingPage`、`ChangeRequestPage`、`Footer.tsx`、`useOnboarding`、`useChangeRequest`、`utils/share.ts`。rebase 后它们可能回来，需要重新删除。
- 状态表：隐藏 Model/Price/Listed-days 列；`MOBILE_ROW_HEIGHT=200`；`HIDDEN_ANNOTATION_IDS` 过滤 `frequency`、`public_service`、`key_type`、`sponsor_*`。
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
make ci
```

rebase 后重点检查：已删页面是否回归、主题 hue 是否被上游覆盖、品牌字符串是否完整、状态表隐藏列和注解过滤是否还在。

## 9. 运维命令

```bash
# 状态/日志/重启
ssh ssh-tokyo 'cd /opt/stack && docker compose ps relay-pulse'
ssh ssh-tokyo 'cd /opt/stack && docker compose logs --tail=100 relay-pulse'
ssh ssh-tokyo 'cd /opt/stack && docker compose restart relay-pulse'

# 强制拉最新镜像
ssh ssh-tokyo 'cd /opt/stack && docker compose pull relay-pulse && docker compose up -d --force-recreate relay-pulse'

# 查 monitor.db：relay-pulse 镜像内没装 sqlite3，用临时 alpine 容器挂 volume 读
# 表：probe_history / status_events / channel_states / service_states / monitor_overrides（没有 events 表）
# probe_history 列：provider service channel model status sub_status latency timestamp(unix秒) error_detail http_code
ssh ssh-tokyo 'docker run --rm -v stack_relay-pulse-data:/data alpine sh -c "apk add -q sqlite && sqlite3 /data/monitor.db .tables"'
ssh ssh-tokyo 'docker run --rm -v stack_relay-pulse-data:/data alpine sh -c "apk add -q sqlite && sqlite3 -header -column /data/monitor.db \"SELECT channel,status,http_code,latency,timestamp FROM probe_history ORDER BY id DESC LIMIT 10;\""'

# 备份：挂 /opt/stack/backups 直接落盘，.backup 在线备份不锁库
ssh ssh-tokyo 'docker run --rm -v stack_relay-pulse-data:/data -v /opt/stack/backups:/backup alpine sh -c "apk add -q sqlite && sqlite3 /data/monitor.db \".backup /backup/relay-pulse-\$(date +%F).db\""'

# 健康
curl -sI https://status.sakrylle.com/health
curl -s https://status.sakrylle.com/api/version | jq
```

看到红块时，按端到端链路验证：

```bash
# 1. relay-pulse 侧 probe_id
ssh ssh-tokyo 'docker logs --tail=200 relay-pulse 2>&1 | grep -E "gpt-pro|probe_id"'

# 2. sub2api 侧 usage_logs，key_id 在 sub2api 后台查
ssh ssh-tokyo 'docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT created_at, model_name, total_cost FROM usage_logs WHERE api_key_id=<id> ORDER BY id DESC LIMIT 5"'

# 3. 如果 relay-pulse 日志 200 但 usage_logs 空，优先怀疑静默假阳性
```

## 10. API key 轮换

1. sub2api 后台 -> API Keys -> 重置 `relay-pulse-<channel>`，记下新值。
2. 修改 `/opt/stack/relay-pulse/.env` 中对应 `MONITOR_SAKRYLLE_<CHANNEL>_API_KEY=...`。
3. 运行 `docker compose up -d --force-recreate relay-pulse`，因为 env 不热更新。
4. 在 sub2api 后台撤销旧 key；Redis pub/sub 会自动失效，旧 key 缓存最长约 60s。

## 11. 相关上下文

- 网关仓库：`/Volumes/APFS_HD/Documents/Github/sub2api`，分支 `theme/monet-purple`。它的 `CLAUDE.md` 包含上游 channel 映射、group 倍率、`/v1/models` 聚合逻辑。
- Obsidian 部署笔记：`20 Work/ServerOps/Self-Hosted/Sub2API 部署.md`。
- Cursor 场景规则见 `.cursor/rules/`；完整 AI 运维手册仍可参考 `CLAUDE.md`。
