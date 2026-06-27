# BranchScribe 开发文档

## 1. 项目定位

BranchScribe 是一个基于 LLM API 的非线性小说撰写工具。

它不是传统 Chatbox，而是一个面向长篇小说创作的文本工程系统。核心目标是帮助作者管理：

* 小说正文 block
* 多版本改写
* 故事分支
* 角色线
* 世界观设定
* 前文摘要
* 上下文编排
* LLM 续写、改写、扩写、局部修改
* 设定一致性检查

传统 Chatbox 的主要问题是：对话历史是线性的，难以管理小说的分支、版本、回填和多角色线。BranchScribe 采用类似蓝图或 ComfyUI 的节点图界面，让用户通过图结构管理故事结构，通过富文本编辑器管理正文，通过上下文构建器决定每次传给 LLM 的内容。

---

## 2. 项目目标

### 2.1 核心目标

实现一个可以用于实际小说创作的 LLM 写作 IDE，支持：

1. 将小说拆分为多个 block。
2. 每个 block 支持多版本 revision。
3. 用户可以 fork 一个 block，生成不同走向。
4. 用户可以比较两个 revision，选择一个继续写作。
5. 用户可以对 block 进行续写、改写、局部修改、扩写、缩写。
6. 用户可以管理角色设定、世界观设定、地点设定、事件设定。
7. 系统可以根据当前写作位置自动构建 LLM 上下文。
8. 长篇内容超过上下文限制时，系统自动使用摘要和记忆代替全文。
9. 用户侧始终能看到完整正文，LLM 侧看到经过压缩和编排的上下文。
10. 支持配置模型参数，例如 temperature、top_p、max_tokens、context_window 等。

### 2.2 非目标

MVP 阶段暂不做：

* 本地 vLLM 部署。
* Ollama 本地模型管理。
* 多人协作编辑。
* 移动端适配。
* 商业化支付系统。
* 云同步。
* EPUB 高级排版。
* 复杂 Agent 自动写完整本小说。

项目优先使用服务商 API，例如 OpenAI-compatible API、OpenAI、Anthropic、Google、DeepSeek、Moonshot、OpenRouter、SiliconFlow 等。

---

## 3. 推荐技术栈

### 3.1 前端

* Vue 3
* TypeScript
* Vite
* Vue Flow
* Tiptap
* Pinia
* Vue Router
* TanStack Query for Vue
* UnoCSS 或 Tailwind CSS
* Monaco Editor，可选，用于 prompt 模板编辑

### 3.2 后端

* Go
* Gin 或 Echo
* PostgreSQL
* pgvector
* Redis，可选，MVP 可以不引入
* SSE 或 WebSocket，用于 LLM 流式输出
* sqlc 或 Ent，用于数据库访问
* Goose 或 Atlas，用于数据库迁移

### 3.3 LLM API 层

MVP 阶段建议先实现一个 Go 版本的 OpenAI-compatible client。

后续可以扩展为多 Provider Adapter：

* OpenAI-compatible Provider
* OpenAI Provider
* Anthropic Provider
* Gemini Provider
* OpenRouter Provider
* DeepSeek Provider
* Moonshot Provider

MVP 不需要引入 vLLM、Ollama、LangGraph。

### 3.4 向量检索

MVP 优先使用：

* PostgreSQL + pgvector

理由：

* 主数据和向量数据可以放在同一个数据库。
* 便于事务管理。
* 便于用 SQL 过滤 project_id、entity_type、branch_id 等元数据。
* 部署复杂度低。

后期如果向量检索量变大，可以再接入 Qdrant。

---

## 4. 总体架构

```text
Frontend: Vue 3
  ├── Project Workspace
  ├── Graph Canvas: Vue Flow
  ├── Block Editor: Tiptap
  ├── Revision Diff Viewer
  ├── Context Preview Panel
  ├── Model Config Panel
  └── Canon / Memory Manager

Backend: Go API
  ├── Project Service
  ├── Graph Service
  ├── Block Service
  ├── Revision Service
  ├── Branch Service
  ├── Canon Service
  ├── Memory Service
  ├── Summary Service
  ├── Context Builder
  ├── LLM Gateway
  └── Generation Run Recorder

Database: PostgreSQL + pgvector
  ├── projects
  ├── branches
  ├── blocks
  ├── block_revisions
  ├── graph_edges
  ├── canon_entities
  ├── memory_chunks
  ├── summary_snapshots
  ├── model_profiles
  ├── prompt_templates
  └── generation_runs
```

---

## 5. 核心概念

### 5.1 Project

Project 表示一部小说或一个写作项目。

包含：

* 项目名称
* 简介
* 默认语言
* 默认风格
* 默认模型配置
* 全局写作规则
* 全局世界观设定

### 5.2 Branch

Branch 表示一条故事线。

例如：

* 主线
* 女主视角线
* 男主视角线
* if 线
* 废案线
* 第二结局线

Branch 不应该复制整本小说，而是引用一组 block 路径。

### 5.3 Block

Block 是小说正文的最小创作单元。Block 可以只是片段，`title` 允许为空，界面应为无标题片段提供自动显示名。

一个 block 可以是：

* scene
* chapter
* note
* summary
* canon
* outline

MVP 阶段建议主要支持 scene 和 chapter。

### 5.4 Revision

Revision 是 block 的一个具体版本。

每次用户手动修改、LLM 改写、LLM 续写，都应该生成一个新的 revision，而不是直接覆盖原文。

