# 知港 KnowPort

本地优先的 RAG 知识库与对话工作台。Go 后端 + React 前端 + Qdrant 向量库一体化打包，支持 Ollama 等本地模型，也可挂任意 OpenAI 兼容服务，全部数据落在本地磁盘。

---

## 特点

- **本地优先**：对话历史、知识库元数据、上传文件、向量索引全部存在 `backend/data/` 与 Qdrant 容器卷里，不依赖任何 SaaS。
- **多模型**：聊天 / Embedding 各自独立配置，可挂 Ollama、任意 OpenAI 兼容网关，或同时使用。
- **多用户**：自带账号体系（SQLite + bcrypt + JWT），每个用户独立的对话历史、知识库范围与模型 API Key；用户级 API Key 可选 AES-GCM 落盘加密。
- **MCP 支持**：内置 MCP server，可被外部 Agent 通过 `/mcp` 端点调用知识库检索能力。
- **混合检索**：稠密向量 + 稀疏向量（BM25 风格）RRF 融合，结构化文档（CSV/Excel）自动生成摘要 chunk 排在前位。
- **可选增强**：查询改写、多查询扩展、语义重排、上下文压缩、语义缓存，均以特性开关控制，默认关闭。
- **单二进制部署**：构建阶段把前端 `dist/` 嵌入到 Go 后端二进制，生产模式下后端同时托管 SPA 与 API。

---

## 技术栈

| 层       | 选型                                  | 说明                                                  |
| -------- | ------------------------------------- | ----------------------------------------------------- |
| 后端语言 | Go 1.25                               | 单二进制，模块化 service / handler / router           |
| Web 框架 | gin v1.9.1                            | + 自实现 requestID / accessLog / CORS 中间件          |
| 鉴权     | golang-jwt/jwt v5                     | access 15min + refresh 30 天，单 flight 续期          |
| 存储     | modernc.org/sqlite                    | 纯 Go SQLite（无 CGO），`auth.db` + `chat-history.db` |
| 文档解析 | ledongthuc/pdf, xuri/excelize         | PDF、Excel、CSV、Markdown 文本抽取                    |
| 向量库   | Qdrant v1.13.4                        | 通过 HTTP API 直连，无 SDK 依赖                       |
| 前端框架 | React 18 + Vite 5 + TypeScript 5      | SPA                                                   |
| 富文本   | react-markdown + remark-gfm + mermaid | 助手回复支持 Markdown / 代码块 / 图表                 |
| 模型 SDK | openai v4                             | 仅在前端用作 OpenAI 兼容侧的探活辅助                  |
| 部署     | Docker Compose                        | 3 容器：qdrant / backend / frontend(nginx)            |

---

## 开发启动（本地热重载）

适合日常调试：Qdrant 跑在容器里，后端 `go run`，前端 `vite dev`。

### 前置条件

- Go 1.25+
- Node.js 18+
- Docker（仅给 Qdrant 用）
- 任一种模型来源：
  - 本地 Ollama，且至少有一个 chat 模型（`qwen2.5:7b` / `qwen2.5:0.5b` 等）和一个 embedding 模型（`bge-m3`、`nomic-embed-text` 等）
  - 或任意 OpenAI 兼容服务的 base URL + key（启动后从前端设置面板填入）

### 一行启动

```bash
./dev.sh
```

`dev.sh` 行为：

1. 若主机上没有 Qdrant 容器在跑，自动 `docker compose up -d qdrant` 拉起。
2. 在后台 `go run .`（日志写到 `/tmp/knowport-backend.log`），等待 `/health` 通过。
3. 前台启动 `npm run dev --port 5174 --host 0.0.0.0`。
4. `Ctrl+C` 退出时自动 kill 后端进程；Qdrant 容器保留（数据不丢）。

启动完毕后访问：

- 前端：<http://localhost:5174>
- 后端 API：<http://localhost:8080>
- Qdrant：<http://localhost:6333>

### 环境变量（可选）

直接 `export` 或写到根目录 `.env`：

```bash
QDRANT_VECTOR_SIZE=1024          # 必须与 embedding 模型输出维度一致
OLLAMA_BASE_URL=http://localhost:11434
JWT_SECRET=please-change-in-prod
CONFIG_ENCRYPTION_KEY=           # 32 字节 base64，留空则用户 API Key 明文落盘
```

完整环境变量见 `backend/internal/config/config.go` 的 `LoadServerConfig`。

---

## 正式部署（Docker Compose）

适合放在自己的服务器或本机长期跑。

### 1. 准备 `.env`

```bash
cat > .env <<'EOF'
QDRANT_VECTOR_SIZE=1024
OLLAMA_BASE_URL=http://host.docker.internal:11434
QDRANT_API_KEY=
EOF
```

