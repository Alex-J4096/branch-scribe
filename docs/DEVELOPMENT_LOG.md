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

### Step 68: 简化 Block 工具正文与 LLM 布局

- 正文和 LLM 标签内不再显示只有一个内容项的重复折叠标题，标签本身作为唯一导航。
- 两个工具面板填满浮动 Block 工具和独立 Block 工具页的剩余空间。
- 外层工具容器关闭滚动，正文仅由编辑区滚动，LLM 仅由消息区滚动，避免嵌套滚动条争抢操作。

### Step 69: 统一应用工具型界面视觉密度

- 以 LLM Chatbox 的 11–13px 文字、30–32px 控件和紧凑间距为基准，统一全局表单、按钮和图标尺寸。
- 收紧项目列表、主工作台顶栏与侧栏、Block 节点和浮动工具窗口，提升画布与正文的有效工作面积。
- 同步调整模型配置、Canon、Memory、独立 Block 工具页和正文编辑器的标题、留白及控件密度。
- 运行前端生产构建并通过；在浏览器中核对项目页、主工作台、正文工具与模型配置页。

### Step 70: 启动 Phase 8 小说工程数据基础

- 新增 `character_states`、`foreshadowings` 和 `timeline_events` 三类项目数据表，包含项目归属、Block/Canon 关联、JSON metadata、索引与更新时间触发器。
- 启动兼容迁移会为已有数据库幂等补建 Phase 8 表、索引和触发器，新安装则由初始化 schema 直接创建。
- 新增角色状态 CRUD API，可按角色筛选并记录状态键、结构化状态值、发生位置与关联 Block。
- 新增伏笔 CRUD API，支持埋设、发展、回收、废弃四种生命周期状态，并可关联埋设与回收 Block。
- 新增时间线事件 CRUD API，支持故事内时间文本、手动排序以及 Block/Canon Entity 关联。
- 运行 `go test ./...` 并通过；连接真实本地 PostgreSQL 启动后端，确认兼容迁移执行成功且全部 Phase 8 基础路由完成注册。
- 更新 `ARCHITECTURE.md`，完成 Phase 8 的三项数据库任务和三项基础记录后端任务。

### Step 71: 修复 LLM Chatbox 发送后未清空输入框

- 单版本流式生成在请求开始后立即清空输入框，不再等待完整回复结束。
- 双版本生成在保存本次请求参数后立即清空输入框，与单版本发送体验保持一致。

### Step 72: 从后续剧情提取可追溯角色卡

- 新增角色卡提取 API，可选择后续剧情 Block 与 LLM Profile，根据旧角色卡和剧情正文生成完整角色卡候选及变化摘要。
- 提取结果先进入可编辑预览，用户确认后才更新当前 Canon 角色卡，避免模型输出直接覆盖硬设定。
- 每次确认都会向 `character_states` 新增 `character_card` 快照，记录来源 Block、模型、完整描述、Attributes 和变化摘要，同一角色可保留多个历史版本。
- 角色设定页新增“从后续剧情提取”入口与版本历史区，可展开查看每个版本的时间、变化摘要和完整快照。
- 增加角色卡 JSON 响应解析测试；后端全量测试与前端类型检查通过。
- 更新 `ARCHITECTURE.md`，完成 Phase 8 的角色状态提取、角色状态面板与用户维护角色状态任务。

### Step 73: 按故事分支汇总角色卡提取范围

- 角色卡提取改为选择一个起始 Block，自动列出该节点及同一分支中顺序在其后的全部 Block，并默认全选。
- 用户可在提取前逐个取消后续 Block；起始 Block 固定纳入，最终勾选列表按故事顺序发送。
- 后端验证所有 Block 均属于起始节点所在分支且顺序不早于起点，按顺序拼接正文后再生成角色卡候选。
- Generation Run 与角色卡历史版本 metadata 均记录完整来源 Block ID 列表，保留本次摘要的准确覆盖范围。

### Step 74: 实现伏笔生命周期面板

- 新增独立伏笔管理页，并在项目工作台提供直接入口。
- 支持伏笔新建、编辑、删除以及按已埋设、发展中、已回收、已废弃四种状态筛选。
- 支持关联埋设 Block；伏笔进入已回收状态后可额外关联回收 Block，并在列表中展示两端位置。
- 运行后端全量测试与前端生产构建并通过。
- 更新 `ARCHITECTURE.md`，完成 Phase 8 的伏笔面板与用户维护伏笔列表验收任务。

