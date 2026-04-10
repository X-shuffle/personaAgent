# 🧠 PersonaAgent — Engineering Requirement Document

---

## 1. 📌 Project Overview

**PersonaAgent** is an AI agent system that simulates personalized conversational behavior using:

* Persona modeling (style & tone)
* Memory system (RAG)
* Emotion-aware responses
* Tool integration (external knowledge)

The system is implemented in **Go**, with a custom agent orchestration layer.

---

## 2. 🎯 Objectives

### Functional Goals

* Simulate a specific persona using user-provided data
* Support multi-turn conversations with memory
* Adapt tone based on user emotion
* Retrieve real-time information via tools

---

### Non-Functional Goals

* Modular architecture (extensible)
* Low latency (<2s per response target)
* Scalable session handling
* Clear separation of concerns

---

## 3. 🏗️ System Architecture

```text
User Input
   ↓
Intent Detection
   ↓
Knowledge Routing
   ├── Memory Retrieval
   ├── Tool Invocation
   └── Persona Injection
   ↓
Emotion Detection
   ↓
Prompt Builder
   ↓
LLM
   ↓
Response
   ↓
Memory Update
```

---

## 4. 🧩 Core Modules & Requirements

---

### 4.1 Agent Orchestrator

**Responsibilities:**

* Handle full request lifecycle
* Coordinate all modules

**Input:**

* user_input
* session_id

**Output:**

* response_text

---

### 4.2 Persona Module

**Responsibilities:**

* Store and load persona configuration
* Provide persona prompt context

**Data Structure:**

```json
{
  "tone": "warm",
  "style": "concise",
  "values": ["family", "patience"],
  "phrases": ["慢慢来", "别着急"]
}
```

---

### 4.3 Memory Module

**Responsibilities:**

* Store and retrieve memory
* Support semantic search

---

#### Memory Types

* episodic
* semantic
* summary

---

#### Memory Schema

```json
{
  "id": "string",
  "type": "episodic",
  "content": "text",
  "embedding": [],
  "emotion": "string",
  "timestamp": "int",
  "importance": 0.0
}
```

---

### 4.4 Data Ingestion Module

**Responsibilities:**

* Process user-uploaded historical data

---

#### Pipeline

```text
Raw → Parse → Clean → Structure → Segment → Embed → Store
```

---

#### Requirements

* Support TXT / JSON
* Remove low-value content
* Extract speaker & content
* Batch embedding

---

### 4.5 Emotion Module

**Responsibilities:**

* Detect user emotion per input

---

#### Output

```json
{
  "emotion": "sad",
  "intensity": 0.7
}
```

---

### 4.6 Tool Module

**Responsibilities:**

* Provide external capabilities

---

#### Tool Interface

```go
type Tool interface {
    Name() string
    Execute(input string) (string, error)
}
```

---

#### Required Tools

* search_current_events
* search_memory (optional)

---

### 4.7 Knowledge Router

**Responsibilities:**

* Decide data source for each query

---

#### Routing Logic

| Condition        | Action     |
| ---------------- | ---------- |
| Personal context | Use memory |
| Real-time query  | Use tool   |
| General chat     | Direct LLM |

---

### 4.8 Prompt Builder

**Responsibilities:**

* Combine persona, memory, emotion, tool results

---

#### Prompt Structure

```text
Persona:
...

Emotion:
...

Relevant Memory:
...

Tool Result:
...

User Input:
...
```

---

## 5. 🔄 Core Workflow

---

### Chat Flow

```text
1. Receive user input
2. Detect intent
3. Detect emotion
4. Route:
   - memory retrieval
   - tool call (if needed)
5. Build prompt
6. Call LLM
7. Return response
8. Store memory
```

---

## 6. 🌐 API Design

---

### POST /chat

#### Request

```json
{
  "session_id": "string",
  "message": "string"
}
```

---

#### Response

```json
{
  "response": "string"
}
```

---

### POST /ingest

#### Request

```json
{
  "data": "raw text"
}
```

---

#### Response

```json
{
  "status": "ok"
}
```

---

## 7. 🧱 Project Structure

```text
/internal
  /agent
  /persona
  /memory
  /emotion
  /tool
  /ingestion
  /llm
```

---

## 8. 🚀 Development Milestones

---

### Phase 1 — MVP

* Basic chat API
* Persona injection
* LLM integration

---

### Phase 2 — Memory

* Vector DB integration
* Memory retrieval
* Memory storage

---

### Phase 3 — Emotion

* Emotion detection
* Prompt adaptation

---

### Phase 4 — Tool System

* Tool interface
* Tool routing
* External API integration

---

### Phase 5 — Advanced

* Memory summarization
* Importance scoring
* Persona consistency

---

## 9. 🧪 Testing Strategy

* Unit tests for each module
* Mock LLM responses
* Test memory retrieval accuracy
* Tool invocation tests

---

## 10. 📌 Constraints

* No fine-tuning required
* Must use user-provided data only
* Avoid storing unnecessary chat logs
* Ensure modular design

---

## 11. ✅ Definition of Done

* Chat API works end-to-end
* Persona behavior is consistent
* Memory retrieval improves responses
* Tool integration works
* Code is modular and maintainable

---

## 12. 🔥 Key Engineering Highlights

* Custom agent orchestration (no heavy framework)
* Structured memory system (not naive RAG)
* Emotion-aware response generation
* Tool-based external knowledge integration

---