> `QDRANT_VECTOR_SIZE` 必须与你后续在前端设置面板里选的 embedding 模型一致。常见值：bge-m3 → 1024，nomic-embed-text → 768。

### 2. 本地构建镜像

```bash
./build.sh
```

`build.sh` 把 `BUILD_TIME` 与 `GIT_COMMIT` 注入构建参数，然后 `docker compose build`。

### 3. 起栈

```bash
docker compose up -d
```

容器对外端口：

| 服务     | 容器端口 → 主机端口 | 说明                                        |
| -------- | ------------------- | ------------------------------------------- |
| qdrant   | 6333 → 6333         | 向量库 HTTP API                             |
| backend  | 8080 → 8081         | Go 后端 + 嵌入式 SPA                        |
| frontend | 4173 → 5173         | Nginx 反代 `/api`、`/v1`、`/mcp` 到 backend |

完成后：

- 前端：<http://localhost:5173>
- 后端直连：<http://localhost:8081/health>

### 4. 数据持久化

- `./backend/data/` 经 bind mount 进容器，包含 SQLite 数据库与上传文件，**不要手动删**。
- Qdrant 用 named volume `qdrant_storage`，重建容器不丢；如需彻底清空：`docker compose down -v`。

### 5. 升级

```bash
git pull
./build.sh
docker compose up -d
```

---

## 项目目录

```
ai-localbase/
├── backend/                   Go 后端
│   ├── main.go                 进程入口；加载配置 / 初始化 service / 挂路由 / embed dist
│   ├── dist/                   嵌入式前端产物（构建时由前端 build 填充）
│   ├── data/                   运行时数据（大部分被 gitignore）
│   │   ├── app-state.json      全局应用状态（系统级默认 chat/embedding 配置等）
│   │   ├── auth.db             SQLite：用户、refresh token、用户级模型配置
│   │   ├── chat-history.db     SQLite：对话与消息
│   │   └── uploads/            原始上传文件
│   ├── eval/                   RAG 评估脚本（命令行工具）
│   └── internal/
│       ├── auth/               JWT 中间件、密码哈希、token 仓库
│       ├── config/             ServerConfig 加载与默认值
│       ├── handler/            HTTP handler：app_handler.go / auth_handler.go
│       ├── router/             路由组装 + 中间件
│       ├── service/            核心业务（见下方"模块"）
│       ├── mcp/                MCP server 与工具暴露
│       ├── model/              领域结构体
│       └── util/               文档抽取、ID 生成等
├── frontend/                  React 前端
│   └── src/
│       ├── App.tsx             根组件 + 全局状态机
│       ├── api/auth.ts         token 存储 + 401 单 flight 续期
│       ├── components/
│       │   ├── AuthGate.tsx
│       │   ├── LoginPage.tsx
│       │   ├── LandingPage.tsx
│       │   ├── Sidebar.tsx
│       │   ├── ChatArea.tsx
│       │   ├── brand/          "知港"品牌组件
│       │   ├── welcome/        登录后欢迎页
│       │   ├── knowledge/      知识库面板
│       │   └── settings/       设置抽屉（模型 / MCP / 高级）
│       ├── constants/          文案常量
│       └── index.css           全局样式（含磨砂背景实现）
├── docker/                    Dockerfile 与 nginx 配置
├── docker-compose.yml         生产 compose（唯一一份）
├── dev.sh                     本地开发启动脚本
├── build.sh                   本地镜像构建脚本
└── docs/                      学习与设计参考（部分文件本地保留，未推送）
```

---

## 模块介绍

### 后端 service（`backend/internal/service/`）

| 文件                    | 职责                                                                                                |
| ----------------------- | --------------------------------------------------------------------------------------------------- |
| `app_service.go`        | 应用层聚合：知识库 CRUD、文档上传/索引/删除、对话保存、模型配置探活、用户级 embedding/chat 配置解析 |
| `app_state_store.go`    | 全局 `app-state.json` 读写，含系统默认 chat/embedding 配置                                          |
| `user_store.go`         | 用户表 + 用户级模型配置（API Key 可选 AES-GCM 加密落盘）                                            |
| `chat_history_store.go` | 对话与消息的 SQLite 存储                                                                            |
| `llm_service.go`        | 上游 chat completion 调用（Ollama / OpenAI 兼容）                                                   |
| `rag_service.go`        | 文档切分、Embedding、Qdrant 写入/检索、RRF 融合、查询改写                                           |
| `model_probe.go`        | 模型可用性探活（前端"测试连接"用）                                                                  |

### 后端 handler / router