### Step 75: 完成 Phase 8 小说工程后端

- 新增 Block 一致性检查 API，以当前正文及其关联角色、地点和全局规则 Canon 为依据，返回带严重度、原文主张、冲突设定、解释和修订建议的结构化冲突列表。
- 一致性结果严格校验 Canon Entity ID，拒绝模型虚构的设定引用，并以 `check_consistency` 任务类型记录完整 Generation Run。
- 新增 Block 时间线事件提取 API，按正文顺序返回可编辑事件候选、故事内时间表达和可验证的 Canon 关联。
- 时间线提取结果会重新规范排序，并移除模型输出中不存在的 Canon 关联；调用以 `extract_timeline_events` 任务类型记录 Generation Run。
- 增加一致性响应与时间线提取响应解析测试。
- 运行 `go test ./...` 并通过。
- 更新 `ARCHITECTURE.md`，完成 Phase 8 全部后端任务。

### Step 76: 完成 Phase 8 一致性检查与时间线前端

- Block 详情新增一致性检查面板，可选择当前 LLM Profile 执行检查，并展示检查摘要、冲突严重度、正文主张、Canon 依据、解释和修订建议。
- 每条冲突支持打开对应 Canon Entity，设定页会自动定位并进入该设定的编辑状态；同时支持打开冲突来源 Block。
- 新增项目级故事时间线页面，支持事件新增、编辑、删除，按故事排序展示时间表达、来源 Block 与相关 Canon。
- 时间线页面支持选择 Block 和模型提取事件，提取结果携带 Generation Run 来源并直接加入可继续维护的时间线。
- 工作台新增伏笔与时间线入口。
- 运行前端类型检查与生产构建并通过。
- 更新 `ARCHITECTURE.md`，完成 Phase 8 全部前端任务与验收标准。

### Step 77: 启动 Phase 9 导出与备份后端

- 新增 Markdown 导出接口，支持按 Branch 导出包含祖先分支正文的完整故事线，或按 Chapter 导出至下一个章节节点之前的正文。
- HTML Revision 在导出时转换为可脱离编辑器阅读的 Markdown 文本，并提供安全的下载文件名。
- 新增项目 JSON 备份接口，覆盖项目、分支、Block、Revision、图、Canon、Memory、摘要、小说工程记录、模型配置、Prompt、Generation Run 与 LLM 对话。
- 备份明确排除模型 API key 和向量 embedding，避免敏感凭据外泄并保持文件可移植。
- 新增事务化 JSON 导入接口，分阶段恢复循环外键；目标 Project UUID 已存在时返回冲突，不覆盖现有项目。
- 增加 Markdown 生成及文件名清理测试。
- 运行后端全量测试并通过；连接真实本地 PostgreSQL 完成临时项目的备份、删除、恢复、清理闭环，并验证现有 Branch 可成功导出 Markdown。
- 更新 `ARCHITECTURE.md`，完成 Phase 9 的 Markdown、Branch、Chapter 导出及 JSON 备份导入后端任务。

### Step 78: 完成 Phase 9 导出与备份前端

- 工作台新增“导出”入口和独立导出与备份页面。
- 正文导出支持在 Branch 与 Chapter 范围间切换；格式选择保留为 Markdown，并在项目不存在 Chapter 时提供明确空状态和禁用反馈。
- 支持直接下载项目 JSON 备份，并在页面说明 API Key 与 embedding 不会进入备份。
- 导出页支持选择 JSON 文件恢复项目；项目列表也提供导入入口，确保没有现存项目时仍可从备份恢复。
- 导入前先在浏览器解析并校验 JSON，恢复成功后自动打开对应项目；同 UUID 项目仍存在时显示后端冲突提示。
- 运行前端类型检查和生产构建并通过。
- 使用本地真实前后端在浏览器验证工作台入口、Branch Markdown 导出、Chapter 空状态、JSON 备份下载及项目列表导入入口，页面控制台无错误。
- 更新 `ARCHITECTURE.md`，完成 Phase 9 全部前端任务与验收标准。

## 2026-06-29

### Step 79: 将模型配置改为全局设置

- 移除 `model_profiles.project_id`，新增兼容既有数据库的迁移脚本，已有模型配置会保留并转为全局共享。
- Model Profile 列表与创建 API 改为全局 `/api/model-profiles`，生成、上下文预览和 Embedding 链路不再按项目限制 profile。
- 模型配置页迁移到 `/settings/model-profiles`，项目列表增加“全局模型”入口；各项目工作台、角色、时间线和记忆功能统一读取全局 profiles。
- 保留项目对全局 profile 的默认选择和每次生成时的 profile 选择能力；项目备份不再携带或恢复全局模型及凭据。
- 修复全局化兼容迁移在第二次启动时仍引用已删除 `project_id` 的问题，并连续启动两次后端验证迁移幂等。

