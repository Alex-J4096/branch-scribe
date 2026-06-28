# BranchScribe Development Log

本文档记录每轮实际完成的工程变更，作为 `ARCHITECTURE.md` 任务清单之外的执行流水。

## 2026-06-26

### Step 1: 建立任务追踪与开发日志

- 新增本开发日志，用于记录每次推进内容。
- 将 `ARCHITECTURE.md` 的阶段任务作为主任务清单维护。

### Step 2: 补齐数据库核心 schema

- 扩展初始化 SQL，启用 `pgcrypto`，用于 `gen_random_uuid()`。
- 新增 `backend/migrations/init/002_core_schema.sql`。
- 覆盖核心表：`projects`、`branches`、`blocks`、`block_revisions`、`graph_edges`、`canon_entities`、`memory_chunks`、`summary_snapshots`、`model_profiles`、`prompt_templates`、`generation_runs`。
- 添加核心外键、检查约束、查询索引和 `updated_at` 触发器。
- 更新 `ARCHITECTURE.md` 中已完成的数据库与 Docker 相关任务。

### Step 3: 搭建 Go 后端基础工程

- 新增 `backend/go.mod`，初始化 Go module。
- 选择 Gin 作为 HTTP 框架，pgxpool 作为 PostgreSQL 连接池。
- 新增配置加载模块，支持 `.env`、`DATABASE_URL` 和 Docker compose 使用的 `POSTGRES_*` 环境变量。
- 新增统一 API 响应结构 `{ "data": ..., "error": ... }`。
- 新增 request id middleware、CORS middleware 和 `/health`、`/api/health` 健康检查。
- 新增 `.env.example`、`.gitignore` 和 README 基础启动说明。
- 更新 `ARCHITECTURE.md` 中已完成的 Phase 0 后端与 DevOps 任务。

### Step 4: 后端依赖整理与基础验证

- 运行 `go mod tidy`，下载 Gin、pgx 及其间接依赖，生成 `go.sum`。
- 运行 `go test ./...`，当前后端包均编译通过。
- 本机未安装 `psql`，改用 Docker 容器内的 `psql` 执行初始化 SQL。
- 在 `branchscribe-postgres` 中成功执行 `001_init_extensions.sql` 和 `002_core_schema.sql`。
- 修正配置加载路径，支持从仓库根目录或 `backend` 目录启动后端时读取 `.env`。
- 启动后端并调用 `GET /health`，确认返回 `{"data":{"status":"ok"},"error":null}`。

### Step 5: 实现 Project CRUD

- 新增 Project repository 和 HTTP handler。
- 实现 `GET /api/projects`、`POST /api/projects`、`GET /api/projects/:projectId`、`PATCH /api/projects/:projectId`、`DELETE /api/projects/:projectId`。
- 创建 project 时在同一事务中自动创建默认 branch：`主线`。
- 将 Gin 启动方式改为 `http.Server`，支持收到退出信号后优雅关闭。
- 更新 `ARCHITECTURE.md` 中 Project Service 和默认 branch 创建任务。
- 运行 `go test ./...`，后端编译通过。
- 通过真实 API 创建测试项目，读取项目详情，确认数据库中自动创建 `主线` branch，再通过 DELETE API 清理测试项目。

### Step 6: 实现 Branch CRUD

- 新增 Branch repository 和 HTTP handler。
- 实现 `GET /api/projects/:projectId/branches`、`POST /api/projects/:projectId/branches`、`POST /api/projects/:projectId/branches/fork`、`PATCH /api/branches/:branchId`、`DELETE /api/branches/:branchId`。
- 支持 branch 状态更新：`active`、`archived`。
- 更新 `ARCHITECTURE.md` 中 Branch Service 任务。

### Step 7: 实现 Block、Revision 与 Graph API

- 新增 Block repository 和 HTTP handler。
- 实现 `GET /api/projects/:projectId/blocks`、`POST /api/projects/:projectId/blocks`、`GET /api/blocks/:blockId`、`PATCH /api/blocks/:blockId`、`DELETE /api/blocks/:blockId`、`POST /api/blocks/:blockId/fork`。
- 创建 block 时在同一事务中创建初始 revision，并设置为 `current_revision_id`。
- 新增 revision API：`GET /api/blocks/:blockId/revisions`、`POST /api/blocks/:blockId/revisions`、`GET /api/revisions/:revisionId`、`POST /api/blocks/:blockId/revisions/:revisionId/select`。
- 保存 revision 时生成 `content_hash`。
- 新增 Graph repository 和 HTTP handler。
- 实现 `GET /api/projects/:projectId/graph`、`POST /api/projects/:projectId/graph/edges`、`PATCH /api/projects/:projectId/graph/nodes/:blockId/position`、`DELETE /api/projects/:projectId/graph/edges/:edgeId`。
- 更新 `ARCHITECTURE.md` 中 Block、Revision、Graph 和 content_hash 相关任务。

### Step 8: Phase 1 后端核心链路验证

- 运行 `go test ./...`，后端所有包编译通过。
- 启动后端后，通过真实 API 完成以下冒烟链路：
  - 创建 project。
  - 读取自动创建的默认 branch。
  - 创建 block，并确认初始 revision 创建成功。
  - 创建第二个 revision，并设置为当前版本。
  - 选择初始 revision 作为当前版本。
  - fork block，并确认 graph 中出现 2 个节点和 1 条 `fork` 边。
  - 更新 fork 节点位置。
  - 删除 graph edge。
  - 删除测试 project，清理测试数据。

### Step 9: 初始化前端工程

- 新增 `frontend` Vite 工程。
- 添加 Vue 3、TypeScript、Vue Router、Pinia、TanStack Query for Vue、Vue Flow 和 lucide 图标依赖。
- 新增前端环境变量示例：`frontend/.env.example`。
- 新增前端 API client 和集中类型定义，对齐后端 `{ data, error }` 响应格式。
- 新增 Pinia workspace store，用于维护当前选中的 block。

### Step 10: 实现前端 MVP 工作台

- 新增 `ProjectList` 页面，支持项目列表、创建项目、删除项目和进入工作台。
- 新增 `ProjectWorkspace` 页面，采用左侧分支与创建 block、中间图画布、右侧 block inspector 的三栏布局。
- 集成 Vue Flow 展示 block node 和 graph edge。
- 支持点击节点选择 block、拖动节点后保存位置。
- 新增 Block Inspector，支持查看当前正文、保存新 revision、选择历史 revision 和 fork block。
- 新增前端基础样式，面向写作 IDE 的密集工作台界面。
- 运行 `npm run typecheck`，前端类型检查通过。
- 更新 `ARCHITECTURE.md` 中 Phase 0 前端任务和 Phase 1 前端/验收任务。

### Step 11: 前端构建与本地启动验证

