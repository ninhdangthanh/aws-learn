# AI Insight API from ETL Data — Simple Learning Project

## Goal

Build a simple AI backend service that:

```text
ETL data
 → send business metrics to LLM
 → return AI-generated insights
```

Focus:

* understand LLM integration
* prompt engineering
* streaming response
* structured AI output
* simple AI backend architecture

NOT focus on:

* training model
* ML theory
* complex infra

---

# Tech Stack

| Component | Tech       |
| --------- | ---------- |
| Language  | Golang     |
| HTTP API  | Gin        |
| Database  | PostgreSQL |
| Cache     | Redis      |
| LLM       | OpenAI API |
| Container | Docker     |

---

# Simple Architecture

```text
PostgreSQL
    ↓
Metrics Service
    ↓
AI Insight Service
    ↓
OpenAI API
    ↓
Frontend / Postman
```

---

# Phase 1 — Prepare Fake ETL Data

## Goal

Create simple retail/F&B metrics table.

## Example tables

### daily_sales

```sql
date
store_id
revenue
orders
refunds
```

### product_sales

```sql
product_name
quantity
revenue
```

Insert fake data manually.

---

# Phase 2 — Metrics API

## Goal

Expose business metrics via backend.

## Endpoint

```http
GET /metrics/weekly
```

## Response

```json
{
  "revenue": 120000,
  "orders": 3200,
  "refund_rate": 3.2,
  "top_products": [
    "Coffee",
    "Milk Tea"
  ]
}
```

---

# Phase 3 — AI Insight API

## Goal

Send metrics to LLM and get natural language insight.

## Endpoint

```http
POST /insights
```

---

# Flow

```text
Client request
 → query metrics
 → build prompt
 → call OpenAI API
 → return insight
```

---

# Example Prompt

```text
You are a retail business analyst.

Analyze the following business metrics:

Revenue: $120000
Orders: 3200
Refund rate: 3.2%
Top products:
- Coffee
- Milk Tea

Provide:
1. Business summary
2. Potential issue
3. Recommendation
```

---

# Example Response

```json
{
  "summary": "Revenue is stable this week...",
  "issue": "Refund rate increased slightly...",
  "recommendation": "Review delivery process..."
}
```

---

# Phase 4 — Streaming Response

## Goal

Learn AI streaming.

## Implement

Server-Sent Events (SSE).

```text
LLM streaming tokens
 → backend stream
 → frontend receive real-time text
```

Learn:

* chunked response
* streaming APIs
* AI UX basics

---

# Phase 5 — Redis Cache

## Goal

Avoid duplicate LLM requests.

## Flow

```text
same metrics request
 → check Redis
 → return cached insight
```

Simple cache key:

```text
weekly-insight-store-1
```

Learn:

* AI cost optimization
* response caching

---

# Phase 6 — Structured Output

## Goal

Learn structured AI response.

Ask LLM to return JSON only.

## Example

```json
{
  "summary": "...",
  "risk": "...",
  "recommendation": "..."
}
```

Learn:

* prompt engineering
* JSON parsing
* AI reliability

---

# Suggested Folder Structure

```text
/project
  /cmd
  /internal
    /api
    /service
    /repository
    /llm
    /cache
  /docker
```

---

# Concepts You Will Learn

## Backend

* API design
* SSE streaming
* Redis caching
* Docker
* service architecture

## AI

* prompt engineering
* LLM orchestration
* structured output
* hallucination basics
* token usage
* AI latency

---

# What To Put On CV

```text
Built AI-powered insight API in Go:
- analyzed ETL business metrics using LLM APIs
- implemented streaming AI responses with SSE
- added Redis caching for AI cost optimization
- generated structured business insights from retail data
```

---

# Important Mindset

This project is NOT:

```text
“becoming ML scientist”
```

It IS:

```text
“learning how backend systems integrate AI capabilities”
```

Và đó là hướng phù hợp nhất cho backend engineer muốn transition sang AI product/backend hiện tại.
