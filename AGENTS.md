# Repository Guidelines

> 本文件为仓库中所有「智能体 / 机器人 / 助手」提供协作规范，**仅供 AI 使用和维护**。人类贡献者一般无需阅读或修改本文件。

## 交互与语言约定

- 本项目所有「智能体 / 机器人 / 助手」与维护者互动时，应**始终使用简体中文**进行沟通与回复。
- 评论、Issue、PR 描述和代码内用户可见文案，优先使用中文；如需英文，请保证含义与中文一致，并以中文为主。
- 如果外部工具或脚本只能输出英文，应在说明中简要补充中文解释。

## 文档策略（仅供 AI）

- 面向人类读者，项目方**重点维护的核心文档只有**：
  - `README.md`（入口、快速开始、本地开发）
  - `QUICKSTART.md`（快速部署与常见问题）
  - `docs/user/config.md`（配置与环境变量说明）
  - `CONTRIBUTING.md`（贡献流程与规范）
- `AGENTS.md`、`CLAUDE.md` 视为 AI 内部文档，**不要在面向人类的回复、README 导航等位置主动推荐或暴露**，除非用户明确询问。
- `archive/` 中的文档均为**历史文档**：可以作为补充背景使用，但引用时必须标注「历史文档，仅供参考，以当前核心文档和代码实现为准」。
- AI 不应随意新增顶层文档；如确有必要，应与用户确认后再创建。

## 仓库背景

