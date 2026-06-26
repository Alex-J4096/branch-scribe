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
