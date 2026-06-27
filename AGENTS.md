# Project Rules

## Documents
开发文档: 
docs/ARCHITECTURE.md

开发日志
docs/DEVELOPMENT_LOG.md

---

## Workflow
根据开发文档中的任务进行开发，每完成一个任务要在工作清单中将完成的任务改为 [x]，然后撰写开发日志
对样式的小修小改等，不需要写入开发日志。
---

## Backend Startup

When running the backend:

1. Assume `.env` exists in the project root.
2. Environment variables are configured correctly.
3. If database connection fails, first check whether `.env` has been loaded instead of assuming the configuration is missing.