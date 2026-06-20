# CLAUDE.md

Sakrylle Status fork of [prehisle/relay-pulse](https://github.com/prehisle/relay-pulse). 仅运维笔记 — 上游架构/开发流程读 upstream `README.md`。"改了什么" 走 `git diff upstream/main`。

Cursor 按场景规则见 `.cursor/rules/`（从本文件拆分）；完整手册仍以本文件为准。

## Repository

- Upstream: [prehisle/relay-pulse](https://github.com/prehisle/relay-pulse)
- Fork: [Ranshen1209/sakrylle-status](https://github.com/Ranshen1209/sakrylle-status), 分支 `theme/sakrylle`（所有定制都在这里）
- Image: `ghcr.io/ranshen1209/relay-pulse:sakrylle`
- 生产: `status.sakrylle.com`
- 服务器: `cliproxyapi-jp` (64.83.47.108, SSH 别名 `ssh-tokyo`)，compose stack 在 `/opt/stack/`

Sakrylle API 网关（`Ranshen1209/sub2api`，分支 `theme/monet-purple`）的伴生监测站。探针打的是网关上真实的 group，成本/排障与网关耦合。

## Build & deploy

### 代码改动 — 推 GHA → GHCR → 服务器拉

```bash
git push origin theme/sakrylle
gh run list -R Ranshen1209/sakrylle-status --limit=1   # 等 ~3 分钟
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

7 个探针，全部 `interval: 3m`，全部打 `https://api.sakrylle.com`，**每个 group 一把独立 API key**（sub2api `api_keys.group_id` 是单值）。

| Channel key | Service | Template | Model | Group (sub2api) | Rate |
|---|---|---|---|---|---|
| `claude-code-awsq` | `cc` | `cc-haiku-openai-chat` | `claude-haiku-4-5-20251001` | Claude-Code-AWSQ (id 12) | 0.4x |
| `claude-kiro` | `cc` | `cc-haiku-openai-chat` | `claude-haiku-4-5-20251001` | Claude-Kiro (id 15) | 0.9x |
| `claude-kiro-special` | `cc` | `cc-haiku-openai-chat` | `claude-haiku-4-5-20251001` | Claude-Kiro-Special (id 2) | 0.6x |
| `gpt-pro` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Pro (id 14, 号池) | 0.5x |
| `gpt-pro-special` | `cx` | `cx-gpt-mini-chat` | `gpt-5.4-mini` | GPT-Pro-Special (id 3, 旧名 GPT-Pro) | 0.4x |
| `deepseek-official` | `dx` | `dx-flash-openai-chat` | `deepseek-v4-flash` | Deepseek-Official (id 9, 官方直连) | 1.0x |
| `grok` | `gk` | `gk-grok-openai-chat` | `grok-4.20-0309-non-reasoning` | Grok-API (id 22) | 0.001x |

不监测（用户指定）：Claude-Max (id 16)、Claude-Max-C (id 17)。生图分组：GPT-Image (id 5)、GPT-Image-2-4K (id 11)、GPT-Image-2-Async (id 21) — 按调用计费，探针成本过高。已下架：Agnes-API (id 23)、Deepseek-Special (id 6 → 28)，见 2026-06-15 调整。

合计探针成本极小（haiku/mini/flash 单价 + grok envelope ping）。3m 节奏自 2026-05-26 收紧（原 9m）。`gk`/`ag` 是新 service code，前端 `ServiceIcon.tsx` 已加 Grok/Agnes 品牌图标（Agnes 探针虽下架，图标保留——无探测时不渲染，移除会徒增 rebase 冲突）。

**2026-06-04 重大调整**：sub2api 后台将旧 GPT-Pro (id 3) 重命名为 GPT-Pro-Special，新上线 GPT-Pro (id 14) 号池。relay-pulse 同步调整：`gpt-pro` 历史数据迁移至 `gpt-pro-special`，新增 `gpt-pro` 和 `claude-code-awsq` 两个探针。

**2026-06-13 调整**：下架 GPT-Plus (id 4) 探针，清除历史数据。探针数 7→6。下架 Claude-Kiro (id 2) 探针，清除全部历史数据。探针数 6→5。

**2026-06-14 重建**：清空 monitor.db 全部历史（不备份），探针扩至 9 个 —— 监测除 Claude-Max/Claude-Max-C/生图外全部 token 计费分组。新增 `claude-kiro` (id 15)、`claude-kiro-special` (id 2，sub2api 把旧 Claude-Kiro 重命名而来)、`grok` (id 22)、`agnes` (id 23)。sub2api 同期把 id 6 `Deepseek` 重命名为 `Deepseek-Special`。补回此前仅存服务器、未入库的 `dx-flash-openai-chat.json` 模板。探针数 5→9。

**2026-06-15 调整**：清空 monitor.db 全部历史（不备份），下架 `agnes` (id 23) 探针（从 `config.yaml` 删除 Agnes 块，热重载生效）。探针数 9→8。`ag-flash-openai-chat.json` 模板与前端 `ServiceIcon.tsx` 的 Agnes 图标保留未删；`.env` 里 `MONITOR_SAKRYLLE_AGNES_API_KEY` 成孤立行（无害，下次轮换清掉）。

**2026-06-15 调整（二）**：下架 `deepseek` (Deepseek-Special) 探针，仅清该 channel 历史（`DELETE ... WHERE channel='deepseek'`，未动 `deepseek-official`）。探针数 8→7。`.env` 里 `MONITOR_SAKRYLLE_DEEPSEEK_API_KEY` 成孤立行。**`deepseek-official` 不受影响，保留。**

下架原因（排查记录，别再绕）：Deepseek-Special 的 group 后台账号是 Krill 逆向号（`api.cdn-krill-ai.com/coding`，anthropic 平台）。**Krill 的 deepseek-v4-flash 默认带 `thinking` 推理块，而 sub2api 的 cc 转发器处理不了流里的 `thinking` block → 报 `upstream stream ended without response` (502)**。判定链：① 直连 Krill（绕过 sub2api）stream/non-stream 都 200，加 `thinking:{type:"disabled"}` 则只回干净 text → Krill 本身没坏；② 官方 Deepseek (`api.deepseek.com/anthropic`) 默认不出 thinking，走同一条 sub2api cc 路径稳定 200；③ 经 sub2api 打 Krill 必 502，与 `max_tokens`/prompt/模板无关。**坑：sub2api 后台「测试账号连接」是非流式直连，对 thinking 号会假阳性（显示 active/测试完成），但真实服务路径是流式、必挂。看到账号"绿"≠服务路径通。** 修复路径（未采纳，直接下架了）：sub2api 侧给该账号关 thinking（`extra` 当时为 `{}`，无开关），或换非 thinking 号。另注：group 28 有较紧的 RPM 限流，密集 curl 会 429。

### Retry / timeout

每个 monitor pin `retry: 3`（共 4 次尝试）+ `timeout: 30s`。模板默认 `retry: 0`，**monitor 优先级最高**（monitor > template > global，见 `internal/config/lifecycle.go`）。

`internal/monitor/probe.go` **超时不重试** — per-call deadline 与 retry-loop ctx 共享，`timeout` 是所有尝试的总预算。挑值：`timeout > slowest_real_call + sum(backoffs)`。

退避默认：`retry_base_delay=200ms`、`retry_max_delay=2s`、`retry_jitter=0.2`。

### API keys

放在 `/opt/stack/relay-pulse/.env`（mode 600），通过 compose 的 `env_file:` 注入。`config.yaml` 里用 `env_var_name:` 显式引用，覆盖自动名 `MONITOR_{PROVIDER}_{SERVICE}_{CHANNEL}_API_KEY`。

key 在 sub2api 后台建，命名 `relay-pulse-{channel}`，绑定对应 group。env 变量名约定：`MONITOR_SAKRYLLE_<CHANNEL>_API_KEY`。

孤立的 env 行（无 monitor 引用）无害，下次轮换时清掉。

## 已踩过的坑

### claude-kiro-special 必须用 OpenAI-compat 模板，不能用原生 cc-haiku-arith

上游 `cc-haiku-arith` 打 `/v1/messages` + claude-cli headers。在 Sakrylle 返回 200 ~15ms 但 model 字段为空 — sub2api `usage_logs` 24h 内**一行都没**。探针"成功"但根本没走计费链路 → 静默假阳性。

`cc-haiku-openai-chat` 是 `cx-gpt-mini-chat` 的 fork，POST `/v1/chat/completions` + Claude model id。sub2api 内部 OpenAI→Anthropic 翻译上行，产生真实 `usage_logs`（~2s，真实计费）。**不要"修"回原生模板。**

### Grok / Agnes 用 envelope 校验，不校验正文

`grok`/`agnes` 是逆向端点，chat-completions 路径**拿不到可读回复**：

- **Agnes** (`agnes-2.0-flash` / `agnes-1.5-flash`)：永远 `content=null`、`finish_reason=length`、固定 128 输出 token，**完全忽略 `max_tokens`**。prompt_tokens 固定 218（sub2api 注入大 system prompt）。
- **Grok** (`grok-4.20-0309-non-reasoning`)：能返回正文但算术不可靠（问 6+7 答 "7"）。

所以这俩模板不发算术题、不校验答案，改用 **envelope 校验**：`success_contains: "{{MODEL}}"` —— `{{MODEL}}` 在 `probe.go` 里会被替换成 request_model，匹配响应里回显的 `"model":"<id>"`。确认 200 + 正确路由 + 真实计费（usage_logs 有行），不依赖正文。两者均已实测产生真实 `usage_logs`，非静默假阳性。**别给它们换回算术模板。**

`grok-build-console`（挂牌最便宜）从未被真实调用、疑似非对话 console 模型，**别用**。Agnes `max_tokens` 调大无用（永远 128）。

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

# 查 monitor.db —— relay-pulse 镜像内没装 sqlite3，用临时 alpine 容器挂 volume 读
# 表：probe_history / status_events / channel_states / service_states / monitor_overrides（没有 events 表）
# probe_history 列：provider service channel model status sub_status latency timestamp(unix秒) error_detail http_code
ssh ssh-tokyo 'docker run --rm -v stack_relay-pulse-data:/data alpine sh -c "apk add -q sqlite && sqlite3 /data/monitor.db .tables"'
ssh ssh-tokyo 'docker run --rm -v stack_relay-pulse-data:/data alpine sh -c "apk add -q sqlite && sqlite3 -header -column /data/monitor.db \"SELECT channel,status,http_code,latency,timestamp FROM probe_history ORDER BY id DESC LIMIT 10;\""'

# 备份（同样用临时容器；挂 /opt/stack/backups 直接落盘，.backup 在线备份不锁库）
ssh ssh-tokyo 'docker run --rm -v stack_relay-pulse-data:/data -v /opt/stack/backups:/backup alpine sh -c "apk add -q sqlite && sqlite3 /data/monitor.db \".backup /backup/relay-pulse-\$(date +%F).db\""'

# 健康
curl -sI https://status.sakrylle.com/health
curl -s https://status.sakrylle.com/api/version | jq
```

### 探针端到端验证（看到红块时）

```bash
# 1. relay-pulse 侧 probe_id
ssh ssh-tokyo 'docker logs --tail=200 relay-pulse 2>&1 | grep -E "claude-kiro-special|probe_id"'

# 2. sub2api 侧 usage_logs（key_id 在 sub2api 后台查）
ssh ssh-tokyo 'docker exec sub2api-postgres psql -U sub2api -d sub2api -c \
  "SELECT created_at, model_name, total_cost FROM usage_logs WHERE api_key_id=<id> ORDER BY id DESC LIMIT 5"'

# 3. 日志 200 但 usage_logs 空 → 静默假阳性（见 claude-kiro-special 那节）
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
