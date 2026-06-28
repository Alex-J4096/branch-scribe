# BranchScribe

BranchScribe is an LLM API based nonlinear fiction writing tool.

## Local Database

```bash
docker compose up -d postgres
```

The PostgreSQL container is named `branchscribe-postgres` and uses pgvector.

## Backend

```bash
cd backend
go run ./cmd/server
```

The health endpoint is available at:

```text
GET http://localhost:8080/health
```

## Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend expects the backend API at `http://localhost:8080/api` by default.

## LLM Debug CLI

Start the standalone listener in one terminal:

```bash
cd backend
go run ./cmd/llm-debug
```

Then opt the backend into debug reporting:

```bash
cd backend
LLM_DEBUG_URL=http://127.0.0.1:6069 go run ./cmd/server
```

The listener prints the final `messages` sent to the provider and the corresponding
streamed reasoning/content events. Open `http://127.0.0.1:6069` for the readable
Web UI, which groups messages, reasoning, and content by request and updates them
live. It never receives the API key. Use `-addr` to change its default
`127.0.0.1:6069` listen address. If the listener is stopped, generation continues
normally and debug events are silently discarded.