- 修正 Vite 配置，启用 `vue()` 插件并添加 `@ -> src` 路径别名。
- 运行 `npm run build`，前端生产构建通过。
- 启动后端服务，确认 `GET /health` 返回 `{"data":{"status":"ok"},"error":null}`。
- 启动 Vite dev server，确认 `http://localhost:5173/` 可访问。

### Step 12: 补齐 Phase 1 前端交互

- 将 block `title` 调整为可选字段，支持无标题片段；数据库 schema、后端模型和前端类型同步改为允许空标题。
- 同步修复 graph 查询的 block node 扫描逻辑，允许 graph 返回 `title: null` 的节点。
- 前端无标题 block 在图节点和详情面板中使用自动显示名，创建 block 时标题输入改为可选。
- 将新建项目表单移动到弹窗中，完成 Phase 1 的创建项目弹窗任务。
- 在项目工作台左侧新增 edge 创建表单，支持选择起点、终点、关系类型和可选标签。
- 对运行中的 `branchscribe-postgres` 执行 `ALTER TABLE blocks ALTER COLUMN title DROP NOT NULL;`，同步已初始化数据库。
- 运行 `go test ./...`、`npm run typecheck`、`npm run build`，均通过。
- 通过本地 API 冒烟验证：创建两个无标题 block，创建一条 `next` edge，graph 返回 2 个节点和 1 条边。
- 更新 `ARCHITECTURE.md` 中 block title 说明、Phase 1 前端任务清单和验收清单。

### Step 13: 增加图上拖拽吸附连线

- 在 Vue Flow block 节点左右两侧加入 target/source 连接点。
- 接入 Vue Flow `connect` 事件，用户从 source 拖到另一个 block 的 target 时自动创建默认 `next` edge。
- 增加前端连接校验，禁止连接到自身，并避免重复创建同向 `next` edge。
- 保留左侧 edge 表单作为备用入口，用于创建指定类型和标签的 edge。
- 运行 `npm run typecheck` 和 `npm run build`，均通过。
- 更新 `ARCHITECTURE.md` 中 Phase 1 创建 edge 任务和验收描述。

### Step 14: 修复拖拽吸附连线可用性

- 扩大 block 节点右侧连接热区，用户可以从节点右边缘拖拽，而不需要精准拖中小圆点。
- 增加自定义 pointer 连线逻辑，松手时如果落在目标 block 上或靠近目标 block 左侧，会自动吸附并创建默认 `next` edge。
- 增加拖拽过程中的全局预览线，提升连接操作反馈。
- 保留 Vue Flow handle 和 `connect` 事件作为底层兼容路径，左侧 edge 表单继续作为备用方案。
- 运行 `npm run typecheck` 和 `npm run build`，均通过。
- 更新 `ARCHITECTURE.md` 中 Phase 1 edge 创建任务和验收描述。

### Step 15: 增加 Block 列表管理

- 在工作台左侧新增 Block 列表，按 graph nodes 展示所有 block。
- 列表行显示自动标题、block 类型、入边数量和出边数量，方便快速扫读结构。
- 点击列表项可以快速选中 block，并在右侧 inspector 打开详情。
- 新增前端 `deleteBlock` API client 方法，并在列表中提供删除按钮；删除当前选中 block 时会自动清空选择。
- 保留画布作为结构视图，列表作为定位和删除的管理视图。
- 运行 `npm run typecheck` 和 `npm run build`，均通过。
- 更新 `ARCHITECTURE.md` 中 Phase 1 前端任务和验收清单。

### Step 16: 接入 Tiptap 富文本正文编辑器

- 安装 `@tiptap/vue-3` 和 `@tiptap/starter-kit`。
- 新增 `RichTextEditor` 组件，提供粗体、斜体、二级标题、无序列表、有序列表、撤销和重做工具栏。
- 将 block inspector 中的 textarea 替换为 Tiptap 编辑器。
- 新保存的 revision 使用 `html` content format；旧的 markdown/text 内容会作为普通段落载入，避免内容丢失。
- 保持现有手动保存链路，每次点击保存都会创建新的 current revision。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 对 Tiptap 相关 bundle 给出体积警告，功能不受影响。
- 更新 `ARCHITECTURE.md` 中 Phase 2 的 Tiptap、正文编辑器、手动保存和对应验收项。

### Step 17: 增加标题编辑、字数统计和 revision 状态

- 在 block inspector 中新增可选标题编辑表单，复用后端 `PATCH /api/blocks/:blockId`。
- 正文编辑区显示实时字数统计，中文按字计数，英文和数字按词计数。
- 正文编辑区显示当前内容是否未保存，便于区分草稿和 current revision。
- 新增 revision 状态条，展示当前 revision 的 content format、content hash 短码和创建信息。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 2 的标题编辑、字数统计和当前 revision 状态任务。

### Step 18: 增加 revision diff viewer

- 在历史版本区域新增旧版本/新版本两个选择器，支持选择任意两个 revision 对比。
- 新增前端文本 diff viewer，先将 HTML/文本内容转为纯文本，再按中文字符、英文词、数字和标点做 LCS diff。
- diff viewer 用绿色标识新增内容，用红色删除线标识删除内容。
- 保留原有点击历史版本恢复为 current revision 的交互，作为 Phase 2 rollback 路径。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 2 的 diff viewer、双 revision 对比、rollback 和对应验收项。

### Step 19: 增加本地草稿自动保存

- 在 block inspector 中增加本地草稿自动保存，编辑正文后 600ms 自动写入 `localStorage`。
- 草稿按 project/block 隔离，并记录 base revision，避免旧草稿覆盖新的 current revision。
- 打开 block 时如果存在同 base revision 的未保存草稿，会自动恢复并显示草稿状态。
- 保存 revision、选择历史 revision 回滚或点击“丢弃草稿”时，会清除对应本地草稿。
- 同步确认后端 revision `metadata` 和 `source` 已在创建 revision 链路中落库。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 2 的自动保存草稿、metadata、source 和验收状态。

### Step 20: 实现 Model Profile 后端 CRUD

- 新增 `backend/internal/modelprofile` 包，实现 Model Profile 的 list/create/get/update/delete。
- 新增 API 路由：
  - `GET /api/projects/:projectId/model-profiles`
  - `POST /api/projects/:projectId/model-profiles`
  - `GET /api/model-profiles/:profileId`
  - `PATCH /api/model-profiles/:profileId`
  - `DELETE /api/model-profiles/:profileId`
- Model Profile 响应只返回 `has_api_key`，不返回 `api_key` 或 `api_key_ref` 明文。
- 支持配置 provider、base_url、model、temperature、top_p、max_tokens、context_window 和 metadata。
- 运行 `go test ./...`，后端测试通过。
- 通过本地 API 冒烟验证：创建带 api_key 的 model profile，响应中 `has_api_key=true` 且没有 `api_key` 字段。
- 更新 `ARCHITECTURE.md` 中 Phase 3 的 Model Profile CRUD 和 API key 不回传任务。

