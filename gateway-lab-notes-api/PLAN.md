# Project Plan

This plan keeps the project small so the learning stays focused on API Gateway instead of application complexity.

## Phase 1 - Local Backend

Build a simple Notes API locally before touching AWS.

### Tasks

- Create a small Go HTTP API.
- Add `/health`.
- Add `/login` with hardcoded demo users.
- Return signed JWTs from `/login`.
- Add JWT middleware locally so the backend behavior is easy to test.
- Add notes CRUD with in-memory storage or SQLite/PostgreSQL.
- Add role check for `/admin/usage`.

### Demo Users

```text
user@example.com  -> role=user
admin@example.com -> role=admin
```

### Done When

- Local `curl /health` works.
- Local login returns JWT.
- Local protected routes reject missing/invalid JWT.
- Admin route blocks normal user.

## Phase 2 - Put API Gateway In Front

Deploy the backend and expose it through API Gateway.

### Option A: Backend On Lambda

```text
API Gateway REST API -> Lambda backend
```

This is the cleanest API Gateway learning path.

### Option B: Backend On EC2

```text
API Gateway REST API -> EC2 public HTTP endpoint
```

This is closer to traditional backend deployment, but requires the EC2 service to be reachable by API Gateway.

### Tasks

- Create API Gateway REST API.
- Create resources and methods for MVP routes.
- Connect methods to backend integration.
- Deploy a `dev` stage.
- Test with the API Gateway invoke URL.

### Done When

- Public API Gateway URL can call `/health`.
- API Gateway can forward requests to the backend.

## Phase 3 - Add Authentication

Move authentication checks to the gateway layer.

### Recommended First Version

Use a Lambda Authorizer that:

- Reads `Authorization: Bearer <jwt>`.
- Validates JWT signature.
- Extracts `sub`, `email`, and `role`.
- Allows or denies the request.
- Passes user context to backend.

### Tasks

- Create Lambda Authorizer.
- Attach authorizer to protected routes.
- Leave `/health` and `/login` public.
- Update backend to read user context from forwarded headers or authorizer context.

### Done When

- Missing token is blocked before backend execution.
- Invalid token is blocked before backend execution.
- Valid token reaches backend.

## Phase 4 - Add Authorization

Protect admin-only route.

### Recommended Approach

Keep coarse authorization in the authorizer and detailed authorization in the backend.

Example:

```text
Lambda Authorizer:
  - validates token
  - passes role=admin/user

Backend:
  - checks role for /admin/usage
```

### Done When

- Normal user cannot call `/admin/usage`.
- Admin user can call `/admin/usage`.

## Phase 5 - Add API Keys And Rate Limit

Use API Gateway usage plans to learn throttling and quota.

### Tasks

- Create API key for a free client.
- Create API key for a pro client.
- Create usage plans.
- Attach API keys to usage plans.
- Require API key on `/notes` routes.
- Test request burst behavior.

### Example Limits

```text
Free plan:
  Rate: 2 req/sec
  Burst: 5
  Quota: 100 requests/day

Pro plan:
  Rate: 20 req/sec
  Burst: 50
  Quota: 10000 requests/day
```

### Done When

- Missing `x-api-key` is rejected.
- Valid API key works.
- Too many requests return `429`.

## Phase 6 - Observability

Make the gateway debuggable.

### Tasks

- Enable CloudWatch access logs.
- Log request id, route, status, latency, and caller.
- Add backend logs with request id.
- Create simple CloudWatch metric checks for `4xx`, `5xx`, and latency.

### Done When

- You can identify why a request failed: auth, rate limit, backend error, or validation error.

## Phase 7 - Optional Extensions

Only add these after the core API Gateway lessons are complete.

- Add request validation with JSON schemas.
- Add custom domain like `api.example.com`.
- Add WAF basic rules.
- Add Cognito Authorizer and compare with Lambda Authorizer.
- Add WebSocket API for note-change notifications.
- Add canary deployment for a `v2` route.