- 本仓库是 [prehisle/relay-pulse](https://github.com/prehisle/relay-pulse) 的 Sakrylle Status fork。
- 上游：`prehisle/relay-pulse`
- Fork：`Ranshen1209/sakrylle-status`
- 定制分支：`theme/sakrylle`，所有 Sakrylle 定制都在此分支。
- 镜像：`ghcr.io/ranshen1209/relay-pulse:sakrylle`
- 生产域名：`status.sakrylle.com`
- 服务器：`cliproxyapi-jp`（`64.83.47.108`，SSH 别名 `ssh-tokyo`）
- Compose stack：`/opt/stack/`
- 本站是 Sakrylle API 网关（`Ranshen1209/sub2api`，分支 `theme/monet-purple`）的伴生监测站。探针打的是网关上真实的 group，成本和排障与网关强耦合。

查看 fork 与上游差异时，优先使用：

```bash
git diff upstream/main
```

## 配置与安全提示

- 禁止提交真实 API Key、数据库密码等敏感信息；仅更新 `config.yaml.example`，实际值通过环境变量或本地未入库配置文件注入。
- API key 生产实际值放在服务器 `/opt/stack/relay-pulse/.env`，文件权限应为 `600`，通过 compose 的 `env_file:` 注入。
- `config.yaml` 支持 fsnotify 热重载；`.env` **不支持热更新**，改完必须重建容器。
- 修改与存储相关逻辑时，需同时在 SQLite（默认）和 PostgreSQL 场景下验证。
- 不要直接修改 `internal/api/frontend/` 下的嵌入产物；前端源码在 `frontend/`，Go embed 需要的 `internal/api/frontend/dist/` 由构建脚本同步，且已被 `.gitignore` 忽略。

## 构建与部署

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

## 生产服务器布局

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

## 生产探针

生产当前为 6 个探针，全部 `interval: 3m`，全部打 `https://api.sakrylle.com`。每个 group 使用独立 API key，因为 sub2api 的 `api_keys.group_id` 是单值。

| Channel key | Service | Template | Model | Group (sub2api) | Rate |
|---|---|---|---|---|---|
| `claude-kiro-special` | `cc` | `cc-haiku-openai-chat` (fork) | `claude-haiku-4-5-20251001` | Claude-Kiro-Special (id 2) | 0.5x |
| `claude-code-awsq` | `cc` | `cc-haiku-openai-chat` | `claude-haiku-4-5-20251001` | Claude-Code-AWSQ (id 12) | 0.4x |
| `gpt-pro` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Pro (id 14, 号池) | 0.5x |
| `gpt-pro-special` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Pro-Special (id 3, 旧名 GPT-Pro) | 0.4x |
| `deepseek` | `dx` | `dx-flash-openai-chat` | `deepseek-v4-flash` | Deepseek (id 6, 阿里 Token 转售) | 0.7x |
| `deepseek-official` | `dx` | `dx-flash-openai-chat` | `deepseek-v4-flash` | Deepseek-Official (id 9, 官方直连) | 1.0x |

不监测：

- GPT-Image (id 5)、GPT-Image-2-4K (id 11)：按调用计费，探针成本过高。
- Claude-Code (id 7)、Claude-Special (id 8)、GPT-Plus (id 4)、GPT-Plus-Special (id 10)：已下架。

重要历史：

- 2026-05-26：探针节奏由 9m 收紧到 3m。
- 2026-06-04：sub2api 后台将旧 GPT-Pro (id 3) 重命名为 GPT-Pro-Special，新上线 GPT-Pro (id 14) 号池。relay-pulse 同步将 `gpt-pro` 历史数据迁移至 `gpt-pro-special`，新增 `gpt-pro` 和 `claude-code-awsq` 两个探针。
- 2026-06-13：下架 GPT-Plus (id 4) 探针，清除历史数据。探针数 7→6。

### Retry / timeout

- 每个 monitor 固定 `retry: 3`，即总共 4 次尝试。
- 每个 monitor 固定 `timeout: 30s`。
- 模板默认 `retry: 0`。
- 配置优先级：`monitor > template > global`，见 `internal/config/lifecycle.go`。
- `internal/monitor/probe.go` 中超时不重试；per-call deadline 与 retry-loop ctx 共享，`timeout` 是所有尝试的总预算。
- 挑选 timeout 时要满足：`timeout > slowest_real_call + sum(backoffs)`。
- 默认退避：`retry_base_delay=200ms`、`retry_max_delay=2s`、`retry_jitter=0.2`。

### API key 约定

- sub2api 后台创建 key，命名 `relay-pulse-<channel>`，绑定对应 group。
- 环境变量名约定：`MONITOR_SAKRYLLE_<CHANNEL>_API_KEY`。
- `config.yaml` 中使用 `env_var_name:` 显式引用，覆盖自动名 `MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY`。
- 孤立的 env 行（无 monitor 引用）无害，下次轮换时清理。

## 已踩过的坑

### claude-kiro-special 必须使用 OpenAI-compatible 模板

不要把 `claude-kiro-special` 改回上游原生 `cc-haiku-arith` 模板。

上游 `cc-haiku-arith` 打 `/v1/messages` 并携带 claude-cli headers。Sakrylle 曾返回 `200` 且耗时约 `15ms`，但 model 字段为空，sub2api `usage_logs` 24 小时内无记录。也就是说探针显示成功，但没有经过真实计费链路，是静默假阳性。

`cc-haiku-openai-chat` 是 `cx-gpt-mini-chat` 的 fork，走 `POST /v1/chat/completions` 加 Claude model id。sub2api 内部执行 OpenAI -> Anthropic 翻译上行，会产生真实 `usage_logs`，约 2s，是真实计费。

### Deepseek 探针走 OpenAI 路径

- `dx-flash-openai-chat.json` 是自建模板。
- `max_tokens: 8` 用来压低 thinking-token 成本。
- Anthropic 路径也能通，但计费语义不同，因此锁定 OpenAI 路径。
- 只探 `v4-flash`。`v4-pro` 与其上游同源，flash 通则 pro 通；pro 单价约 6x，不值得 3m 探一次。

### GPT-Image 已下线

GPT-Image 按调用计费，真实探针成本约 `$0.15/call`。仅 `/v1/models` ping 又拿不到实质信号，因此 2026-05-23 已移除。

## Sakrylle 定制与上游同步

这些定制都在 `theme/sakrylle`，每次 rebase 上游都容易冲突：

- 主题：从上游 4 个主题改为 2 个主题（`default-dark`、`light-cool`）。亮色主题使用 hue 256（Monet 薰衣草），不是上游 hue 210。
- 品牌："Sakrylle Status" 分布于 React i18n（zh/en/ru/ja 4 个 locale）和 Go SSR meta（`internal/api/meta.go` title/description/JSON-LD/404）。
- Logo：樱花 SVG 替代上游 RP 文字 logo。
- 语言切换：使用文字代码（ZH/EN/RU/JA），无国旗图标。
- 删除页面/组件：`ContactPage`、`OnboardingPage`、`ChangeRequestPage`、`Footer.tsx`、`useOnboarding`、`useChangeRequest`、`utils/share.ts`。rebase 后它们可能回来，需要重新删除。
- 状态表：隐藏 Model/Price/Listed-days 列；`MOBILE_ROW_HEIGHT=200`；`HIDDEN_ANNOTATION_IDS` 过滤 `frequency`、`public_service`、`key_type`、`sponsor_*`。
- Channel 解析：`parseChannelType` 对无前缀 channel 返回 `null`，上游返回 `'unknown'`。
- 主题迁移：`useTheme.ts` 含 `LEGACY_THEME_MIGRATION`，用于处理旧 localStorage。

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

## 常用运维命令

```bash
# 状态/日志/重启
ssh ssh-tokyo 'cd /opt/stack && docker compose ps relay-pulse'
ssh ssh-tokyo 'cd /opt/stack && docker compose logs --tail=100 relay-pulse'
ssh ssh-tokyo 'cd /opt/stack && docker compose restart relay-pulse'

# 强制拉最新镜像
ssh ssh-tokyo 'cd /opt/stack && docker compose pull relay-pulse && docker compose up -d --force-recreate relay-pulse'

# 查 monitor.db
ssh ssh-tokyo 'docker exec relay-pulse sqlite3 /data/monitor.db ".tables"'
ssh ssh-tokyo 'docker exec relay-pulse sqlite3 /data/monitor.db "SELECT * FROM events ORDER BY id DESC LIMIT 10"'

# 备份
ssh ssh-tokyo 'docker exec relay-pulse sqlite3 /data/monitor.db ".backup /tmp/monitor.db.bak" && docker cp relay-pulse:/tmp/monitor.db.bak /opt/stack/backups/relay-pulse-$(date +%F).db'

# 健康
curl -sI https://status.sakrylle.com/health
curl -s https://status.sakrylle.com/api/version | jq
```

看到红块时，按端到端链路验证：

```bash
# 1. relay-pulse 侧 probe_id
ssh ssh-tokyo 'docker logs --tail=200 relay-pulse 2>&1 | grep -E "claude-kiro-special|probe_id"'

# 2. sub2api 侧 usage_logs，key_id 在 sub2api 后台查
ssh ssh-tokyo 'docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT created_at, model_name, total_cost FROM usage_logs WHERE api_key_id=<id> ORDER BY id DESC LIMIT 5"'

# 3. 如果 relay-pulse 日志 200 但 usage_logs 空，优先怀疑静默假阳性
```

## 本地开发与校验

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

`make ci` 包含 gofmt、vet、go test 和 npm lint，代码变更完成后优先运行。

## API key 轮换

1. sub2api 后台 -> API Keys -> 重置 `relay-pulse-<channel>`，记下新值。
2. 修改 `/opt/stack/relay-pulse/.env` 中对应 `MONITOR_SAKRYLLE_<CHANNEL>_API_KEY=...`。
3. 运行 `docker compose up -d --force-recreate relay-pulse`，因为 env 不热更新。
4. 在 sub2api 后台撤销旧 key；Redis pub/sub 会自动失效，旧 key 缓存最长约 60s。

## 相关上下文

- 网关仓库：`/Volumes/APFS_HD/Documents/Github/sub2api`，分支 `theme/monet-purple`。它的 `CLAUDE.md` 包含上游 channel 映射、group 倍率、`/v1/models` 聚合逻辑。
- Obsidian 部署笔记：`20 Work/ServerOps/Self-Hosted/Sub2API 部署.md`。
- Cursor 场景规则见 `.cursor/rules/`；完整 AI 运维手册仍可参考 `CLAUDE.md`。