### Step 21: 实现模型配置页面

- 新增 `ModelProfileSettings` 页面，并添加 `/projects/:projectId/model-profiles` 路由。
- 工作台顶栏新增“模型”入口，可以从项目工作台进入模型配置页面。
- 前端新增 Model Profile 类型和 API client 方法，支持 list/create/update/delete。
- 模型配置页面支持配置 provider、base_url、api_key、model、temperature、top_p、max_tokens 和 context_window。
- API key 输入只用于写入；读取列表和编辑已有配置时只显示是否已配置，不显示明文。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 3 模型配置页面和参数配置任务。

### Step 22: 修复富文本转义并调整模型配置界面

- 修复 Tiptap 编辑器输入后被再次按 markdown 归一化的问题；当内容已经是 HTML 时不再转义，从而避免正文变成可见 HTML 转义符。
- 将模型配置页调整为更接近 Cherry Studio 的设置页结构：左侧 profile 列表，右侧按 Provider 和 Generation 分组编辑。
- Profile 列表显示 provider、model 和 API key 状态，不显示 API key 明文。
- Generation 参数改为滑条和数字输入并排，覆盖 temperature、top_p、max_tokens 和 context_window。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 重新启动 Vite dev server，`http://localhost:5173/` 可访问。

### Step 23: 将工作台左右菜单改为抽屉

- 将项目工作台左右侧栏改为抽屉式布局，左右菜单都支持一键收起和展开。
- 左侧抽屉中的分支、新建 Block、Block 列表和备用创建 Edge 表单都支持独立收起。
- 右侧抽屉中的 block inspector 支持整体收起，内部标题、正文、Fork 和历史版本功能组也支持独立收起。
- 收起左右抽屉后画布自动扩展，便于在图上管理 block 和拖拽连接。
- 运行 `npm run typecheck`、`npm run build` 和 `go test ./...`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 1 的抽屉式菜单和功能组折叠任务。

### Step 24: 实现 Prompt Template CRUD

- 新增 `backend/internal/prompttemplate` 包，实现 Prompt Template 的 list/create/get/update/delete。
- 新增 API 路由：
  - `GET /api/projects/:projectId/prompt-templates`
  - `POST /api/projects/:projectId/prompt-templates`
  - `GET /api/prompt-templates/:templateId`
  - `PATCH /api/prompt-templates/:templateId`
  - `DELETE /api/prompt-templates/:templateId`
- 列表接口支持通过 `task_type` query 参数过滤。
- 创建或更新默认模板时，会取消同项目同 `task_type` 下其他模板的默认状态，避免默认模板歧义。
- 前端 API client 新增 Prompt Template 类型和 list/create/get/update/delete 方法，供后续生成界面复用。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 3 的 Prompt Template CRUD 任务和 API 路由列表。

### Step 25: 实现 OpenAI-compatible GenerateOnce

- 新增 `backend/internal/generation` 包，实现一次性非流式生成链路。
- 新增 `POST /api/generate/once` API，输入 project、block、task_type、model_profile 和可选 prompt_template。
- 实现 OpenAI-compatible provider，按 Chat Completions 兼容格式请求 `POST {base_url}/chat/completions`，发送 messages、temperature、top_p 和 max_tokens。
- 生成前读取 Model Profile、当前 block 的 current revision 和 Prompt Template；未指定模板时优先使用同 task_type 默认模板，否则使用内置任务模板。
- 内置模板覆盖 continue、rewrite_block、rewrite_selection、expand、condense 和 polish。
- 每次调用都会创建 `generation_runs` 记录，成功后写入 succeeded、token usage 和 latency，失败后写入 failed 和 provider 错误信息。
- LLM 请求使用 60 秒 timeout，provider 非 2xx 或异常响应会转成清晰错误。
- 前端 API client 新增 GenerateOnce 类型和 `generateOnce` 方法，供后续 inspector 操作面板复用。
- 为 OpenAI-compatible provider 增加成功响应和 provider 错误解析单元测试。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Go provider 测试因 `httptest` 需要监听本地端口，使用提升权限运行；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 3 的 provider、GenerateOnce、generation run、任务类型、timeout 和 provider error 任务。

### Step 26: 增加 SiliconFlow Provider 选项

- 在 Model Profile 的 provider 枚举中加入 `siliconflow`。
- 更新数据库初始化 schema 中的 `model_profiles_provider_check` 约束。
- 更新运行中的 `branchscribe-postgres` 数据库约束，允许保存 SiliconFlow provider。
- 模型配置页面的 Provider 下拉框新增 SiliconFlow。
- 模型配置页面根据所选 Provider 自动预填默认 Base URL；已有自定义 Base URL 时不会覆盖。
- GenerateOnce 将 `siliconflow` 作为 OpenAI-compatible provider 处理，复用现有 Chat Completions 兼容调用链路。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- Base URL 自动预填改动再次运行 `npm run typecheck` 和 `npm run build`，均通过。
- 更新 `ARCHITECTURE.md` 中服务商范围和 MVP Provider 支持项。

### Step 27: 接入 Block Inspector LLM 操作面板

- 在 Block Inspector 中新增可折叠的 LLM 操作面板，支持选择 Model Profile、选择任务类型、填写用户指令并触发一次性生成。
- 任务按钮覆盖续写、改写、局部改写、扩写、缩写和润色；局部改写暂时提供手动粘贴选中文本的输入框。
- 生成结果先在 inspector 中预览，不会直接覆盖正文。
- 保存生成结果时创建 `source=llm` 的新 revision，并写入 `generation_run_id` 和任务 metadata。
- 后端创建带 `generation_run_id` 的 revision 时，会回填 `generation_runs.output_revision_id`，保证调用记录和产出 revision 可追溯。
- 续写任务保存时会追加到当前草稿正文后；其他任务保存时使用生成结果作为新 revision 正文。
- 运行 `npm run typecheck`、`npm run build` 和 `go test ./...`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 3 的 inspector LLM 按钮、用户指令输入、生成结果保存和 LLM 输出保存为 revision 任务。

### Step 28: 增加自由生成任务

- 新增 LLM 任务类型 `free_write`，前端显示为“自由生成”。
- `free_write` 的内置 prompt 只使用项目简介和用户指令，不引用当前 block 正文。
- 后端为 `free_write` 使用轻量 block metadata context，只读取项目描述和 block 标题，不读取 current revision 内容。
- 保存 `free_write` 结果时与改写类任务一致，生成内容会作为新的 `llm` revision 正文。
- 更新前端 Prompt Template 类型，允许 `free_write` task_type。
- 更新 `ARCHITECTURE.md` 中 LLM 任务类型说明和 Phase 3 任务清单。

### Step 29: 修复 Graph Edge 可见性与拖拽连接