- `auth_handler.go`：`/api/auth/register | login | refresh | logout | me`
- `app_handler.go`：业务接口（详见 `router/router.go`），全部走 `RequireUser` JWT 中间件
- OpenAI 兼容端点：`/v1/chat/completions[/stream]`（也鉴权）
- MCP：`/mcp/*`（基于 `ENABLE_MCP` 开关）

### 前端组件

- `AuthGate.tsx`：未登录则渲染 `LandingPage` + `LoginPage`；已登录进入主工作台。
- `Sidebar.tsx`：会话列表、知识库列表、用户菜单、入口到设置抽屉。
- `ChatArea.tsx`：消息流、Markdown 渲染、知识库召回信息、流式响应。
- `settings/SettingsPanel.tsx`：模型配置 / MCP / 高级开关；带显式"保存"按钮与状态文案。
- `knowledge/`：知识库与文档面板（上传、删除、状态显示）。

---

## 知识库召回策略

完整链路：`PDF/Excel/CSV/MD/TXT → 文本抽取 → 切分 → 结构化摘要前置 → Embedding → Qdrant 入库 → 检索时 RRF 融合`。

### 1. 文本抽取（`internal/util`）

按扩展名分发：PDF（ledongthuc）、Excel（excelize）、CSV（自带表头扫描，输出"字段：xx。数据行数：n。"等结构化摘要行）、Markdown / TXT 直读。失败回退到文件名 + 文件大小占位文本，保证索引链路不中断。

### 2. 切分（`RagService.ChunkText` / `BuildDocumentChunks`）

- 默认 `CHUNK_SIZE=400`，`CHUNK_OVERLAP=80`（按 rune 计算，CJK 友好）。
- 优先在句末标点处切，避免词中切断。
- 对结构化文档（CSV / Excel）：先抽取"统计摘要 / 字段分布 / 文件汇总"行，生成 `kind=structured_summary` 的摘要 chunk，强制排在 chunk 列表最前位。
- 普通文档的第一个 chunk 也会标记为 `kind=summary`，便于检索时优先曝光开头。

### 3. Embedding（`RagService.EmbedTexts`）

- 用户级配置优先：`currentEmbeddingConfigFor(userID)` 从 user_store 读取，回退到全局默认。
- 调用 Ollama `/api/embeddings` 或 OpenAI 兼容的 `/embeddings`。
- 上游失败时回退到 `deterministicEmbedding`（基于哈希的伪向量，仅保证流程不挂；检索质量会下降，但能等模型恢复）。
- 向量维度 = `QDRANT_VECTOR_SIZE`，必须与 embedding 模型对齐。

### 4. 入库

- 每个知识库对应一个 Qdrant collection，命名 `kb_<knowledgeBaseID>`。
- 创建时按 `QDRANT_VECTOR_SIZE` + `QDRANT_DISTANCE`（默认 `cosine`）建集合。
- 写入 payload 字段：`chunk_id / text / document_id / document_name / knowledge_base_id / chunk_index / kind`。

### 5. 检索（`SearchHybrid` / `MultiQuerySearch`）

- **基线**：`searchDense` 用查询向量在 Qdrant 上做 `/points/search`。
- **混合检索（`ENABLE_HYBRID_SEARCH=true`）**：同时跑 dense + sparse（`BuildSparseVector`，BM25 风格，按 token 频次 + CJK 分词构造稀疏向量），按 RRF（Reciprocal Rank Fusion，`rrfFusion`）合并。
- **多查询扩展**：`MultiQuerySearch` 并发地用 N 条改写查询召回，去重后再融合，缓解原问题表述模糊导致召回偏窄的问题。
- **关键参数**：
  - `RETRIEVAL_TOPK_DOCUMENT=4` —— 单文档/单 KB 最终保留 chunk 数。
  - `RETRIEVAL_CANDIDATE_TOPK_DOCUMENT=8` —— 单文档召回候选数（fusion 前）。
  - `RETRIEVAL_TOPK_KNOWLEDGE_BASE=5` —— "全 KB"模式默认 topK。
  - `RETRIEVAL_CANDIDATE_TOPK_ALL_DOCS=10` —— 全 KB 召回候选数。
  - `RETRIEVAL_MAX_CHUNKS_PER_DOCUMENT=1` —— 防止单文档刷屏。
  - `RETRIEVAL_MAX_CONTEXT_CHARS=2400` —— 拼接进 prompt 的字符上限。

---

## 查询策略

### 1. 查询改写（`ENABLE_QUERY_REWRITE=true`）

- `NewLLMQueryRewriter` 用 chat 模型把原问题改写为 N 条等价/扩展查询（保留原句），列表化输出。
- 用于 `MultiQuerySearch` 的并发召回，去重后送入融合阶段。