### Step 80: 新增独立 LLM 调试 CLI

- 按开发流程先在 `ARCHITECTURE.md` 的 Phase 3 登记本轮目标与任务，再开始代码实现。
- 新增 `backend/cmd/llm-debug` 独立命令，默认监听 `127.0.0.1:6069`，也可通过 `-addr` 修改地址。
- 新增 Provider 调试装饰器；设置 `LLM_DEBUG_URL` 后，后端会上报每次文本生成实际使用的 provider、model、生成参数与最终 `messages`。
- 流式调用逐块上报 reasoning、正文 delta、完成、用量和错误；非流式调用上报完整 reasoning、正文与用量。
- 调试事件不包含 API key，并通过有界异步队列尽力投递；监听器未启动、不可达或消费过慢时不会阻塞正常生成。
- README 补充调试 CLI 与后端联动的启动方式。
- 新增单元测试，覆盖最终 messages 上报、非流式响应、流式事件透传和错误上报。
- 运行 `go test ./...`，后端全部测试通过。
- 启动真实调试 CLI 并发送冒烟事件，确认最终 messages、reasoning、流式正文和 token 用量均按预期打印。
- 完成后勾选 `ARCHITECTURE.md` 中本轮全部任务。

### Step 81: 为 LLM 调试工具增加 Web 界面

- 将独立调试进程升级为内嵌 Web UI 的调试服务，仍保持单个 Go 命令启动，不依赖主前端构建或运行。
- 新增请求列表与详情两栏布局，按请求集中展示模型参数、最终 messages、reasoning、content、状态和 token 用量。
- 通过 SSE 将调试事件实时推送给页面，reasoning 与 content 随模型返回逐块更新。
- messages 按 role 分段展示，messages、reasoning 和 content 均支持折叠，页面支持请求切换与清空最近历史。
- 服务端内存保留最近 100 次请求；终端改为仅打印请求开始、完成和错误概要，避免长文本字墙。
- `start.sh debug` 直接启动调试 Web UI，README 补充浏览器访问地址。
- 新增调试会话聚合与历史清空测试，运行 `go test ./...` 全部通过。
- 使用真实浏览器验证两栏布局、长文本阅读、折叠展示与清空历史，页面控制台无错误。
- 完成后勾选 `ARCHITECTURE.md` 中本轮全部任务。

### Step 82: 优化 LLM 调试界面的阅读体验

- 将调试界面从暗色终端风格调整为明亮主题，重新梳理请求列表、调用标题、状态、参数和正文卡片的视觉层级。
- messages 改为按角色标签和自然文本排版，reasoning 与 content 保留独立阅读区域和换行结构。
- 新增 Metadata 折叠区，先展示 Request ID、时间、Base URL、流式状态等关键字段，再提供包含 messages、输出、状态和 token 数据的完整原始会话 JSON。
- 使用真实浏览器验证明亮主题、内容渲染与 Metadata 展开，页面控制台无错误。
- 运行 `go test ./...`，后端全部测试通过。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

## 2026-06-30

### Step 83: 修复 LLM User 消息偶发重复显示

- 定位到流式生成期间的竞态：后端已落库的 User 消息被查询刷新到列表后，本地 pending 占位消息仍在显示，导致同一内容短暂出现两次。
- 发送前记录当前会话消息 ID；当相同内容的新 User 消息已经落库时，自动隐藏对应 pending 占位，同时不误伤用户有意连续发送的相同内容。
- 为单版本流式发送增加同步 starting 状态，在创建会话和刷新历史的异步准备阶段也阻止 Enter 或点击快速重复触发。
- 发送按钮在准备阶段禁用并显示 loading，重新生成入口同步遵循防重状态。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍仅提示既有 bundle 体积警告。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 83: 编写记忆功能说明书

- 新增 `docs/MEMORY_FEATURES_GUIDE.md`，系统说明角色卡、地点卡、世界规则、Memory、角色状态、伏笔与时间线的定位和使用方法。
- 根据当前 Context Builder 实现说明各类资料进入 LLM 上下文的条件、顺序、数量限制与 token 裁剪行为。
- 明确普通写作的 Memory 自动召回目前使用关键词匹配，管理页语义搜索与 Reindex 使用向量索引。
- 补充角色卡版本提取、Block 关联、一致性检查、事件提取、常见误区与推荐工作流。
- 更新 `ARCHITECTURE.md`，登记并完成本轮文档任务。