- 修复 block 节点 handle 被 CSS `pointer-events: none` 禁用的问题，恢复 Vue Flow 原生连接点交互。
- 为 Vue Flow edge 明确设置 `smoothstep` 类型、source/target handle、箭头 marker、线条宽度和 label 样式。
- 补充不同 edge_type 的可见颜色，菜单创建 edge 后能在画布上明确看到连接线、箭头和标签。
- 修复自定义拖拽吸附逻辑中松手查找目标时丢失 source block 的问题，避免最近目标误判。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 1 graph edge 可见性任务。

### Step 30: 修复拖动 Block 后 Edge 消失

- 将 `BlockGraph` 的 Vue Flow nodes/edges 从 computed prop 改为本地 `v-model:nodes` 和 `v-model:edges` 状态。
- 拖动节点时 Vue Flow 现在会直接更新本地 node 状态，不再与每次渲染重新生成的 computed nodes/edges 互相覆盖。
- 外部 graph 数据刷新时仍会同步回本地 nodes/edges，保留后端位置持久化后的最终状态。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。

### Step 31: 修复 Block 间连接箭头不可见

- 定位到根因：`BlockGraph.vue` 的 `isValidConnection` 直接复用了 `canCreateNextConnection`，它会拒绝任何已存在同向 `next` edge 的连接。
- Vue Flow 在 `setEdges`（加载后端已有 edge）阶段也会调用 `isValidConnection`，于是每条从后端回来的 `next` edge 都被自己判为重复连接，触发 `EDGE_INVALID`，画布上完全渲染不出箭头，控制台只留下 `An edge needs a source and a target` 警告。
- 重写 `isValidConnection`：先用 `isEdge(connection)` 区分“带 id 的既有 edge”和“新建拖拽 Connection”；既有 edge 一律放行，只有真正的拖拽才走 `canCreateNextConnection` 的重复校验，保留新建连线的去重逻辑。
- 同步将 `markerEnd` 从默认的 `MarkerType.ArrowClosed` 改为带颜色对象，箭头填色与线条颜色一致，避免 Vue Flow 默认箭头使用灰色 `#b1b1b7`。
- 通过 headless Chrome 连接 dev server 验证：修复前 `.vue-flow__edge` 元素数为 0、控制台报 `An edge needs a source and a target`；修复后画布上能看到一条 `vue-flow__edge-smoothstep` 路径、`next` 标签和 `#2f7d76` 颜色的闭合箭头。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。

### Step 32: 实现 LLM 流式生成与 SSE 输出

- 扩展 OpenAI-compatible provider，新增 `GenerateStream`，按 Chat Completions SSE 格式发送 `stream=true` 请求并解析 `delta`、`usage` 和 `[DONE]`。
- 新增 `POST /api/generate/stream`，生成开始前创建 `generation_runs` running 记录，流式结束后更新为 succeeded，失败时更新为 failed。
- SSE 输出统一发送 JSON 事件：`delta`、`done`、`error`；`done` 事件返回 generation run、prompt、model profile 和 prompt template 信息，供前端保存 revision 时追溯。
- 前端 API client 新增 `generateStream`，使用 `fetch` + `ReadableStream` 解析 SSE。
- Block Inspector 的生成按钮改为流式生成，生成文本会增量显示，完成后才允许保存为新的 `llm` revision；生成中支持取消当前请求。
- 为 OpenAI-compatible provider 增加流式响应单元测试，覆盖 delta 拼接和 token usage 解析。
- 运行 `npm run typecheck`、`npm run build` 和 `go test ./...`，均通过；Go provider 测试因 `httptest` 需要监听本地端口，使用提升权限运行；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 3 的 GenerateStream、SSE 流式输出和前端流式显示任务。

### Step 33: 修复前端 SSE 事件解析

- 修复流式生成时报 `Unexpected non-whitespace character after JSON` 的问题。
- 根因是前端 SSE 解析按 chunk 做整体拆分，遇到 CRLF 空行分隔、多个事件粘在同一 chunk 或换行符落在 chunk 边界时，可能把多条 `data:` 拼成一个 JSON 解析。
- 将事件解析改为行状态机：逐行收集 `data:`，遇到空行才解析并派发一个 SSE event。
- 支持 `\n`、`\r\n` 和 `\r` 换行，并允许 `data:` 行前存在空白。
- 解析失败时返回更明确的 `INVALID_STREAM_EVENT` 错误，便于定位异常 payload。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。

### Step 34: 重启后端使流式生成路由生效

- 定位到浏览器控制台 `POST /api/generate/stream 404` 的根因：8080 端口运行的是旧后端进程，尚未加载 Step 32 新增的流式生成路由。
- 停止旧的 `server` 进程，并用当前代码重新启动后端。
- 启动日志确认已注册 `POST /api/generate/stream`。
- 使用空 JSON 请求验证 `/api/generate/stream` 返回 `400 INVALID_GENERATION_REQUEST` 而不是 404，说明路由已生效。

### Step 35: 移除流式生成 60 秒硬超时

- 修复流式生成较慢时被 `context deadline exceeded` 强制截断的问题。
- 移除 `GenerateStream` handler 中包裹 provider 请求的 `context.WithTimeout(60s)`，流式生成现在随浏览器请求取消或连接断开而停止。
- 移除 OpenAI-compatible provider 的 `http.Client{Timeout: 60s}` 全局超时，避免流式响应被 HTTP client 截断。
- 非流式 `GenerateOnce` 仍保留 handler 层 60 秒 timeout，用于普通一次性请求的错误控制。
- 运行 `go test ./...`，后端测试通过；Go provider 测试因 `httptest` 需要监听本地端口，使用提升权限运行。
- 重启后端以加载本次超时修复。

### Step 36: 支持选中文本执行局部改写

- `RichTextEditor` 新增选区捕获能力，Tiptap selection 变化时会把当前选中文本传给 Block Inspector。
- `RichTextEditor` 暴露 `replaceSelectionWithHTML`，用于把 LLM 生成结果替换回上次正文选区。
- Block Inspector 的 `rewrite_selection` 任务自动使用正文编辑器中的选中文本作为 `selected_text`，没有选区时禁止生成并提示用户。
- `rewrite_selection` 生成完成后，保存按钮改为“替换选区并保存”，会先替换编辑器选区，再创建新的 `source=llm` revision。
- 切换 block 时会清理选区和生成状态，避免旧选区串到新 block。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 3 的局部选中文本任务和验收项。

### Step 37: Phase 3 API Key 改为环境变量引用

