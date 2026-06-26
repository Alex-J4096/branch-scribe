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
