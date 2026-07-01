# BranchScribe

BranchScribe 是一个基于大语言模型（LLM）的非线性长篇故事创作工具。

它不是传统的聊天式 AI 助手，而是一套面向小说创作的文本工程系统：作者可以把故事拆分成节点，在图结构中组织章节、场景和分支，为正文保留多个版本，并精确控制每次发送给模型的上下文。

> 项目目前处于持续开发阶段，适合本地运行、功能体验和参与开发。

## 主要功能

- **节点式故事工作台**：用图结构管理章节、场景、笔记、摘要、大纲和故事分支。
- **正文与版本管理**：使用富文本编辑器创作，每次保存形成新的 Revision，可比较和回溯历史版本。
- **非线性分支**：从任意 Block 派生不同剧情走向，在同一项目中并行探索多个版本。
- **LLM 辅助写作**：支持续写、改写、扩写、缩写、润色、局部修改和自由指令，并以流式方式展示结果。
- **上下文编排**：按当前正文、前文路径、摘要、Canon 和 Memory 组合上下文，并可在生成前预览和取舍。
- **设定管理**：维护角色卡、地点卡和世界规则，辅助模型遵守故事中的硬设定。
- **剧情工程工具**：管理角色状态、伏笔和故事时间线，并对正文执行设定一致性检查。
- **模型与 Prompt 配置**：集中管理 LLM、Embedding 模型和写作操作模板。
- **导出与备份**：按 Branch 或 Chapter 导出 Markdown，也可通过 JSON 备份和恢复整个项目。

## 核心概念

| 概念 | 说明 |
| --- | --- |
| Project | 一部小说或一个独立创作项目 |
| Branch | 一条故事线，例如主线、角色视角线、IF 线或不同结局 |
| Block | 最小创作单元，可以是章节、场景、笔记、摘要、设定或大纲 |
| Revision | Block 的一个正文版本，用于保留修改历史和不同写法 |
| Canon | 角色、地点、规则等不应被模型随意违背的硬设定 |
| Memory | 可检索的背景资料、重要经历、对话、线索等语义记忆 |

## 技术栈

- 前端：Vue 3、TypeScript、Vite、Vue Flow、Tiptap、Pinia
- 后端：Go、Gin、pgx
- 数据库：PostgreSQL、pgvector

## 开始之前

请先安装：

