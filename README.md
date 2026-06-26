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