### 2. 语义重排（`ENABLE_SEMANTIC_RERANKER=true`）

- 在 RRF 融合后，对 top-K 候选用 chat 模型做相关性打分重排（开销大，默认关）。

### 3. 上下文压缩（`ENABLE_CONTEXT_COMPRESSION=true`）

- 检索到的 chunk 在拼 prompt 前先用小模型做"抽取式压缩"，去掉与问题无关的句子，降低上下文消耗。

### 4. 语义缓存（`ENABLE_SEMANTIC_CACHE=true`）

- 用问题向量在内存 LRU 里查相似 QA 对，命中阈值内直接复用答案，跳过检索 + LLM 推理（适合 FAQ 类高频问题）。

### 5. 上下文拼装（`BuildContext`）

- 把命中 chunk 按文档分组、限制条数与字符上限，附带 `chunk_id` / `document_name` 供前端展示"召回来源"。
- 失败容忍：若检索完全失败，返回空上下文继续走纯对话，让链路不会因为向量库故障整体挂掉。

### 6. 自动扩展（`RETRIEVAL_ENABLE_AUTO_EXPAND=false`，默认关）

- 当首轮命中分数偏低时自动拓展窗口或追加 chunk，避免回答时上下文不足。

---

## 常见问题

### Q1：向量维度不匹配（`Vector dimension error: expected dim: 768, got 1024`）

Qdrant collection 创建时的维度与当前 embedding 模型输出维度不一致。修复：

1. 在前端设置面板确认 embedding 模型，或本地 `ollama list` 查看模型维度。
2. 修改 `.env` 的 `QDRANT_VECTOR_SIZE`（bge-m3 = 1024；nomic-embed-text = 768）。
3. 清旧数据后重启：
   ```bash
   docker compose down
   docker volume rm ai-localbase_qdrant_storage
   docker compose up -d
   ```

### Q2：容器内无法访问宿主 Ollama

宿主跑 Ollama，容器请求 `host.docker.internal` 失败。`docker-compose.yml` 里 backend 已配置：

```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
environment:
  OLLAMA_BASE_URL: http://host.docker.internal:11434
```

如果仍然失败，先在宿主验证 Ollama API 是否真的起来：

```bash
curl -v http://localhost:11434/v1/models
```

返回 200 + 模型列表才算正常。Linux 用户若 `host-gateway` 不可用，可手动给容器加路由或把 Ollama 改成监听 `0.0.0.0:11434`，然后用宿主 LAN IP 替换 `host.docker.internal`。

### Q3：上传文档后调用 embedding 模型没扣额度

历史遗留 bug：旧版本 `IndexDocument` 用全局默认 embedding 配置，忽略了用户在设置面板填的 base URL / key。已修复为按当前登录用户的 user-level 配置走（`currentEmbeddingConfigFor(userID)`）。如果仍遇到，确认：

1. 设置面板里 embedding 模型 base URL / key 已点 **保存**，看到 "已保存" 状态。
2. 后端日志能看到向你期望的 base URL 发请求。

### Q4：登录后白屏，console 没报错

绝大多数是 Vite dev server 长时间运行后 HMR 状态损坏。本地开发模式下：

```bash
# 重启前端 dev 进程
pkill -f 'vite' ; ./dev.sh
```

浏览器侧 `Ctrl+Shift+R` 强刷一次。生产模式（Docker）出现白屏通常是 Nginx 反代到 backend 不通，看 `docker compose logs frontend backend`。

### Q5：知识库 / 对话数据想换机器迁移

打包 `backend/data/` 与 Qdrant volume 即可：

```bash
docker compose down
tar czf knowport-data.tgz backend/data
docker run --rm -v ai-localbase_qdrant_storage:/src -v "$PWD":/dst alpine \
  tar czf /dst/knowport-qdrant.tgz -C /src .
```

新机器解压到对应位置后 `docker compose up -d` 即可恢复。

### Q6：完整重启 + 重置（最后手段）

```bash
docker compose down -v          # 清 qdrant volume
rm -f backend/data/app-state.json   # 清全局默认配置
rm -f backend/data/auth.db backend/data/chat-history.db   # 清用户与历史
docker compose up -d
```

注意此操作会清空全部数据。

### 调试速查

```bash
# 后端日志
docker compose logs backend --tail 100
# 前端 nginx 日志
docker compose logs frontend --tail 100
# Qdrant 健康
curl -s http://localhost:6333/healthz
# 后端健康
curl -s http://localhost:8081/health
```

---

## 许可证

MIT，沿用原仓库。详见 `LICENSE`。