- 完成 Phase 3 安全项：MVP 阶段不再把 API key 明文保存到数据库。
- Model Profile 写入 API key 时只接受环境变量名，并保存为 `env:<VAR_NAME>` 形式的 `api_key_ref`。
- 生成请求读取 Model Profile 时会解析 `env:<VAR_NAME>` 并从进程环境变量中读取真实 API key；缺失时返回清晰的 invalid generation request 错误。
- Model Profile 列表和详情只有在 `api_key_ref` 是 `env:` 引用时才显示 `has_api_key=true`，旧明文遗留值不会再被当作有效配置。
- 前端模型配置页将 API key 输入改为“API key 环境变量”，并更新 `.env.example` 示例。
- 初始化 schema 增加 `model_profiles_api_key_ref_check`，限制 `api_key_ref` 只能为空或 `env:` 引用。
- 同步运行中的 `branchscribe-postgres`：清理 1 条旧的非 env API key 引用，并添加 `model_profiles_api_key_ref_check` 约束。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Go provider 测试因 `httptest` 需要监听本地端口，使用提升权限运行；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md`，Phase 3 所有任务和验收项已完成。

### Step 38: Phase 4 启动 Canon Entity CRUD

- 新增 `backend/internal/canon` 包，实现 Canon Entity 的 list/create/get/update/delete。
- 新增 API 路由：
  - `GET /api/projects/:projectId/canon`
  - `POST /api/projects/:projectId/canon`
  - `GET /api/canon/:entityId`
  - `PATCH /api/canon/:entityId`
  - `DELETE /api/canon/:entityId`
- 支持 entity 类型：`character`、`location`、`faction`、`item`、`rule`、`event`。
- 列表接口支持按 `type`、`status` 和 `q` 过滤；`q` 会匹配 name、description 和 aliases。
- 支持 aliases 去重、attributes JSON、importance 1-10 和 status：`canon`、`draft`、`deprecated`。
- 前端 API client 新增 Canon Entity 类型和 CRUD 方法，供后续角色/地点/世界规则页面复用。
- 重启后端后，通过真实 API 冒烟验证：创建临时 project，创建 character canon entity，按 type/q 查询，读取详情，更新 status/importance，删除 entity，删除测试 project。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Go provider 测试因 `httptest` 需要监听本地端口，使用提升权限运行；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的 Canon Entity CRUD、entity 类型和 canon entity keyword search 任务。

### Step 39: 实现 Memory Chunk CRUD

- 新增 `backend/internal/memory` 包，实现 Memory Chunk 的 list/create/get/update/delete。
- 新增 API 路由：
  - `GET /api/projects/:projectId/memory`
  - `POST /api/projects/:projectId/memory`
  - `GET /api/memory/:memoryId`
  - `PATCH /api/memory/:memoryId`
  - `DELETE /api/memory/:memoryId`
- 列表接口支持按 `source_type`、`chunk_kind`、`tag` 和 `q` 过滤；`q` 会匹配 `chunk_text`。
- 支持手动创建记忆 chunk，保存 `source_type`、可选 `source_id`、`chunk_text`、`chunk_kind`、去重后的 `tags` 和 `metadata` JSON。
- 前端 API client 新增 `MemoryChunk`、`MemoryChunkInput` 类型和 CRUD 方法，供后续 memory 列表页与 RAG 配置复用。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 用户启动 Docker 数据库后，通过真实 API 冒烟验证：创建临时 project，创建 manual memory chunk，按 `chunk_kind`、`tag`、`q` 查询，读取详情，更新正文和 tags，删除 memory chunk，删除测试 project。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的 Memory Chunk CRUD 任务和 Memory API 路由说明。

### Step 40: 支持 Block 关联角色、地点和标签

- 新增 `PATCH /api/blocks/:blockId/associations`，用于单独更新 block 的 Phase 4 关联信息。
- 关联信息写入 block `metadata`：`character_ids`、`location_id` 和 `tags`，不会覆盖 metadata 中其他键。
- 后端会去重并清理空的 `character_ids` 和 `tags`，空 `location_id` 会保存为 `null`。
- 前端 API client 新增 `BlockAssociationsInput` 和 `updateBlockAssociations`，供后续 Block Inspector metadata 编辑器直接调用。
- 通过真实 API 冒烟验证：创建临时 project、block、character canon、location canon，调用关联接口后确认 metadata 保留既有键，并写入去重后的 `character_ids`、`location_id` 和 `tags`，最后删除测试 project。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的 block 关联任务和 Blocks API 路由说明。

### Step 41: Block Inspector 添加 Metadata 关联编辑

- 在 Block Inspector 中新增“关联”折叠面板，接入 Phase 4 block association API。
- 面板会加载项目内 `character` 和 `location` 类型的 Canon Entity；角色支持多选，地点支持单选。
- 新增标签输入，支持用中文逗号、英文逗号或换行分隔，并在提交前去重。
- 保存后调用 `PATCH /api/blocks/:blockId/associations`，刷新当前 block、revisions 和 graph 缓存，保证画布与 Inspector 使用同一份 metadata。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的 Block Inspector metadata 编辑、选择出现角色、选择地点任务。

### Step 42: 修复新建 Block 历史版本重复显示 user

- 定位到新建 block 后历史版本面板显示三个 `user` 的原因：后端实际只创建 1 条 revision，但前端在只有 1 条 revision 时仍渲染两个 diff 下拉框，每个下拉框都会显示同一条 revision，再加上版本列表本身共出现三次。
- 调整 Block Inspector：只有 revisions 数量大于等于 2 时才显示 diff 对比控件，单版本时只显示历史版本列表。
- 通过真实 API 冒烟验证新建 block 后 `/api/blocks/:blockId/revisions` 只返回 1 条 `user` revision。

### Step 43: 显示当前 Block 关联的 Canon Entities

- 在 Block Inspector 顶部新增关联概览，直接显示当前 block 已保存的角色、地点和标签。
- 角色和地点从 Canon Entity 列表按 metadata 中的 `character_ids`、`location_id` 解析为名称，避免只暴露 UUID。
- 概览使用已保存的 block metadata，不把关联编辑表单里尚未保存的选择误展示为当前状态。
- 未关联任何 canon 时显示空状态，便于快速判断当前 block 是否已完成上下文标注。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的“显示当前 block 关联的 canon entities”任务。

### Step 44: Phase 4 管理页与从 Block 生成 Memory

- 新增 Canon 管理页 `CanonManager.vue`，角色、地点、世界规则共用同一个 CRUD 页面。
- 新增 Memory 管理页 `MemoryManager.vue`，支持按关键词、chunk kind、tag 过滤，支持手动创建、编辑、删除 memory chunk。
- Memory 管理页支持选择已有 block，并调用 `POST /api/blocks/:blockId/memory` 从当前 revision 生成 `block_revision` memory chunk。
- 工作台顶部新增入口：角色、地点、规则、Memory。
- 新增后端路由 `POST /api/blocks/:blockId/memory`，从 block 当前 revision 读取正文，净化 HTML 后保存到 `memory_chunks`，source 指向当前 revision。
- 更新前端 API client，新增 `createMemoryChunkFromBlock` 和对应类型。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的角色设定页面、地点设定页面、世界规则页面、memory 列表页面和从 block 生成 memory chunk 任务。

### Step 45: LLM 生成读取相关 Canon

- Generation repository 新增 block canon facts 加载逻辑：读取 block metadata 中的 `character_ids`、`location_id`，并自动加载项目内 `status=canon` 的世界规则。
- Prompt 渲染新增 `{{canon_facts}}` 变量，并将 canon facts 写入 generation run 的 `input_context_snapshot`。
- 默认生成模板加入“硬设定”段，续写、改写、局部改写、扩写、缩写、润色和自由生成都会要求遵守 canon。
- 通过真实 API 冒烟验证：创建临时 project、block、character、location、rule，关联 block 后触发流式生成到不可用本地 provider，错误事件返回的 prompt 中包含角色、地点和世界规则名称。
- 同一轮冒烟验证 `POST /api/blocks/:blockId/memory` 会从 HTML revision 生成纯文本 memory chunk。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Go provider 测试因 `httptest` 需要监听本地端口，使用提升权限运行；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的“LLM 生成时可以读取相关 canon”验收标准。

### Step 46: 实现非 Embedding Memory 文本搜索

- 新增 `POST /api/projects/:projectId/memory/search`，作为非 embedding 的关键词搜索端点。
- 搜索端点复用 Memory repository 的过滤能力，支持 `q`、`source_type`、`chunk_kind` 和 `tag`。
- 前端 API client 新增 `searchMemoryChunks` 和 `MemorySearchInput`，后续可替换 memory 列表页的 GET 查询或扩展搜索体验。
- 该端点不做 semantic search，不依赖 embedding provider；Phase 4 的 embedding provider、semantic search 和 reindex 仍按用户要求排除。
- 更新 `ARCHITECTURE.md` 中 Phase 4 的“系统可以根据文本检索相关 memory chunks”验收标准。

### Step 47: Phase 5 Context Builder 与上下文预览

- Phase 4 的 embedding provider、semantic search 和 reindex 继续暂缓，先进入 Phase 5 核心生成链路。
- 新增后端 Context Builder：按 task type 加载 current block、recent blocks、关联 canon、已有 branch/chapter summary 和关键词 memory chunks。
- Context Builder 支持近似 token budget、上下文裁剪、手动排除非必需 context item，并生成 system/user/final prompt。
- 新增 `POST /api/generate/context-preview`，前端可在生成前查看最终发送给 LLM 的上下文。
- `generate/once` 和 `generate/stream` 复用同一个 Context Builder，并把 context snapshot 写入 `generation_runs.input_context_snapshot`。
- Block Inspector 的 LLM 面板新增上下文预览区域，展示来源 item、预计 token、system prompt、user prompt 和 final prompt；用户可临时取消非必需 item 后再生成。
- 默认生成模板接入 `{{recent_blocks}}`、`{{branch_summary}}`、`{{chapter_summary}}` 和 `{{memory_chunks}}`，续写会自动包含最近正文，改写和局部改写会包含原文、设定与相关记忆。
- 运行 `go test ./...` 和 `npm run build`，均通过；Vite 仍提示 Tiptap bundle 体积警告。
- 更新 `ARCHITECTURE.md` 中 Phase 4 暂缓项、Generation API 路由和 Phase 5 任务清单。

### Step 48: API Key 配置改为直接粘贴

- 修正模型配置体验：模型页面的 API key 输入框现在接受真实 provider key，保存后即可用于生成。
- 继续保留 `env:VAR_NAME` 高级用法；生成时遇到 `env:` 前缀才从环境变量读取，否则直接使用保存的 key。
- Model Profile API 仍不回传明文 API key，只返回 `has_api_key` 状态。
- 移除初始化 schema 中 `api_key_ref` 只能为 `env:%` 的限制，并在后端启动时自动 drop 旧本地库中的 `model_profiles_api_key_ref_check` 约束。
- 前端模型配置页文案从“API key 环境变量”改为“API key”，输入框改为 password 类型；工作台生成警告同步更新。
- 更新 `.env.example` 和 `ARCHITECTURE.md` 中 API key 管理说明。

### Step 49: Edge 管理、上下文与工作台布局修复

- 新增 `PATCH /api/projects/:projectId/graph/edges/:edgeId`，支持修改 edge 类型、标签和 metadata；重复同类型 edge 返回 invalid graph。
- 前端工作台新增 Edge 管理面板，支持从列表或画布选中 edge、修改类型和标签、删除 edge；选中 edge 会在画布上高亮。
- Context Builder 的 recent block 查询同时读取 `references` / `summarizes` 的出边和入边，修复当前 block 指向前文时无法加载关联正文的问题。
- 修复上下文预览 checkbox：切换 item 后列表保持展开，并自动刷新 final prompt，不再整块消失。
- OpenAI-compatible provider 支持解析 `reasoning_content` 和 `reasoning` 字段；流式生成新增 reasoning 事件，前端以“模型推理内容”折叠区展示 provider 明确返回的推理文本。
- 重构工作台布局：正文和 LLM 操作从右侧 Inspector 移到画布下方的标签工作区；右侧只保留标题、关联、Fork 和历史版本等轻量模块。
- 运行 `go test ./...` 和 `npm run build`，均通过；Vite 仍提示 bundle 体积警告。
- 更新 `ARCHITECTURE.md` Graph API 路由。

### Step 50: 工作台工具整合为浮动窗口

- 移除占用固定页面高度的下方正文 / LLM 工作区和独立右侧详情栏。
- 将 Block 详情、正文编辑器和 LLM 操作整合为画布内浮动标签窗口，窗口内容独立滚动，不再拉长页面。
- 浮动窗口支持折叠和关闭；关闭后可通过画布右下角的 Block 工具按钮重新打开。
- 三个标签复用同一个 Block Inspector 实例，切换时保留正文草稿、编辑器选区和 LLM 生成状态。
- 增加移动端尺寸约束，浮窗始终限制在画布范围内。

### Step 51: Phase 5 核验与 Block Summary

- 核对 Phase 5 清单和实现：Context Builder、task type、token budget、上下文来源与裁剪、预览 API、前端预览和 generation snapshot 均已落地。
- 运行后端测试、前端类型检查和生产构建，确认 Phase 5 可验收；Vite 仍仅有既有的 bundle 体积警告。
- 开始 Phase 6，新增 `POST /api/blocks/:blockId/summarize`，使用请求指定的模型配置为 block 当前 revision 生成摘要。
- 摘要生成会净化 HTML 正文，并使用低温度、最多 800 token 的专用小说摘要提示词。
- 成功结果写入 `summary_snapshots`，记录 `target_type=block`、当前 covered revision、摘要 token 数、模型、valid 状态和 provider token metadata。
- 添加 block summary 请求规范化测试，并更新 `ARCHITECTURE.md` 中 Phase 6 的 block summary 任务。

### Step 52: Phase 6 摘要生命周期与前端操作

- 扩展 block summarize：普通 block 生成 block summary；chapter block 自动聚合章节自身及子 block 的 current revisions，生成 chapter summary。
- 新增 branch summary、summary refresh 和项目摘要列表 API；生成新 snapshot 时将同目标旧 valid snapshot 标记 stale。
- 创建 current revision 或切换 current revision 时，在同一事务中将相关 block、所属 chapter 和所属 branch 摘要标记 stale。
- Context Builder 对每个目标只读取最新摘要，优先使用 valid snapshot；最新 snapshot 为 stale 时返回可见提示但不加入最终 prompt。
- Block Inspector 新增摘要面板，显示 valid、stale、failed 状态、摘要正文和覆盖 revision 数，并支持手动生成或刷新。
- 工作台分支列表显示摘要状态，支持选择模型后生成或刷新分支摘要。
- Context Preview 展示 chapter/branch 摘要来源；过期摘要禁用勾选并明确标记“已过期”。
- 添加 stale summary 不进入上下文预算的单元测试；运行 Go 测试、前端类型检查和生产构建均通过，Vite 仍有既有 bundle 体积警告。
- 更新 `ARCHITECTURE.md`，完成 Phase 6 后端、前端与验收清单。

### Step 53: Block 工具拖动与独立标签页

- Block 工具浮窗标题栏支持 Pointer Events 拖动，并限制窗口始终位于画布可见范围内。
- 扩大浮窗默认宽高，调整圆角、阴影、标题栏、标签栏和内容背景层级。
- 移除功能重复的最小化按钮，仅保留单一收起按钮；收起后仍可通过画布右下角入口重新打开。
- 浮窗新增“在新标签页打开”操作，为当前 block 生成独立工具页路由 `/projects/:projectId/blocks/:blockId/tool`。
- 独立工具页保留详情、正文、LLM 操作三个标签，并提供返回项目工作台入口。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍有既有 bundle 体积警告。

### Step 54: 修复分支摘要 404 与错误响应解析

- 定位分支摘要失败原因为 8080 仍运行未注册 Phase 6 摘要路由的旧后端进程；重启最新后端后摘要列表与分支摘要路由生效。
- 前端 API client 不再无条件调用 `response.json()`，改为先读取响应文本并安全解析 envelope。
- 后端返回纯文本或空响应时，前端现在显示真实 HTTP 错误内容，不再抛出 `Unexpected non-whitespace character after JSON`。
- 使用项目配置的 DeepSeek V4 Flash 完成真实端到端验证：分支摘要返回 201、写入 8 个 covered revisions，并可通过项目摘要列表 API 查询。
- `contentscript.js` 的 EventEmitter / ObjectMultiplex 警告确认来自浏览器扩展内容脚本，与 BranchScribe 前后端无关。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍有既有 bundle 体积警告。

### Step 55: Phase 6 摘要状态完整性加固

- 摘要列表与 Context Builder 加载摘要前，会根据 `covered_revision_ids` 和目标当前 revisions 主动校验状态。
- 主动校验覆盖 block、chapter 和 branch，revision 集合不一致时将 valid snapshot 持久化标记为 stale，补足仅依赖 revision 写入钩子的缺口。
- provider 调用失败或返回空摘要时写入 failed snapshot，记录模型、覆盖 revisions 和错误 metadata，并将同目标旧 valid snapshot 标记 stale。
- Block Inspector 和分支摘要操作在生成失败后主动刷新摘要查询，使 failed 状态和刷新入口立即可见。
- 在真实数据库调用项目摘要列表，确认主动 stale 检测 SQL 执行成功且当前分支摘要保持 valid。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍有既有 bundle 体积警告。

### Step 56: 补完 Phase 4 Embedding 与语义检索

- 新增 OpenAI-compatible embedding provider，通过模型 Profile 的 Base URL/API key 调用 `/embeddings`，支持批量输入和可选 dimensions。
- 模型配置页新增 Embedding model 与 dimensions 字段，存入 Profile metadata，不影响现有生成模型参数。
- 新增项目级 Memory reindex：按批次为全部 memory chunks 和非 deprecated canon entities 生成向量并写入 pgvector。
- Memory search 支持 keyword 与 semantic 两种模式；semantic 模式生成查询向量，使用 cosine distance 排序并返回 similarity。
- Memory 页面新增 embedding Profile 选择、Memory + Canon reindex、显式语义搜索和相似度展示。
- Memory 正文或 Canon 可嵌入字段发生修改时清空旧 embedding，避免继续使用失效向量。
- 使用现有 SiliconFlow Profile 配置 `Qwen/Qwen3-Embedding-0.6B`（1024 维）完成真实验证：成功索引 5 条 Memory 和 1 条 Canon，语义查询返回按相似度排序的结果。
- 添加 embedding provider 和 vector literal 单元测试；运行 `go test ./...`、`npm run typecheck`、`npm run build`，均通过；Vite 仍有既有 bundle 体积警告。
- 更新 `ARCHITECTURE.md`，完成 Phase 4 的 embedding provider、memory semantic search 和手动 reindex 清单。

### Step 57: LLM 与 Embedding Profile 解耦

- `model_profiles` 新增 `profile_type`、`embedding_profile_id` 和 `embedding_dimensions`，同一项目可分别维护 LLM 与 Embedding Profile。
- Embedding Profile 拥有独立 provider、Base URL、API key、model 和 dimensions，不再复用主 LLM 的 provider 配置。
- LLM Profile 可选择关联一个 Embedding Profile；后端接受 LLM Profile ID 时会自动解析其关联的 embedding 配置。
- 启动兼容迁移会将旧 Profile metadata 中的 embedding 配置复制为独立 Embedding Profile、回填关联，并清理旧 metadata 字段。
- 模型配置页面按 LLM Profiles 与 Embedding Profiles 分类展示，提供分别新建和编辑的表单；Embedding Profile 不再出现在正文生成模型选择中。
- Memory 页面只列出 Embedding Profile 供 reindex 和 semantic search 使用。
- 真实迁移生成独立 `Qwen/Qwen3-Embedding-0.6B` Profile，并关联到 DeepSeek V4 Flash；直接使用 Embedding Profile 或关联的 LLM Profile 进行语义检索均返回 200。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍有既有 bundle 体积警告。

### Step 58: 允许过期摘要作为可选上下文

- Context Builder 不再强制排除 stale summary，保留原摘要正文并与 valid summary 一样参与 token budget。
- stale summary 默认可进入最终 prompt；用户仍可在 Context Preview 中临时取消。
- Context Preview 保留“摘要已过期”提示，但恢复 checkbox 操作，不再禁用选择。
- Block 摘要状态文案调整为：过期摘要仍可作为前文参考，也可选择刷新。
- 更新单元测试，验证 stale summary 可以被上下文预算纳入。

### Step 59: Phase 7 分支写作增强

- 新增 `GET /api/branches/:branchId/path`，按 base branch 与 fork block 还原当前分支的祖先正文路径。
- 分支 fork 会校验项目、起点 block 和 revision 的归属，并支持从任意历史 revision 建立分支起点。
- 新增 `compare_revisions` 生成模板与 `POST /api/generate/candidates`，固定生成两个具有实质差异的候选版本。
- 候选比较界面支持并排阅读、保存为两个非当前 revision、选择一个设为当前版本，以及展开为独立 Block 或 Branch。
- 画布按 branch 显示颜色；fork edge 保留独立颜色与动画；右键 block 可创建新 branch 并生成 fork block。
- 分支列表支持切换当前 branch 和归档不用的 branch。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍有既有 bundle 体积警告。
- 更新 Branch / Generation API 文档并完成 Phase 7 清单。

### Step 60: 支持自选故事线上下文节点数量

- 生成、候选生成、流式生成和上下文预览请求新增 `context_node_count`，未传时兼容为前 1 个节点。
- 支持输入任意非负前文节点数；`0` 表示不加入前文，也可选择当前故事线全部前文节点。
- Context Builder 沿当前节点的 `next` / `fork` 图边递归加载故事前文，并按根节点到当前节点的顺序组装；其他分支和引用边不会混入，“全部”仍受模型 token budget 裁剪。
- 将本次选择写入 generation run 的 context snapshot，便于追溯每次生成使用的上下文策略。
- 在 Block LLM 面板增加故事线前文节点选择器，并同步更新上下文预览。
- 添加请求默认值、全部节点和非法数量校验测试；运行 Go 测试、前端类型检查和生产构建均通过，Vite 仍有既有 bundle 体积警告。
- 使用现有项目向 `context-preview` 发送 `context_node_count: -1`，确认递归路径 SQL 返回完整前文列表，并按 token budget 标记最终纳入项。
- 更新 `ARCHITECTURE.md` 的 Phase 5 工作清单和验收标准。

### Step 61: 将 LLM 操作升级为持久化 Chatbox

- 新增 `llm_conversations` 与 `llm_messages`，对话归属于 block，assistant message 可关联 generation run。
- 新增对话 list/create/update/delete、消息 list/update API；编辑历史 user message 时会截断后续旧消息。
- 生成请求支持 `conversation_id`，调用模型时按 `system + 历史 user/assistant + 当前 user` 组装多轮消息。
- 生成请求支持本轮覆盖 temperature、top_p 和 max_tokens，不修改原 Model Profile。
- LLM 操作界面改为消息流与底部 Chatbox：模型快捷切换、发送/停止位于输入框底栏。
- 自由生成、续写、改写等任务类型，以及上下文选择和参数调整收进二级工具菜单。
- 支持创建、切换、删除 block 对话；每轮消息支持复制，user 消息支持编辑并从该轮重新开始。
- 保留上下文预览、候选生成和将最新回复保存为 Revision 的现有能力。
- 运行 Go 测试、前端类型检查和生产构建均通过；使用真实数据库验证 conversation create/list/delete，并在浏览器中确认 Chatbox 自动展开和二级菜单不被裁切。

### Step 62: 将写作操作升级为可编辑 Prompt 库

- 为每个项目初始化自由生成、续写、改写、局部改写、扩写、缩写和润色七个默认操作，默认操作以普通 `prompt_templates` 数据保存，不再只是界面中的写死选项。
- 新项目通过数据库 trigger 自动获得默认操作；已有项目通过一次性兼容迁移补齐，用户删除默认操作后不会在后续启动时被重复创建。
- LLM Chatbox 接入 Prompt Template CRUD，可选择、新增、编辑和删除写作操作；生成与上下文预览明确携带当前 `prompt_template_id`。
- 重做写作操作与上下文二级菜单：使用标题、说明、列表选中态、内嵌 Prompt 编辑器和分区式上下文预览，视觉与当前 Chatbox 保持一致。
- Prompt 编辑器展示当前实际支持的模板变量，项目架构文档同步修正变量列表。

### Step 63: 将候选生成改为 Chatbox 双版本模式

- 移除写作操作菜单中的一次性“生成两个候选版本”命令，在输入框底栏增加可保持状态的“双版本”开关。
- 开关关闭时维持单版本流式回复；开启时，每次提交同一条用户消息都会使用当前写作操作生成两个差异化候选。
- 双版本请求复用当前 Prompt Template、模型参数、上下文节点数量、临时排除项和已有对话历史。
- 后端生成两个候选时不再重复写入 user 消息；两个候选完成后统一写入一次 user 消息和两条 assistant 消息。
- 双版本生成期间发送按钮显示旋转进度图标并保持禁用，避免此前只有灰色按钮、缺少状态反馈的问题。

### Step 64: 完善 LLM 回复操作栏与历史编辑

- 将 assistant 回复改为更接近 CherryStudio 的开放式消息布局，移除回复正文外层的厚重卡片感。
- 复制、编辑和保存 Revision 统一放在每条回复底部的轻量图标操作栏，移除旧版醒目的“保存为 Revision”大按钮。
- 持久化的 user 与 assistant 消息都支持编辑；保存编辑时仅更新当前消息，不再截断其后的旧对话。
- 任意历史 assistant 回复都可直接保存为 Revision，并记录来源 conversation、message 和 generation run。
- 流式生成完成后的临时回复同样提供复制与保存 Revision 操作。

### Step 65: 修复 LLM Chatbox 历史交互问题

- 统一消息正文、消息编辑框和底部输入框的字体、字号与行高，减少输入态和阅读态之间的视觉跳变。
- Agent 历史回复通过 generation run 展示实际模型名称，流式回复展示当前所选模型。
- user 与 assistant 消息操作栏均增加“重新生成”；assistant 会复用其前一条 user 消息，并保留已有对话历史。
- 消息工具使用统一的悬停名称提示；编辑提示同步明确为仅保存当前消息、不影响后续内容。

### Step 66: 修正 Chatbox 重新生成语义

- 点击重新生成时直接使用目标轮次的用户输入发起请求，不再把提示词回填到底部输入框。
- user 消息的重新生成定位其后一条 assistant 回复；assistant 消息的重新生成直接定位自身。
- 后端重新生成时不追加 user 或 assistant 消息，而是原位更新目标 assistant 内容与 generation run 关联。
- 流式结果在原回复位置展示，生成上下文只读取目标回复之前的历史消息。

### Step 67: 修复 Agent 回复完成态

- 普通流式生成成功后等待持久化消息刷新，再移除临时回复，避免页面长期停留在缺少完整操作栏的流式状态。
- 完成态回复统一读取 generation run 的模型名称，并复用持久化消息的编辑、复制、重新生成和保存 Revision 操作。
