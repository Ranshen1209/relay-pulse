# CLAUDE.md

Sakrylle Status fork of [prehisle/relay-pulse](https://github.com/prehisle/relay-pulse). 仅运维笔记 — 上游架构/开发流程读 upstream `README.md`。"改了什么" 走 `git diff upstream/main`。

## Repository

- Upstream: [prehisle/relay-pulse](https://github.com/prehisle/relay-pulse)
- Fork: [Ranshen1209/relay-pulse](https://github.com/Ranshen1209/relay-pulse), 分支 `theme/sakrylle`（所有定制都在这里）
- Image: `ghcr.io/ranshen1209/relay-pulse:sakrylle`
- 生产: `status.sakrylle.com`
- 服务器: `cliproxyapi-jp` (64.83.47.108, SSH 别名 `ssh-tokyo`)，compose stack 在 `/opt/stack/`

Sakrylle API 网关（`Ranshen1209/sub2api`，分支 `theme/monet-purple`）的伴生监测站。探针打的是网关上真实的 group，成本/排障与网关耦合。

## Build & deploy

### 代码改动 — 推 GHA → GHCR → 服务器拉

```bash
git push origin theme/sakrylle
gh run list -R Ranshen1209/relay-pulse --limit=1   # 等 ~3 分钟
ssh ssh-tokyo 'cd /opt/stack && docker compose pull relay-pulse && docker compose up -d --force-recreate relay-pulse'
curl -sI https://status.sakrylle.com/health
```

### 仅 config.yaml 改动 — 热更新，无需重启

`config.yaml` 走 fsnotify 热重载（upstream `internal/config/watcher.go`）：

```bash
scp config/config.yaml ssh-tokyo:/opt/stack/relay-pulse/config/config.yaml
ssh ssh-tokyo 'docker logs --tail=20 relay-pulse 2>&1 | grep -i reload'
```

`.env` **不**热更新 — 进程启动时读。改完 `.env` 必须 `docker compose up -d --force-recreate relay-pulse`。

## 服务器布局

```
/opt/stack/
├── docker-compose.yml
└── relay-pulse/
    ├── .env                    # API keys (mode 600)，env_file: 注入
    ├── config/
    │   ├── config.yaml
    │   └── templates/          # 探测模板
    └── (volume) stack_relay-pulse-data → /data/monitor.db (SQLite)
```

反代链路：`Public 443 → sslh → 127.0.0.1:8443 → Nginx (server_name status.sakrylle.com) → relay-pulse:8080`。Nginx 配置 `/opt/stack/nginx/conf.d/sakrylle-status.conf`。

网络：`stack_default`（与 sub2api 系列共用）。

## Monitors（生产）

8 个探针，全部 `interval: 3m`，全部打 `https://api.sakrylle.com`，**每个 group 一把独立 API key**（sub2api `api_keys.group_id` 是单值）。

| Channel key | Service | Template | Model | Group |
|---|---|---|---|---|
| `claude-kiro` | `cc` | `cc-haiku-openai-chat` (fork) | `claude-haiku-4-5-20251001` | Claude-Kiro (id 2) |
| `claude-code` | `cc` | `cc-haiku-openai-chat` | `claude-haiku-4-5-20251001` | Claude-Code (id 7) |
| `claude-special` | `cc` | `cc-haiku-openai-chat` | `claude-haiku-4-5-20251001` | Claude-Special |
| `gpt-pro` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Pro (id 3) |
| `gpt-plus` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Plus (id 4) |
| `gpt-plus-special` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Plus-Special (@0.15x) |
| `deepseek` | `dx` | `dx-flash-openai-chat` | `deepseek-v4-flash` | Deepseek (id 6, 转售) |
| `deepseek-official` | `dx` | `dx-flash-openai-chat` | `deepseek-v4-flash` | Deepseek-Official (id 9, 官方直连) |

合计成本 ~$0.017/天（7 探针时对账 sub2api `usage_logs`；`gpt-plus-special` 于 2026-05-29 加入后未重新对账，实际略高）。3m 节奏自 2026-05-26 收紧（原 9m）。

### Retry / timeout

每个 monitor pin `retry: 3`（共 4 次尝试）+ `timeout: 30s`。模板默认 `retry: 0`，**monitor 优先级最高**（monitor > template > global，见 `internal/config/lifecycle.go`）。

`internal/monitor/probe.go` **超时不重试** — per-call deadline 与 retry-loop ctx 共享，`timeout` 是所有尝试的总预算。挑值：`timeout > slowest_real_call + sum(backoffs)`。

退避默认：`retry_base_delay=200ms`、`retry_max_delay=2s`、`retry_jitter=0.2`。

### API keys

放在 `/opt/stack/relay-pulse/.env`（mode 600），通过 compose 的 `env_file:` 注入。`config.yaml` 里用 `env_var_name:` 显式引用，覆盖自动名 `MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY`。

key 在 sub2api 后台建，命名 `relay-pulse-{channel}`，绑定对应 group。env 变量名约定：`MONITOR_SAKRYLLE_<CHANNEL>_API_KEY`。

孤立的 env 行（无 monitor 引用）无害，下次轮换时清掉。

## 已踩过的坑

### claude-kiro 必须用 OpenAI-compat 模板，不能用原生 cc-haiku-arith

上游 `cc-haiku-arith` 打 `/v1/messages` + claude-cli headers。在 Sakrylle 返回 200 ~15ms 但 model 字段为空 — sub2api `usage_logs` 24h 内**一行都没**。探针"成功"但根本没走计费链路 → 静默假阳性。

`cc-haiku-openai-chat` 是 `cx-gpt-mini-chat` 的 fork，POST `/v1/chat/completions` + Claude model id。sub2api 内部 OpenAI→Anthropic 翻译上行，产生真实 `usage_logs`（~2s，真实计费）。**不要"修"回原生模板。**

### Deepseek 探针走 OpenAI 路径 + max_tokens=8

`dx-flash-openai-chat.json` 是自建模板。`max_tokens: 8` 用来抑制 thinking-token 成本（deepseek 的 `thinking` block 不限制会爆）。Anthropic 路径也能通（sub2api 双向翻译），但计费语义不同 — 锁定 OpenAI 路径。

只探 `v4-flash`。`v4-pro` 上游同源，flash 通则 pro 通；pro 单价 6x，不值得 3m 一次。

### GPT-Image 已下线

按调用计费 ($0.15/call)。真实探针太贵。仅 `/v1/models` ping 拿不到实质信号。2026-05-23 移除。

### Sakrylle 主题 vs 上游 — rebase 冲突面

这些定制全在 `theme/sakrylle`，**每次 rebase 上游必冲突**：

- **主题**: 4 → 2（`default-dark`、`light-cool`）。亮色主题改用 hue 256（Monet 薰衣草），原 210。
- **品牌**: "Sakrylle Status" 散布于 React i18n（zh/en/ru/ja 4 个 locale）和 Go SSR meta（`internal/api/meta.go` title/description/JSON-LD/404）。
- **Logo**: 樱花 SVG 替代上游 RP 文字 logo。
- **语言切换**: 文字代码（ZH/EN/RU/JA），无国旗图标。
- **删除的页面/组件**: `ContactPage`、`OnboardingPage`、`ChangeRequestPage`、`Footer.tsx`、`useOnboarding`、`useChangeRequest`、`utils/share.ts`（rebase 后会回来 — 重新删）。
- **状态表**: 隐藏 Model/Price/Listed-days 列；`MOBILE_ROW_HEIGHT=200`；`HIDDEN_ANNOTATION_IDS` 过滤 frequency/public_service/key_type/sponsor_*。
- **Channel 解析**: `parseChannelType` 对无前缀 channel 返回 `null`（上游返回 `'unknown'`）。
- **主题迁移**: `useTheme.ts` 含 `LEGACY_THEME_MIGRATION` 处理旧 localStorage。

**冲突常客**：

- `frontend/src/i18n/locales/{zh,en,ru,ja}.json`
- `internal/api/meta.go`
- `frontend/src/styles/themes/*.css`
- `frontend/src/components/{Header,StatusTable,StatusCard,Footer,RefreshButton,ChannelTypeIcon}.tsx`
- `frontend/src/hooks/useTheme.ts`
- `frontend/src/router.tsx`
- `Dockerfile`

同步流程：

```bash
git fetch upstream && git checkout theme/sakrylle && git rebase upstream/main
# 重新删除已删页面、修主题色 hue、补品牌字符串。
make ci   # gofmt + vet + go test + npm lint
```

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
ssh ssh-tokyo 'docker exec relay-pulse sqlite3 /data/monitor.db ".backup /tmp/monitor.db.bak" && \
  docker cp relay-pulse:/tmp/monitor.db.bak /opt/stack/backups/relay-pulse-$(date +%F).db'

# 健康
curl -sI https://status.sakrylle.com/health
curl -s https://status.sakrylle.com/api/version | jq
```

### 探针端到端验证（看到红块时）

```bash
# 1. relay-pulse 侧 probe_id
ssh ssh-tokyo 'docker logs --tail=200 relay-pulse 2>&1 | grep -E "claude-kiro|probe_id"'

# 2. sub2api 侧 usage_logs（key_id 在 sub2api 后台查）
ssh ssh-tokyo 'docker exec sub2api-postgres psql -U sub2api -d sub2api -c \
  "SELECT created_at, model_name, total_cost FROM usage_logs WHERE api_key_id=<id> ORDER BY id DESC LIMIT 5"'

# 3. 日志 200 但 usage_logs 空 → 静默假阳性（见 claude-kiro 那节）
```

### 推前本地校验

```bash
go build -o /tmp/relay-pulse ./cmd/server && go vet ./...
go run ./cmd/verify/main.go -provider Sakrylle -service cc -v
```

### 轮换 API key

1. sub2api 后台 → API Keys → 重置 `relay-pulse-<channel>`，记下新值。
2. 改 `/opt/stack/relay-pulse/.env` 对应 `MONITOR_SAKRYLLE_<CHANNEL>_API_KEY=...`。
3. `docker compose up -d --force-recreate relay-pulse`（env 不热更）。
4. 旧 key 在 sub2api 后台撤销；Redis pub/sub 自动失效，旧 key 缓存 ≤60s。

## 本地开发（最小集）

```bash
./scripts/setup-dev.sh                      # 首次
./scripts/setup-dev.sh --rebuild-frontend   # 改完前端
make dev                                    # 后端热重载（air）
cd frontend && npm run dev                  # 前端 dev
make ci                                     # gofmt + vet + go test + npm lint
```

**Embed 陷阱**：源码在 `frontend/`，但 Go embed 需要 `internal/api/frontend/dist/`（已 `.gitignore`）。`setup-dev.sh --rebuild-frontend` 自动同步。**别直接改 `internal/api/frontend/`** — 不会被 git 追踪，下次构建丢失。

## Related

- 网关：`/Volumes/APFS_HD/Documents/Github/sub2api`（`theme/monet-purple`）。它的 `CLAUDE.md` 有上游 channel 映射、group 倍率、`/v1/models` 聚合逻辑。
- Obsidian 部署笔记：`20 Work/ServerOps/Self-Hosted/Sub2API 部署.md`。