这样可以支持：

* 历史回滚
* 版本对比
* 分支选择
* LLM 生成记录追溯
* 不同走向并行展开

### 5.5 Canon Entity

Canon Entity 是硬设定。

例如：

* 角色
* 地点
* 阵营
* 物品
* 世界规则
* 时间线事件

这类内容不应该只靠向量检索，而应该结构化保存。

### 5.6 Memory Chunk

Memory Chunk 是可以被 RAG 检索的语义记忆。

例如：

* 某角色过去经历
* 重要对话
* 伏笔
* 地点描写
* 章节摘要
* 世界观背景片段

### 5.7 Summary Snapshot

Summary Snapshot 是对 block、chapter、branch、arc 或 character line 的摘要快照。

摘要必须记录覆盖了哪些 revision，否则用户修改前文后，摘要可能过期。

### 5.8 Generation Run

Generation Run 记录一次 LLM 调用。

需要保存：

* 使用的模型
* 模型参数
* prompt 模板
* 输入上下文快照
* 输出 revision
* token 用量
* 耗时
* 错误信息

---

## 6. 数据库设计

### 6.1 projects

```sql
CREATE TABLE projects (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    default_language TEXT DEFAULT 'zh',
    default_style_profile JSONB DEFAULT '{}'::jsonb,
    default_model_profile_id UUID,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.2 branches

```sql
CREATE TABLE branches (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    base_branch_id UUID REFERENCES branches(id),
    fork_from_block_id UUID,
    fork_from_revision_id UUID,
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.3 blocks

```sql
CREATE TABLE blocks (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_id UUID REFERENCES branches(id) ON DELETE SET NULL,
    type TEXT NOT NULL,
    title TEXT,
    current_revision_id UUID,
    parent_block_id UUID REFERENCES blocks(id),
    position_x DOUBLE PRECISION DEFAULT 0,
    position_y DOUBLE PRECISION DEFAULT 0,
    order_index INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.4 block_revisions

```sql
CREATE TABLE block_revisions (
    id UUID PRIMARY KEY,
    block_id UUID NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    parent_revision_id UUID REFERENCES block_revisions(id),
    content TEXT NOT NULL,
    content_format TEXT NOT NULL DEFAULT 'markdown',
    content_hash TEXT,
    source TEXT NOT NULL,
    generation_run_id UUID,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.5 graph_edges

```sql
CREATE TABLE graph_edges (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_block_id UUID NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    target_block_id UUID NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    edge_type TEXT NOT NULL,
    label TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.6 canon_entities

```sql
CREATE TABLE canon_entities (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    aliases TEXT[] DEFAULT '{}',
    description TEXT,
    attributes JSONB DEFAULT '{}'::jsonb,
    importance INTEGER DEFAULT 5,
    status TEXT NOT NULL DEFAULT 'canon',
    embedding VECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.7 memory_chunks

```sql
CREATE TABLE memory_chunks (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    source_id UUID,
    chunk_text TEXT NOT NULL,
    chunk_kind TEXT NOT NULL,
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}'::jsonb,
    embedding VECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.8 summary_snapshots

```sql
CREATE TABLE summary_snapshots (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    summary_text TEXT NOT NULL,
    covered_revision_ids UUID[] DEFAULT '{}',
    token_count INTEGER DEFAULT 0,
    model TEXT,
    status TEXT NOT NULL DEFAULT 'valid',
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.9 model_profiles

```sql
CREATE TABLE model_profiles (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    base_url TEXT,
    api_key_ref TEXT,
    temperature DOUBLE PRECISION DEFAULT 0.8,
    top_p DOUBLE PRECISION DEFAULT 0.9,
    max_tokens INTEGER DEFAULT 2048,
    context_window INTEGER DEFAULT 32768,
    profile_type TEXT NOT NULL DEFAULT 'llm',
    embedding_profile_id UUID REFERENCES model_profiles(id) ON DELETE SET NULL,
    embedding_dimensions INTEGER,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.10 prompt_templates

```sql
CREATE TABLE prompt_templates (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    task_type TEXT NOT NULL,
    template_text TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    is_default BOOLEAN DEFAULT false,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 6.11 generation_runs

```sql
CREATE TABLE generation_runs (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    block_id UUID REFERENCES blocks(id) ON DELETE SET NULL,
    task_type TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    temperature DOUBLE PRECISION,
    top_p DOUBLE PRECISION,
    max_tokens INTEGER,
    context_window INTEGER,
    prompt_template_id UUID REFERENCES prompt_templates(id),
    input_context_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_revision_id UUID,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 7. 后端模块设计

### 7.1 Project Service

负责：

* 创建项目
* 更新项目
* 删除项目
* 查询项目列表
* 查询项目详情
* 设置默认模型配置
* 设置默认风格配置

### 7.2 Graph Service

负责：

* 获取项目节点图
* 创建节点
* 更新节点位置
* 创建边
* 删除边
* 删除节点
* 根据 branch 查询故事路径

### 7.3 Block Service

负责：

* 创建 block
* 更新 block 标题和元数据
* 删除 block
* 查询 block 详情
* 查询 block 当前 revision
* 设置 block 当前 revision

### 7.4 Revision Service

负责：

* 创建 revision
* 查询 revision 列表
* 查询 revision 详情
* 对比两个 revision
* 回滚到指定 revision
* 将某个 revision 设置为当前版本

### 7.5 Branch Service

负责：

* 创建 branch
* 从某个 block fork branch
* 查询 branch 列表
* 查询 branch path
* 归档 branch
* 合并 branch，MVP 可只做手动合并

### 7.6 Canon Service

负责：

* 创建角色、地点、世界观规则等 canon entity
* 编辑 canon entity
* 查询 canon entity
* 根据类型、标签、名称检索 canon entity
* 为 canon entity 生成 embedding
* 标记 canon entity 状态：canon、draft、deprecated

### 7.7 Memory Service

负责：

* 从 block revision 生成 memory chunk
* 从 summary 生成 memory chunk
* 手动创建 memory chunk
* 语义检索 memory chunk
* 通过 project_id、branch_id、tags、chunk_kind 过滤记忆

### 7.8 Summary Service

负责：

* 为 block 生成摘要
* 为 chapter 生成摘要
* 为 branch 生成摘要
* 检查摘要是否过期
* 当 covered_revision_ids 发生变化时，将旧摘要标记为 stale
* 按需重新生成摘要

### 7.9 Context Builder

这是项目的核心模块。

输入：

```json
{
  "project_id": "...",
  "branch_id": "...",
  "block_id": "...",
  "task_type": "continue",
  "selected_text": "",
  "user_instruction": "继续描写女主进入地下车站后的场景",
  "model_profile_id": "..."
}
```

输出：

```json
{
  "system_prompt": "...",
  "user_prompt": "...",
  "context_items": [
    {
      "type": "recent_block",
      "title": "上一段正文",
      "content": "..."
    },
    {
      "type": "canon_entity",
      "title": "角色设定：艾莉娅",
      "content": "..."
    },
    {
      "type": "summary",
      "title": "当前章节摘要",
      "content": "..."
    }
  ],
  "estimated_input_tokens": 12000,
  "token_budget": 32000
}
```

Context Builder 需要根据 task_type 决定上下文策略。

---

## 8. LLM 任务类型

MVP 阶段支持以下任务：

### 8.1 free_write

完全根据用户指令生成正文，不读取当前 block 正文。

上下文应包含：

* 项目简介
* 用户指令

### 8.2 continue

续写当前 block 或从当前 block 创建下一个 block。

上下文应包含：

* 当前 block 正文
* 前 1 到 3 个 block 正文
* 当前 branch 摘要
* 当前 chapter 摘要
* 相关角色设定
* 相关地点设定
* 相关世界规则
* 用户指令

### 8.3 rewrite_block

重写整个 block。

上下文应包含：

* 原 block 正文
* 用户改写要求
* 当前 block 前后少量上下文
* 不能改变的 canon facts
* 风格要求

### 8.4 rewrite_selection

局部修改。

上下文应包含：

* 原 block 全文
* 用户选中的文本
* 选中文本前后上下文
* 用户修改要求
* 不能改变的 canon facts

输出应该只返回替换后的局部文本，或者返回结构化 JSON：

```json
{
  "replacement": "...",
  "notes": "..."
}
```

### 8.5 expand

扩写当前 block 或选中片段。

### 8.6 condense

压缩当前 block 或选中片段。

### 8.7 polish

润色语言，不改变剧情事实。

### 8.8 compare_revisions

比较两个 revision。

输出评价维度：

* 连贯性
* 角色一致性
* 设定一致性
* 情绪张力
* 节奏
* 可继续展开性
* 推荐选择

### 8.8 check_consistency

检查设定冲突。

输出：

```json
{
  "conflicts": [
    {
      "type": "character_state",
      "severity": "medium",
      "description": "...",
      "suggestion": "..."
    }
  ]
}
```

### 8.9 summarize

生成摘要。

摘要类型：

* block summary
* chapter summary
* branch summary
* character line summary

---

## 9. LLM Provider 设计

### 9.1 接口设计

```go
type ChatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type GenerateRequest struct {
    Provider      string        `json:"provider"`
    Model         string        `json:"model"`
    BaseURL       string        `json:"base_url"`
    APIKey        string        `json:"-"`
    Messages      []ChatMessage `json:"messages"`
    Temperature   float64       `json:"temperature"`
    TopP          float64       `json:"top_p"`
    MaxTokens     int           `json:"max_tokens"`
    Stream        bool          `json:"stream"`
}

type TokenEvent struct {
    Type    string `json:"type"`
    Content string `json:"content"`
    Error   string `json:"error,omitempty"`
}

type LLMProvider interface {
    GenerateStream(ctx context.Context, req GenerateRequest) (<-chan TokenEvent, error)
    GenerateOnce(ctx context.Context, req GenerateRequest) (string, error)
}
```

### 9.2 MVP Provider

先实现 OpenAI-compatible provider。

支持：

* base_url
* api_key
* model
* temperature
* top_p
* max_tokens
* stream

Provider 列表包含 OpenAI-compatible、OpenAI、OpenRouter、DeepSeek、Moonshot、SiliconFlow 等。

前端允许用户创建多个 model profile。

例如：

```json
{
  "name": "DeepSeek Writer",
  "provider": "openai_compatible",
  "base_url": "https://api.deepseek.com",
  "model": "deepseek-chat",
  "temperature": 0.85,
  "top_p": 0.9,
  "max_tokens": 4096,
  "context_window": 64000
}
```

---

## 10. 上下文构建策略

### 10.1 Token Budget

Context Builder 应根据模型 context_window 分配预算。

默认比例：

```text
system prompt: 10%
user instruction: 10%
current block or selected text: 20%
recent blocks: 20%
summaries: 15%
canon entities: 15%
retrieved memories: 10%
```

### 10.2 上下文优先级

从高到低：

1. 用户当前指令
2. 当前选中文本
3. 当前 block 正文
4. 当前 block 前后邻接正文
5. 硬设定 canon facts
6. 当前角色状态
7. 当前章节摘要
8. 当前分支摘要
9. 向量检索记忆
10. 较远章节摘要

### 10.3 硬设定不要只靠向量检索

角色年龄、阵营关系、世界规则、关键事件等内容必须结构化读取。

例如续写时，如果当前 block metadata 中标记了角色：

```json
{
  "characters": ["艾莉娅", "诺兰"],
  "location": "地下车站"
}
```

Context Builder 应直接加载这两个角色和地点，而不是只靠 embedding 相似度搜索。

### 10.4 摘要失效机制

当某个 block 创建了新的 current revision 后：

1. 找出所有覆盖旧 revision 的 summary_snapshots。
2. 将这些 summary 标记为 stale。
3. 在下次生成前，如果需要该摘要，则触发重新摘要。
4. MVP 阶段可以先手动刷新摘要，后续再做自动刷新。

---

## 11. 前端界面设计

### 11.1 主布局

```text
┌──────────────────────────────────────────────────────────────┐
│ Top Bar: Project / Branch / Model Profile / Save Status       │
├───────────────┬──────────────────────────┬───────────────────┤
│ Left Sidebar  │ Graph Canvas              │ Right Inspector   │
│               │                          │                   │
│ Project Tree  │ Vue Flow Nodes            │ Block Editor      │
│ Branches      │ Block Connections         │ Revision List     │
│ Canon          │ Story Branches            │ LLM Actions       │
│ Memory         │                          │ Context Preview   │
└───────────────┴──────────────────────────┴───────────────────┘
```

### 11.2 左侧栏

功能：

* 项目列表
* 分支列表
* 章节列表
* 角色设定
* 世界观设定
* 地点设定
* 伏笔列表，Phase 3
* 摘要列表

### 11.3 中间蓝图画布

使用 Vue Flow。

节点类型：

* Chapter Node
* Scene Block Node
* Fork Node
* Summary Node
* Canon Node，可选

节点显示：

* 标题
* 类型
* 字数
* 当前 revision 数量
* 是否有摘要
* 是否存在设定冲突
* 是否已过期

边类型：

* next
* fork
* alternative
* references
* summarizes

### 11.4 右侧 Inspector

点击 block node 后显示：

* block 标题
* Tiptap 正文编辑器
* revision 列表
* 当前 revision
* LLM 操作按钮
* 模型参数覆盖设置
* 上下文预览
* 生成记录

### 11.5 LLM 操作按钮

MVP 支持：

* 续写
* 改写
* 局部修改
* 扩写
* 缩写
* 润色
* 生成两个候选版本
* 比较两个版本
* 生成摘要

### 11.6 Diff Viewer

用于比较两个 revision。

可以先使用文本 diff，后续再做富文本 diff。

MVP 推荐库：

* diff-match-patch
* jsdiff

---

## 12. API 设计

### 12.1 Project API

```http
GET    /api/projects
POST   /api/projects
GET    /api/projects/:projectId
PATCH  /api/projects/:projectId
DELETE /api/projects/:projectId
```

### 12.2 Branch API

```http
GET    /api/projects/:projectId/branches
POST   /api/projects/:projectId/branches
POST   /api/projects/:projectId/branches/fork
GET    /api/branches/:branchId/path
PATCH  /api/branches/:branchId
DELETE /api/branches/:branchId
```

### 12.3 Block API

```http
GET    /api/projects/:projectId/blocks
POST   /api/projects/:projectId/blocks
GET    /api/blocks/:blockId
PATCH  /api/blocks/:blockId
PATCH  /api/blocks/:blockId/associations
DELETE /api/blocks/:blockId
POST   /api/blocks/:blockId/fork
```

### 12.4 Revision API

```http
GET    /api/blocks/:blockId/revisions
POST   /api/blocks/:blockId/revisions
GET    /api/revisions/:revisionId
POST   /api/blocks/:blockId/revisions/:revisionId/select
POST   /api/revisions/compare
```

### 12.5 Graph API

```http
GET    /api/projects/:projectId/graph
POST   /api/projects/:projectId/graph/edges
PATCH  /api/projects/:projectId/graph/nodes/:blockId/position
PATCH  /api/projects/:projectId/graph/edges/:edgeId
DELETE /api/projects/:projectId/graph/edges/:edgeId
```

### 12.6 Canon API

```http
GET    /api/projects/:projectId/canon
POST   /api/projects/:projectId/canon
GET    /api/canon/:entityId
PATCH  /api/canon/:entityId
DELETE /api/canon/:entityId
```

### 12.7 Prompt Template API

```http
GET    /api/projects/:projectId/prompt-templates
POST   /api/projects/:projectId/prompt-templates
GET    /api/prompt-templates/:templateId
PATCH  /api/prompt-templates/:templateId
DELETE /api/prompt-templates/:templateId
```

### 12.8 Memory API

```http
GET    /api/projects/:projectId/memory
POST   /api/projects/:projectId/memory
POST   /api/blocks/:blockId/memory
GET    /api/memory/:memoryId
PATCH  /api/memory/:memoryId
DELETE /api/memory/:memoryId
POST   /api/projects/:projectId/memory/search
POST   /api/projects/:projectId/memory/reindex
```

### 12.8 Summary API

```http
GET    /api/projects/:projectId/summaries
POST   /api/blocks/:blockId/summarize
POST   /api/branches/:branchId/summarize
POST   /api/summaries/:summaryId/refresh
```

### 12.9 Generation API

```http
POST   /api/generate/stream
POST   /api/generate/once
POST   /api/generate/candidates
POST   /api/generate/context-preview
GET    /api/projects/:projectId/generation-runs
GET    /api/generation-runs/:runId
```

---

## 13. Prompt 模板

### 13.1 模板变量

Prompt Template 支持变量：

```text
{{project_description}}
{{style_profile}}
{{canon_facts}}
{{character_cards}}
{{location_cards}}
{{recent_context}}
{{branch_summary}}
{{chapter_summary}}
{{retrieved_memories}}
{{current_block}}
{{selected_text}}
{{user_instruction}}
```

### 13.2 续写模板示例

```text
你是一个小说创作助手。请严格遵守已有设定和当前故事线。

写作风格：
{{style_profile}}

硬设定：
{{canon_facts}}

相关角色：
{{character_cards}}

相关地点：
{{location_cards}}

前文摘要：
{{branch_summary}}

最近正文：
{{recent_context}}

当前正文：
{{current_block}}

用户要求：
{{user_instruction}}

请继续创作下一段正文。不要解释，不要输出分析过程，只输出小说正文。
```

### 13.3 局部修改模板示例

```text
你是一个小说文本编辑助手。请只修改用户选中的片段，不要改变剧情事实。

硬设定：
{{canon_facts}}

完整 block：
{{current_block}}

需要修改的片段：
{{selected_text}}

用户要求：
{{user_instruction}}

请只输出替换后的片段，不要输出解释。
```

### 13.4 设定一致性检查模板示例

```text
请检查以下正文是否和已知设定冲突。

硬设定：
{{canon_facts}}

角色状态：
{{character_cards}}

前文摘要：
{{branch_summary}}

待检查正文：
{{current_block}}

请输出 JSON：
{
  "conflicts": [
    {
      "type": "...",
      "severity": "low | medium | high",
      "description": "...",
      "suggestion": "..."
    }
  ]
}
```

---

## 14. 推荐仓库结构

```text
branchscribe/
  README.md
  docs/
    development.md
    api.md
    database.md
    prompt-templates.md
  frontend/
    package.json
    vite.config.ts
    src/
      main.ts
      app.vue
      router/
      stores/
      api/
      components/
        graph/
        editor/
        inspector/
        diff/
        canon/
        memory/
      views/
        ProjectWorkspace.vue
        ProjectList.vue
      types/
  backend/
    go.mod
    cmd/
      server/
        main.go
    internal/
      config/
      database/
      middleware/
      llm/
      project/
      branch/
      block/
      revision/
      graph/
      canon/
      memory/
      summary/
      contextbuilder/
      generation/
    migrations/
    tests/
  docker-compose.yml
  .env.example
  .gitignore
```

---

## 15. 分阶段任务清单

任务状态约定：

* `[ ]` 未开始
* `[x]` 已完成

最近更新记录详见 [DEVELOPMENT_LOG.md](./DEVELOPMENT_LOG.md)。

## Phase 0: 项目初始化

目标：建立基础工程结构。

### 后端任务

* [x] 初始化 Go module。
* [x] 选择 Gin 或 Echo。
* [x] 添加配置系统，支持 `.env`。
* [x] 添加 PostgreSQL 连接。
* [ ] 添加数据库 migration 工具。
* [x] 创建基础 health check API。
* [x] 创建统一错误响应格式。
* [x] 创建日志模块。
* [x] 创建 request id middleware。
* [x] 创建 CORS middleware。

### 前端任务

* [x] 初始化 Vue 3 + TypeScript + Vite。
* [x] 添加 Vue Router。
* [x] 添加 Pinia。
* [x] 添加 TanStack Query for Vue。
* [x] 添加基础布局。
* [x] 添加 API client。
* [x] 添加环境变量配置。
* [x] 添加基础页面：ProjectList、ProjectWorkspace。

### DevOps 任务

* [x] 添加 `docker-compose.yml`。
* [x] 添加 PostgreSQL 服务。
* [x] 添加 pgvector 扩展初始化。
* [x] 添加 `.env.example`。
* [x] 添加 README 基础说明。
* [ ] 添加前后端启动脚本。

### 验收标准

* [x] 前端可以启动。
* [x] 后端可以启动。
* [x] 后端可以连接数据库。
* [x] `/health` 返回正常。
* [ ] docker-compose 可以启动数据库。

---

## Phase 1: 核心写作数据模型

目标：完成 project、branch、block、revision、graph edge 的基本 CRUD。

### 数据库任务

* [x] 创建 projects 表。
* [x] 创建 branches 表。
* [x] 创建 blocks 表。
* [x] 创建 block_revisions 表。
* [x] 创建 graph_edges 表。
* [x] 添加必要索引。
* [x] 添加 updated_at 自动更新逻辑，或在服务层处理。

### 后端任务

* [x] 实现 Project Service。
* [x] 实现 Branch Service。
* [x] 实现 Block Service。
* [x] 实现 Revision Service。
* [x] 实现 Graph Service。
* [x] 实现创建 project 时自动创建默认 branch。
* [x] 实现创建 block 时自动创建初始 revision。
* [x] 实现选择 revision 作为 current revision。
* [x] 实现 fork block。
* [x] 实现 revision 列表查询。
* [x] 实现 graph 查询接口。

### 前端任务

* [x] 实现项目列表。
* [x] 实现创建项目弹窗。
* [x] 实现项目工作台页面。
* [x] 集成 Vue Flow。
* [x] 展示 block node。
* [x] 支持拖动节点并保存位置。
* [x] 支持创建 block。
* [x] 支持 block 列表管理、快速选择和删除。
* [x] 支持通过节点边缘热区拖拽吸附创建 edge，保留表单创建作为备用。
* [x] graph edge 在画布上以可见线条、箭头和标签展示。
* [x] 拖动 block 后 graph edge 保持可见并跟随节点位置更新。
* [x] 左右菜单支持抽屉式收起和展开。
* [x] 抽屉中的工作区和 inspector 功能组可以独立收起。
* [x] 点击 block 后在右侧显示详情。
* [x] 显示 revision 列表。
* [x] 支持选择 revision。

### 验收标准

* [x] 用户可以创建项目。
* [x] 用户可以创建 block。
* [x] 用户可以通过列表查看、快速选择和删除 block。
* [x] 用户可以收起左右抽屉，让画布获得更多空间。
* [x] 用户可以在图上看到 block。
* [x] 用户可以拖动 block 并保存位置。
* [x] 用户可以通过节点边缘热区拖拽吸附或备用表单创建 edge 连接两个 block。
* [x] 用户可以创建多个 revision。
* [x] 用户可以选择某个 revision 作为当前版本。
* [x] 用户可以 fork 一个 block。

---

## Phase 2: 富文本编辑与版本对比

目标：让用户真正能够写作、修改、对比和回滚。

### 前端任务

* [x] 集成 Tiptap。
* [x] 实现 block 正文编辑器。
* [x] 支持手动保存为新 revision。
* [x] 支持自动保存草稿，可选。
* [x] 实现 revision diff viewer。
* [x] 支持选择两个 revision 进行对比。
* [x] 支持将旧 revision 恢复为当前 revision。
* [x] 支持 block 标题编辑。
* [x] 支持 block 字数统计。
* [x] 支持当前 revision 状态显示。

### 后端任务

* [x] 实现 content_hash。
* [x] 实现 revision diff API，可选，也可以前端 diff。
* [x] 实现 revision rollback。
* [x] 实现 revision metadata。
* [x] 保存 revision source：user、llm、import。

### 验收标准

* [x] 用户可以在 Tiptap 中编辑正文。
* [x] 每次保存都会创建新 revision。
* [x] 用户可以查看历史版本。
* [x] 用户可以比较两个版本差异。
* [x] 用户可以回滚到旧版本。
* [x] 用户可以 fork 后在分支上继续写。

---

## Phase 3: LLM API 接入

目标：接入服务商 API，实现续写、改写、局部修改和流式输出。

### 数据库任务

* [x] 创建 model_profiles 表。
* [x] 创建 prompt_templates 表。
* [x] 创建 generation_runs 表。

### 后端任务

* [x] 实现 Model Profile CRUD。
* [x] 实现 Prompt Template CRUD。
* [x] 实现 OpenAI-compatible Provider。
* [x] 实现 GenerateOnce。
* [x] 实现 GenerateStream。
* [x] 实现 SSE 或 WebSocket 流式输出。
* [x] 实现 generation run 记录。
* [x] 实现 LLM 输出保存为新 revision。
* [x] 实现任务类型：free_write。
* [x] 实现任务类型：continue。
* [x] 实现任务类型：rewrite_block。
* [x] 实现任务类型：rewrite_selection。
* [x] 实现任务类型：expand。
* [x] 实现任务类型：condense。
* [x] 实现任务类型：polish。

### 前端任务

* [x] 实现模型配置页面。
* [x] 支持配置 provider、base_url、api_key、model。
* [x] 支持配置 temperature。
* [x] 支持配置 top_p。
* [x] 支持配置 max_tokens。
* [x] 支持配置 context_window。
* [x] 在 block inspector 中添加 LLM 操作按钮。
* [x] 实现用户指令输入框。
* [x] 实现流式输出显示。
* [x] 生成完成后允许保存为新 revision。
* [x] 支持局部选中文本后执行 rewrite_selection。

### 安全任务

* [x] API key 不返回给前端。
* [x] MVP 阶段支持在模型配置中保存 API key，且不写入 generation_runs 或日志；也支持 `env:VAR_NAME` 引用。
* [x] 对 LLM 请求做超时控制。
* [x] 对服务商 API 错误做清晰提示。

### 验收标准

* [x] 用户可以配置一个 OpenAI-compatible 模型。
* [x] 用户可以通过 API 对 block 执行续写。
* [x] 用户可以通过 API 对 block 执行改写。
* [x] 用户可以选中文本执行局部修改。
* [x] LLM 输出可以流式显示。
* [x] 生成结果可以保存为新 revision。
* [x] 每次 LLM 调用都有 generation run 记录。

---

## Phase 4: Canon 与全局记忆

目标：让系统知道角色、地点、世界观等硬设定。

### 数据库任务

* [x] 创建 canon_entities 表。
* [x] 创建 memory_chunks 表。
* [x] 启用 pgvector。
* [x] 为 canon_entities 添加 embedding。
* [x] 为 memory_chunks 添加 embedding。
* [x] 添加 project_id、type、status、tags 索引。

### 后端任务

* [x] 实现 Canon Entity CRUD。
* [x] 支持 entity 类型：character、location、faction、item、rule、event。
* [x] 实现 Memory Chunk CRUD。
* [x] 实现 embedding provider。
* [x] 实现 memory semantic search。
* [x] 实现 canon entity keyword search。
* [x] 支持为 block 关联 characters、location、tags。
* [x] 支持从 block 生成 memory chunk。
* [x] 支持手动创建 memory chunk。

### 前端任务

* [x] 实现角色设定页面。
* [x] 实现地点设定页面。
* [x] 实现世界规则页面。
* [x] 实现 memory 列表页面。
* [x] 在 block inspector 中添加 metadata 编辑。
* [x] 支持为 block 选择出现角色。
* [x] 支持为 block 选择地点。
* [x] 支持手动触发 reindex。
* [x] 显示当前 block 关联的 canon entities。

### 验收标准

* [x] 用户可以创建角色卡。
* [x] 用户可以创建地点卡。
* [x] 用户可以创建世界观规则。
* [x] 用户可以将角色和地点关联到 block。
* [x] LLM 生成时可以读取相关 canon。
* [x] 系统可以通过关键词或向量语义检索相关 memory chunks。

---

## Phase 5: Context Builder 与上下文预览

目标：实现项目核心能力。每次生成前，系统自动构建上下文，并让用户看到将发送给 LLM 的内容。

### 后端任务

* [x] 实现 Context Builder。
* [x] 支持按 task_type 构建上下文。
* [x] 支持 token budget。
* [x] 支持加载 current block。
* [x] 支持加载 recent blocks。
* [x] 支持加载 branch summary。
* [x] 支持加载 chapter summary。
* [x] 支持加载 canon entities。
* [x] 支持加载 memory chunks。
* [x] 支持上下文裁剪。
* [x] 支持 context preview API。
* [x] 将 context snapshot 保存到 generation_runs。

### 前端任务

* [x] 实现 Context Preview Panel。
* [x] 展示 system prompt。
* [x] 展示 user prompt。
* [x] 展示上下文来源。
* [x] 展示预计 token 数。
* [x] 展示每个上下文 item 的类型。
* [x] 支持用户临时取消某个 context item。
* [x] 支持用户查看最终 prompt。
* [x] 将 Block 详情、正文和 LLM 操作整合为可折叠、可关闭的画布浮动标签窗口。

### 验收标准

* [x] 用户执行生成前可以看到上下文预览。
* [x] 续写时自动包含最近正文。
* [x] 改写时自动包含原文和设定。
* [x] 局部修改时自动包含选中文本和前后文。
* [x] LLM 调用记录包含 input_context_snapshot。

---

## Phase 6: 摘要系统

目标：解决长篇小说上下文过长的问题。

### 数据库任务

* [x] 完善 summary_snapshots 表。
* [x] 添加 summary status：valid、stale、failed。
* [x] 添加 covered_revision_ids。
* [x] 添加 target_type 索引。
* [x] 添加 target_id 索引。

### 后端任务

* [x] 实现 block summary。
* [x] 实现 chapter summary。
* [x] 实现 branch summary。
* [x] 实现 summary refresh。
* [x] 实现 summary stale 检测。
* [x] 当 block current revision 改变时，标记相关 summary stale。
* [x] Context Builder 优先使用 valid summary。
* [x] 如果 summary stale，返回提示，MVP 可不自动刷新。

### 前端任务

* [x] 在 block 上显示摘要状态。
* [x] 在 chapter 或 branch 上显示摘要状态。
* [x] 支持手动生成摘要。
* [x] 支持手动刷新摘要。
* [x] 在 Context Preview 中展示摘要来源。
* [x] 提示用户哪些摘要已过期。

### 验收标准

* [x] 用户可以为 block 生成摘要。
* [x] 用户可以为 branch 生成摘要。
* [x] 修改 block 后，相关摘要会被标记为 stale。
* [x] LLM 生成时可以使用摘要代替远距离全文。
* [x] 用户侧仍然可以查看完整正文。

---

## Phase 7: 分支写作增强

目标：完善 fork、分支线、候选版本比较。

### 后端任务

* [x] 实现 branch path 查询。
* [x] 支持从任意 block 创建新 branch。
* [x] 支持将某个 revision 作为 branch 起点。
* [x] 实现 compare_revisions LLM task。
* [x] 实现生成两个候选版本。
* [x] 支持将候选版本分别保存为不同 revision。
* [x] 支持将候选版本展开为不同 block 或 branch。

### 前端任务

* [x] 在图上明确显示 fork edge。
* [x] 显示 branch 颜色。
* [x] 支持切换当前 branch。
* [x] 支持从 block 右键 fork。
* [x] 支持一键生成两个候选版本。
* [x] 支持候选版本并排比较。
* [x] 支持选择某个候选版本继续故事线。
* [x] 支持归档不使用的 branch。

### 验收标准

* [x] 用户可以从某个节点直接分叉两条线路。
* [x] 用户可以对同一 block 生成两个不同版本。
* [x] 用户可以比较两个版本并选择一个继续。
* [x] 图上可以清楚看到故事分叉。

---

## Phase 8: 一致性检查与小说工程功能

目标：从“写作工具”升级为“小说工程管理工具”。

### 数据库任务

* [ ] 创建 character_states 表，可选。
* [ ] 创建 foreshadowings 表，可选。
* [ ] 创建 timeline_events 表，可选。

### 后端任务

* [ ] 实现 check_consistency task。
* [ ] 实现角色状态记录。
* [ ] 实现伏笔记录。
* [ ] 实现时间线事件记录。
* [ ] 支持从 block 中提取事件，可选。
* [ ] 支持从 block 中提取角色状态，可选。
* [ ] 支持检查当前正文是否违反 canon。

### 前端任务

* [ ] 实现一致性检查按钮。
* [ ] 显示冲突列表。
* [ ] 支持跳转到相关 canon entity。
* [ ] 支持跳转到相关 block。
* [ ] 实现角色状态面板。
* [ ] 实现伏笔面板。
* [ ] 实现时间线视图，可选。

### 验收标准

* [ ] 用户可以对当前 block 执行一致性检查。
* [ ] 系统可以指出潜在设定冲突。
* [ ] 用户可以维护伏笔列表。
* [ ] 用户可以维护角色状态。

---

## Phase 9: 导出与备份

目标：让项目内容可以脱离系统使用。

### 后端任务

* [ ] 实现 Markdown 导出。
* [ ] 实现按 branch 导出。
* [ ] 实现按 chapter 导出。
* [ ] 实现项目 JSON 备份。
* [ ] 实现项目 JSON 导入。
* [ ] DOCX 导出可放后续。
* [ ] EPUB 导出可放后续。

### 前端任务

* [ ] 添加导出按钮。
* [ ] 支持选择导出 branch。
* [ ] 支持选择导出格式。
* [ ] 支持项目备份下载。
* [ ] 支持项目导入。

### 验收标准

* [ ] 用户可以导出某条 branch 的完整正文。
* [ ] 用户可以导出 Markdown。
* [ ] 用户可以备份整个项目。
* [ ] 用户可以从备份恢复项目。

---

## Phase 10: 桌面端与体验优化

目标：提升个人使用体验。

### 可选技术

* Tauri
* SQLite，本地轻量版可选
* PostgreSQL，本地 Docker 版可选

### 任务

* [ ] 封装 Tauri 桌面端。
* [ ] 支持本地配置文件。
* [ ] 支持本地 API key 管理。
* [ ] 支持自动启动后端服务。
* [ ] 支持本地项目目录。
* [ ] 优化大图性能。
* [ ] 优化大量 block 时的虚拟化渲染。
* [ ] 添加快捷键。
* [ ] 添加暗色主题。

---

## 16. MVP 最小闭环

第一版必须优先完成以下闭环：

```text
创建项目
  -> 创建 block
  -> 编辑正文
  -> 保存 revision
  -> fork block
  -> 调用 LLM 改写或续写
  -> 保存 LLM 输出为新 revision
  -> diff 两个 revision
  -> 选择一个 revision 继续写
```

只要这个闭环完成，BranchScribe 就已经区别于普通 Chatbox。

---

## 17. Codex 执行建议

Codex 应按以下方式推进：

1. 每次只实现一个 Phase 或一个明确模块。
2. 每次修改后运行前端 typecheck。
3. 每次修改后运行后端 test。
4. 数据库 migration 必须可重复执行。
5. 不要在前端硬编码 API key。
6. LLM Provider 必须通过接口抽象。
7. 所有写入操作必须返回创建后的实体。
8. 所有删除操作先做软删除可选，MVP 可以物理删除。
9. 所有 API 返回统一格式。
10. 所有 generation run 都必须记录输入上下文快照。

推荐 API 返回格式：

```json
{
  "data": {},
  "error": null
}
```

错误格式：

```json
{
  "data": null,
  "error": {
    "code": "BLOCK_NOT_FOUND",
    "message": "block not found"
  }
}
```

---

## 18. 代码质量要求

### 后端

* 使用 context.Context。
* 所有数据库操作必须处理错误。
* 所有外部 API 请求必须设置 timeout。
* LLM 流式输出必须支持取消。
* 不要在日志中打印 API key。
* 不要在 generation_runs 中保存明文 API key。
* Service 层不要直接依赖 HTTP request。
* Provider 层不要直接依赖数据库。

### 前端

* 所有 API 类型定义集中管理。
* Vue Flow 节点类型集中定义。
* Tiptap 编辑器封装为独立组件。
* LLM 操作面板封装为独立组件。
* Context Preview 封装为独立组件。
* 不要在组件中散落 API URL。
* 长文本渲染注意性能。
* 大量节点时避免不必要的全量重新渲染。

---

## 19. 后续增强方向

完成 MVP 后，可以继续做：

* 多模型评审。
* 自动章节规划。
* 自动大纲生成。
* 自动角色状态提取。
* 自动伏笔提取。
* 自动 timeline 构建。
* 多 branch 合并辅助。
* 角色口吻学习。
* 风格 profile。
* Prompt 模板市场。
* DOCX 导出。
* EPUB 导出。
* Tauri 桌面端。
* 云同步。
* 多人协作。

---

## 20. 当前优先级总结

最高优先级：

1. block + revision。
2. fork + branch。
3. graph canvas。
4. Tiptap editor。
5. LLM API stream。
6. diff viewer。
7. model profile。
8. context preview。

中优先级：

1. canon entities。
2. memory chunks。
3. pgvector search。
4. summary snapshots。
5. prompt templates。

低优先级：

1. 角色状态表。
2. 伏笔系统。
3. 时间线。
4. Tauri。
5. DOCX / EPUB。
6. 多人协作。