### Step 84: 按当前实现整理开发文档

- 将项目目标、技术栈、总体架构、核心概念、后端模块、LLM Provider、上下文策略、前端界面和仓库结构改写为当前实现。
- 明确保留摘要自动失效与重建、Branch 合并、原生 Provider Adapter、Qdrant 和富文本结构化 Diff 等尚未落地的规划项。
- 更新 `ARCHITECTURE.md` 文档任务清单并标记完成。

### Step 85: 补充分支编辑与空分支清理

- 工作台分支区域新增编辑入口，支持修改名称和说明，并可恢复已归档分支。
- 不含节点的分支显示永久删除入口，解决 fork 节点全部删除后遗留空壳分支的问题。
- 后端删除分支时校验节点和子分支引用，避免节点被静默改为无所属状态或破坏分支继承关系。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过；Vite 仍仅提示既有 bundle 体积警告。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 86: 修复历史消息重新生成的对话边界

- 重新生成历史回复时，先定位目标 assistant 及其对应的前置 user，并只保留该 user 之前的完整轮次。
- 目标 user 会通过本轮 Context Builder 生成的 user prompt 发送，不再同时作为历史消息重复进入 messages。
- 原 assistant 回复及目标 user 之后的所有轮次均不会带入本次 LLM 请求。
- 新增 `a/b/c` 多轮对话回归测试及非法目标校验，运行 `go test ./...` 全部通过。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 87: 支持批量选择和删除 LLM 对话消息

- LLM 对话框新增消息多选模式，User 与 Agent 消息均可独立勾选，并显示当前选中数量。
- 新增批量删除确认操作；删除完成后清理选择状态并刷新消息与对话列表。
- 后端新增会话内消息批量删除接口，通过事务校验所有消息均属于指定会话，避免跨会话或部分删除。
- 增加消息 ID 清理与校验测试，运行 `go test ./...`、`npm run typecheck` 和 `npm run build` 均通过。
- 使用真实本地页面验证 User / Agent 混选、选中数量、删除按钮状态和取消选择，控制台无错误；验收未删除现有数据。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 88: 支持选择模型重新生成历史回复

- 历史消息的重新生成操作增加模型选择器，可临时选择任意已配置 API Key 的 LLM 模型。
- 临时模型仅用于本次重新生成，不改变 Chatbox 当前默认模型；切换到其他模型时使用该模型配置的生成参数。
- 未配置 API Key 的模型保留展示但不可选择，执行前再次校验模型可用性。
- 同步修复消息编辑态随短文本收缩的问题，使 User 与 Agent 编辑框保持稳定的阅读宽度。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍仅提示既有 bundle 体积警告。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 89: 修复 LLM 请求失败后的不可操作消息

- 移除发送前对会话消息查询的阻塞式刷新，避免后端不可达时被查询重试锁在 starting 状态。
- 生成结束或失败后使用一次性请求读取持久化消息，不沿用查询层的自动重试等待。
- 已落库的失败 User 指令会转为正常消息卡片，恢复复制、编辑、重新生成、多选和删除能力。
- 未落库的指令自动退回 Chatbox 输入框，同时清除 pending User 气泡和临时 Agent 输出，避免内容丢失或残留不可操作状态。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；使用隔离后端断开连接完成真实失败测试，确认指令回填、幽灵消息消失且多选与发送按钮恢复可用。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 90: 修复 LLM Debug 流式期间的界面阻塞

- 定位到 Debug Web UI 对每个 reasoning / content chunk 都重建历史列表与完整详情 DOM，高频事件下按钮会在点击过程中被反复替换。
- 将请求列表与详情渲染拆分，流式 chunk 不再重绘历史请求列表，保证列表按钮持续可交互。
- 当前流式详情改为每 100ms 合并刷新一次，降低长文本不断增长时的主线程渲染压力。
- 用户选择历史请求后，其他正在运行请求的流式事件只更新内存状态，不再重绘或覆盖当前历史详情。
- 运行 `go test ./...` 全部通过；使用隔离 Debug 服务持续发送 3000 个流式 chunk，历史切换约 320ms 完成，事件继续到达期间选择和详情均保持稳定。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 91: 支持中断后从孤立 User 消息续接生成