- [Docker Engine](https://docs.docker.com/get-docker/) 24.0 或更高版本
- Docker Compose 2.20 或更高版本
- Go 1.26.0 或更高版本
- Node.js 22.12.0 或更高版本
- npm 10.9.0 或更高版本

本项目后端的 Go 版本由 `backend/go.mod` 声明；前端使用 Vite 7，本说明统一采用其支持的 Node.js 22.12+ 版本线作为运行基线。

然后克隆项目并进入仓库目录：

```bash
git clone https://github.com/Alex-J4096/branch-scribe.git
cd branch-scribe
```

## 配置环境变量

复制根目录的环境变量示例：

```bash
cp .env.example .env
```

默认配置如下：

```dotenv
POSTGRES_USER=branchscribe
POSTGRES_PASSWORD=branchscribe
POSTGRES_DB=branchscribe
POSTGRES_PORT=5432
POSTGRES_HOST=localhost

HTTP_ADDR=:8080
APP_ENV=development

BRANCHSCRIBE_MODEL_API_KEY=your-provider-api-key
```

数据库既可以使用上述 `POSTGRES_*` 变量，也可以通过 `DATABASE_URL` 配置完整连接地址。后端从仓库根目录或 `backend` 目录启动时都会自动读取根目录 `.env`。

如需修改前端访问的后端地址：

```bash
cp frontend/.env.example frontend/.env
```

```dotenv
VITE_API_BASE_URL=http://localhost:8080/api
```

## 启动项目

### 1. 启动数据库

```bash
docker compose up -d postgres
```

首次创建容器时，PostgreSQL 会自动执行 `backend/migrations/init` 中的初始化 SQL。数据库数据保存在 Docker volume `branchscribe_pgdata` 中。

### 2. 安装前端依赖

```bash
cd frontend
npm install
cd ..
```

### 3. 启动前后端

推荐使用仓库根目录的启动脚本：

```bash
./start.sh
```

也可以分别启动。

后端：

```bash
cd backend
go run ./cmd/server
```

前端：

```bash
cd frontend
npm run dev
```

服务默认地址：

| 服务 | 地址 |
| --- | --- |
| 前端 | `http://localhost:5173` |
| 后端 API | `http://localhost:8080/api` |
| 健康检查 | `http://localhost:8080/health` |

## 配置 LLM

项目启动后，在首页进入“全局模型配置”，新建一个 LLM Profile：

1. 选择 Provider，并填写模型名称与 Base URL。
2. 填写真实 API Key，或填写环境变量引用，例如 `env:BRANCHSCRIBE_MODEL_API_KEY`。
3. 根据模型能力设置 `temperature`、`top_p`、`max_tokens` 和 `context_window`。
4. 保存模型，并在项目设置中将它选为默认模型。

当前生成层使用 OpenAI-compatible 接口。OpenAI、OpenRouter、DeepSeek、Moonshot、SiliconFlow 等兼容服务可直接配置；其他服务需要提供兼容接口或后续增加 Provider Adapter。

Embedding Profile 是可选项，仅在使用 Memory 语义搜索和 Reindex 时需要。普通写作中的 Memory 自动召回目前使用关键词匹配。

## 第一次创作

推荐按以下顺序熟悉 BranchScribe：

1. 在首页新建 Project。
2. 进入工作台，新建 Chapter 或 Scene Block，并在富文本编辑器中写入正文。
3. 创建角色卡、地点卡和世界规则；在 Block 的 Metadata 中关联本段涉及的角色与地点。
4. 打开 Block 的写作工具，选择模型和写作操作。
5. 在上下文预览中确认本轮将发送给模型的正文、前文、摘要、Canon 和 Memory。
6. 发送指令，并将满意的回复保存为新的正文版本。
7. 从节点派生 Branch 探索其他剧情走向，或在导出与备份页面输出作品。

Canon、Memory、角色状态、伏笔和时间线的具体差异与使用方法，参见 [记忆功能说明书](docs/MEMORY_FEATURES_GUIDE.md)。

## LLM 请求调试

项目提供独立的 LLM Debug Web UI，用于查看后端最终发送给 Provider 的消息、推理流和正文流。

一键启动前端、后端和调试界面：

```bash
./start.sh debug
```

调试界面默认位于 `http://127.0.0.1:6069`。

也可以手动启动：

```bash
cd backend
go run ./cmd/llm-debug
```

然后在另一个终端让后端上报调试事件：

```bash
cd backend
LLM_DEBUG_URL=http://127.0.0.1:6069 go run ./cmd/server
```

可通过 `LLM_DEBUG_ADDR`（启动脚本）或 `-addr`（调试程序）修改监听地址。调试服务停止后，正文生成仍会正常继续。

## 常用开发命令

后端测试：

```bash
cd backend
go test ./...
```

前端类型检查与构建：

```bash
cd frontend
npm run typecheck
npm run build
```

停止数据库：

```bash
docker compose down
```

如需同时删除本地数据库数据，请明确确认数据不再需要后再删除对应 Docker volume。

## 项目结构

```text
branch-scribe/
├── backend/                 # Go API、业务模块和数据库初始化脚本
│   ├── cmd/server/          # 后端服务入口
│   ├── cmd/llm-debug/       # LLM 请求调试工具
│   ├── internal/            # 业务实现
│   └── migrations/init/     # PostgreSQL 初始化 SQL
├── frontend/                # Vue 3 前端
├── docs/
│   ├── ARCHITECTURE.md      # 架构、数据模型与开发任务清单
│   ├── DEVELOPMENT_LOG.md   # 开发日志
│   └── MEMORY_FEATURES_GUIDE.md
├── docker-compose.yml       # PostgreSQL + pgvector
└── start.sh                 # 本地前后端启动脚本
```

## 常见问题

### 后端提示数据库配置缺失或连接失败

先确认仓库根目录存在 `.env`，且启动命令位于仓库根目录或 `backend` 目录。然后检查 PostgreSQL 容器状态：

```bash
docker compose ps
docker compose logs postgres
```

### 前端无法访问 API

确认后端健康检查可访问，并检查 `frontend/.env` 中的 `VITE_API_BASE_URL`。修改前端环境变量后需要重启 Vite。

### 模型显示未配置 API Key

如果 Profile 中使用 `env:变量名`，对应变量必须存在于后端进程的环境中；写在根目录 `.env` 后需要重启后端。

### 新增初始化 SQL 后没有生效

`docker-entrypoint-initdb.d` 只会在数据库 volume 首次初始化时执行。已有数据环境应使用兼容迁移，或在确认可以丢弃本地数据后重建数据库 volume。

## 相关文档

- [架构与开发任务](docs/ARCHITECTURE.md)
- [开发日志](docs/DEVELOPMENT_LOG.md)
- [记忆功能说明书](docs/MEMORY_FEATURES_GUIDE.md)

## 许可证

本项目采用 [MIT License](LICENSE)。
