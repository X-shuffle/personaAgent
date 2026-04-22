# PersonaAgent

> A Go-based Persona AI Agent with memory, emotion awareness, and MCP tool integration.
> 一个基于 Go 的人设智能体，支持记忆、情绪感知与 MCP 工具调用。

## ✨ Features | 特性

- Persona-driven responses (`tone/style/values/phrases`)
- Memory retrieval and storage (Qdrant + Embedding)
- Emotion detection (`rule` / `llm`)
- Tool calling via MCP runtime
- Data ingestion via `/ingest` (TXT/JSON, JSON body or multipart upload)

## 🏗️ Architecture | 架构

Core flow | 核心链路：

1. Receive user input
2. Retrieve memory + detect emotion
3. Build prompt (Persona + Memory + Emotion)
4. Call LLM (with tool-calling loop)
5. Return response + async store memory

Key folders | 主要目录：

- `cmd/server` — server entry
- `internal/agent` — orchestrator (chat lifecycle)
- `internal/prompt` — prompt builder
- `internal/memory` — memory service / vector store / embedder
- `internal/ingestion` — ingestion pipeline
- `internal/emotion` — emotion detector
- `internal/mcp/runtime` — MCP runtime manager
- `internal/api` — HTTP handlers

## 🚀 Quick Start | 快速开始

### 1) Install dependencies

```bash
go mod tidy
```

### 2) Configure environment

```bash
cp .env.example .env
```

Minimum required (recommended):

- `LLM_MODE=mock` (local debug) or `LLM_MODE=http` (real upstream)
- `MEMORY_EMBED_ENDPOINT`
- `MEMORY_EMBED_API_KEY` (if required)
- `QDRANT_URL`
- `QDRANT_COLLECTION`

### 3) Run server

```bash
go run ./cmd/server
```

Default address: `http://localhost:8080`

## 🔌 API

### `POST /chat`

Request:

```json
{
  "session_id": "user-1",
  "message": "我最近压力有点大"
}
```

Response:

```json
{
  "response": "..."
}
```

cURL:

```bash
curl -X POST http://localhost:8080/chat \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"user-1","message":"你好"}'
```

### `POST /ingest`

Supports JSON body and multipart upload.

JSON request:

```json
{
  "session_id": "user-1",
  "source": "wechat",
  "format": "auto",
  "data": "2026-04-01 10:00:00 张三: 你好",
  "dry_run": false
}
```

Response:

```json
{
  "status": "ok",
  "session_id": "user-1",
  "source": "wechat",
  "accepted": 10,
  "rejected": 2,
  "segments": 4,
  "stored": 4,
  "dry_run": false,
  "warnings": []
}
```

Multipart cURL:

```bash
curl -X POST http://localhost:8080/ingest \
  -F 'session_id=user-1' \
  -F 'source=wechat' \
  -F 'format=auto' \
  -F 'dry_run=false' \
  -F 'file=@./chat.txt'
```

## ⚙️ Configuration

See `.env.example` for full config. Core groups:

- LLM: `LLM_MODE`, `LLM_ENDPOINT`, `LLM_API_KEY`, `LLM_MODEL`, `LLM_PROVIDER`
- Persona: `PERSONA_TONE`, `PERSONA_STYLE`, `PERSONA_VALUES`, `PERSONA_PHRASES`
- Emotion: `EMOTION_DETECTOR_MODE`, `EMOTION_DETECT_TIMEOUT_SECONDS`
- Memory: `MEMORY_TOP_K`, `MEMORY_SIMILARITY_THRESHOLD`, `MEMORY_SHORT_TERM_SIZE`
- Embedding: `MEMORY_EMBED_ENDPOINT`, `MEMORY_EMBED_MODEL`, `MEMORY_VECTOR_DIM`
- Qdrant: `QDRANT_URL`, `QDRANT_COLLECTION`, `QDRANT_API_KEY`
- Ingestion: `INGEST_ENABLED`, `INGEST_ALLOWED_EXTENSIONS`
- MCP: `MCP_SETTINGS_PATH`, `TOOL_MAX_EXEC_ROUNDS`

## 🖥️ Desktop (Wails + React)

Desktop launcher code lives in `apps/desktop`.

### Run desktop in development

1. Start backend API first (desktop calls `/chat` on `http://localhost:8080` by default):

```bash
go run ./cmd/server
```

1. In another terminal, start desktop app:

```bash
cd apps/desktop
wails dev
```

1. Frontend-only debug (optional):

```bash
cd apps/desktop/frontend
npm install
npm run dev
```

Useful env vars:

- `DESKTOP_CHAT_BASE_URL` (default `http://localhost:8080`)
- `DESKTOP_HISTORY_DB_PATH` (override local history SQLite path)

### Build desktop

```bash
cd apps/desktop
wails build
```

```bash
cd /Users/xshuffle/my_work_project/personaAgent/apps/desktop
HTTPS_PROXY=http://127.0.0.1:10808 HTTP_PROXY=http://127.0.0.1:10808 ALL_PROXY=socks5://127.0.0.1:10808 GOPROXY=https://proxy.golang.org,direct DESKTOP_CHAT_BASE_URL=http://localhost:8080 go run github.com/wailsapp/wails/v2/cmd/wails@latest dev
```

## 🧪 Testing

```bash
go test ./...
```

## 🗺️ Roadmap

- [x] Phase 1: Basic chat API + persona injection + LLM integration
- [x] Phase 2: Memory retrieval/storage with vector DB
- [x] Phase 3: Emotion detection + prompt adaptation
- [x] Phase 4: MCP tool interface + routing/runtime
- [x] Phase 5: Summary memory + importance scoring + persona consistency
- [ ] Next: semantic memory path in retrieval pipeline (currently episodic + summary are primary)

## 🤝 Contributing

PRs and issues are welcome.

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, guidelines, and PR checklist.
Please also review [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

Quick flow:

1. Fork and create a feature branch
2. Keep changes focused and test-covered
3. Run `go test ./...`
4. Submit PR with context and test notes

## 📚 Docs

- Requirements: [demand.md](demand.md)
- Change docs: [docs/changes/](docs/changes/)
- Plans: [docs/plan.md](docs/plan.md)

## 🔒 Security Notes

- Do **not** commit real API keys in `.env` or `mcp_settings.json`.
- `mcp_settings.json` may contain third-party credentials; treat it as sensitive.

## 📄 License

This project is licensed under the [MIT License](LICENSE).