- 新增 `retry_user_message_id` 请求语义，区分“从尚无回复的 User 续接”与“替换已有 assistant 回复”。
- 后端校验续接目标必须是当前会话最后一条 User 消息，只携带该 User 之前的历史，并以其内容构建本轮 User Prompt。
- 续接时不重复保存 User 消息；模型成功返回后追加缺失的 Agent 回复，原有 assistant 重新生成仍保持原位替换。
- 前端重新生成入口识别 User 后是否存在紧邻的 assistant；不存在时自动走续接生成，并继续支持临时选择其他模型。
- 增加历史截断、非法角色、非末尾 User、目标不存在和冲突请求字段测试。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build` 均通过；Vite 仍仅提示既有 bundle 体积警告。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 92: 将写作操作改为首轮默认启用的显式开关

- Chatbox 写作操作菜单新增启用/关闭开关；新对话没有历史消息时默认启用，已有多轮历史时默认关闭。
- 选择或新建写作操作会主动启用本轮模板，用户仍可在发送前手动关闭。
- 关闭写作操作时以 `free_write` 任务发送，只保留 system 与原始 User 指令，不加载或组装当前正文、Canon、Memory、摘要等上下文项。
- API 新增向后兼容的 `apply_prompt_template` 字段；旧客户端未传时仍默认启用模板。
- UI 继续保存简短原始 User 指令，模型历史则从关联 generation run 快照恢复首轮实际发送的完整模板 Prompt，保证模板只组装一次但仍留在多轮上下文中。
- 若用户编辑历史 User 消息，后续请求优先使用编辑后的内容，不再使用旧快照 Prompt。
- 增加模板关闭、上下文跳过、默认兼容和历史 Prompt 恢复测试；运行 `go test ./...`、`npm run typecheck` 和 `npm run build` 均通过。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

## 2026-07-01

### Step 93: 标签化 Prompt 上下文并支持 Debug 折叠阅读

- Prompt 模板变量统一渲染为中文标签块，包括项目简介、硬设定、分支/章节摘要、最近正文、相关记忆、当前片段、选中文本和用户指令。
- 兼容已有数据库模板与自定义模板：`硬设定：+变量` 会替换为完整标签块，裸变量自动包裹，已经手写标签的变量不会重复嵌套。
- LLM Debug Messages 新增标签解析，将普通说明与各上下文块分开渲染。
- 超过 240 字的标签块默认折叠，短块默认展开；展开内容限制最大高度并支持内部滚动。
- Metadata 与原始会话 JSON 保持不变，仍可查看模型实际收到的完整标签文本。
- 增加标签渲染、不重复嵌套及 Debug UI 能力测试，运行 `go test ./...` 全部通过。
- 使用隔离 Debug 服务进行浏览器验收：300 字硬设定默认折叠，短当前片段和用户指令默认展开，Metadata 保留原始 `<硬设定>` 标签。
- 完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 90: 增加可编排的分支摘要设置

- 将分支面板中单一的摘要输入策略升级为“摘要设置”菜单，集中配置模型、摘要 Prompt 和上下文构成。
- 支持逐 Block 选择使用完整正文、已有有效摘要或不纳入本次分支摘要，并提供“全部正文”“有摘要则压缩”“仅已有摘要”三个快速预设。
- 没有有效摘要的 Block 会禁用摘要选项；上下文为空时阻止生成，避免向模型发送无效请求。
- 分支摘要 API 新增逐 Block 上下文选择参数，校验 Block 归属、重复项、输入模式和摘要有效性，并只记录实际纳入的 revision。
- 摘要快照 metadata 记录上下文构成、输入策略和 Prompt 模板；再次打开设置时自动恢复，并在摘要失效后允许用户重新调整。
- Prompt 管理页支持从摘要设置定向进入分支摘要类型，并可返回原工作台位置。
- 新增摘要请求参数校验测试；运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过。
- 使用本地真实页面验证设置弹窗、逐项状态、快速预设、空上下文保护和 Prompt 管理入口；完成后勾选 `ARCHITECTURE.md` 中本轮任务。

### Step 91: 完善摘要配置、取消与手写工作流

- 摘要设置的“完成”按钮会将模型、Prompt、输入策略和逐 Block 上下文构成保存到当前 Branch metadata；重新打开设置时优先恢复持久化配置。
- “取消”或关闭设置弹窗会放弃未保存改动；“按此设置生成”会先保存配置，保存成功后再发起摘要请求。
- Block 与 Branch 的 LLM 摘要请求接入 `AbortController`；生成期间显示“取消生成”，取消后终止浏览器请求，并通过 HTTP request context 继续取消上游 Provider 调用。
- Block / Chapter 摘要面板新增“手动新增”和“编辑摘要”；保存时创建新的有效摘要快照、覆盖当前摘要视图并保留旧快照。
- 新增手动摘要 API：`POST /api/blocks/:blockId/summaries` 与 `PATCH /api/summaries/:summaryId`，校验项目、目标与非空摘要内容。
- 新增手动摘要请求参数测试；运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过。
- 使用真实页面验证摘要设置保存关闭、Block 手动新增/编辑入口及编辑内容回填，页面控制台无错误。

### Step 93: 将 README 扩展为中文项目说明书

- 使用中文重写 README，补充项目定位、主要功能、核心概念和技术栈。
- 按实际配置与启动入口整理环境变量、数据库、前后端及 LLM Debug 的运行步骤。
- 补充模型配置、首次创作流程、常用开发命令、项目结构和常见问题。
- 在 `ARCHITECTURE.md` 的工作清单中登记并完成本轮文档任务。

### Step 94: 系统修正前端视觉层级与可用性

- 为全局颜色、表面、描边、强调色、焦点环和窗口阴影建立基础设计 token，统一控件视觉语言。
- 增大按钮和图标按钮的可点击面积，补全键盘焦点态、文本选区样式与减少动态效果偏好。
- 修复工作台顶栏工具过多时挤压项目标题和溢出布局的问题，工具区改为独立横向滚动导航。
- 重新梳理项目列表、弹窗和管理页的背景、分隔、圆角与层级，改善窄屏下的操作区换行。
- 将正文编辑器调整为低干扰的纸张式阅读区，使用中文衬线字体、更舒适的行距和受控行宽。
- 提升状态标签、LLM 工具菜单和编辑提示等过小文字的可读性，并补充正文与摘要设置的窄屏规则。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍仅提示既有 bundle 体积警告。

### Step 95: 补齐摘要模型配置与默认 Prompt 可见性

- Block / Chapter 摘要面板新增独立可见的摘要模型选择器，展示所有 LLM Profile，并禁用未配置 API Key 的模型。
- 摘要 Prompt 选择器明确展示当前默认模板名称；没有项目模板时才显示“系统默认”，避免把内置回退与可编辑 Prompt 混淆。
- 修复老项目摘要 Prompt 补种逻辑，新增 `summary_prompt_operations_v2` 兼容迁移，确保即使旧版迁移曾被错误标记完成，也会按 task type 补齐 Block、Chapter 和 Branch 三类默认模板。
- 使用真实数据库启动后端执行兼容迁移，确认 Prompt 管理页显示 3 个摘要模板，“分支摘要”可见、可编辑且为默认模板。
- 使用真实页面确认 Block 摘要面板同时显示“摘要模型”和“摘要 Prompt”，模型列表与默认 Block 摘要模板正确加载，控制台无错误。
- 运行 `go test ./...`、`npm run typecheck` 和 `npm run build`，均通过。

### Step 96: 修复 Block 角色关联误选

- 将 Block 关联面板的原生角色多选框改为逐项复选框，候选角色默认不关联，只有明确勾选并保存的角色才写入当前 Block。
- 增加角色列表滚动区域和无角色卡提示，保留原有 `character_ids` 保存协议与已保存关联回填逻辑。
- 运行 `npm run typecheck` 和 `npm run build`，均通过；Vite 仍仅提示既有 bundle 体积警告。
- 完成后勾选 `ARCHITECTURE.md` 中对应任务。

### Step 97: 补全 LLM 生成结束原因日志

- OpenAI-compatible 非流式与流式响应均解析供应商 `finish_reason`，保存到 `generation_runs.finish_reason`，并显示在 LLM Debug 页面与终端摘要中，便于区分正常停止、达到长度上限及其他结束原因。
- 流式连接若未收到 `[DONE]` 就结束，不再误记为成功，而是以 `provider stream ended before [DONE]` 标记 generation run 失败。
- 修复 Debug 页面实时事件合并遗漏 `finish_reason` 的问题，生成结束后无需刷新页面即可显示结束原因。
- 为已有数据库增加兼容字段迁移，并补充 `finish_reason=length` 和流提前结束的 Provider 测试。
- 完成后勾选 `ARCHITECTURE.md` 中对应任务。
