# Gateway Lab Notes API

`gateway-lab-notes-api` is a small AWS learning project focused on Amazon API Gateway, authentication, authorization, API keys, and rate limiting.

The app is intentionally simple: users can log in and manage private notes. The main learning value is not the notes feature itself, but how requests pass through API Gateway before reaching the backend.

## Goal

Build a minimal protected API where API Gateway acts as the public entry point:

```text
Client
  -> API Gateway
     -> JWT/API key checks
     -> throttling/rate limit
     -> request validation
     -> logs and metrics
  -> Backend service
  -> Database
```

## Core Features

- Public health endpoint.
- Public login endpoint that returns a JWT.
- Protected notes endpoints that require JWT auth.
- Admin-only endpoint that requires `role=admin`.
- API key requirement for selected routes.
- Usage plan with throttling and quota.
- CloudWatch access logs and execution logs.
- Simple backend that can run on Lambda or an EC2/ECS HTTP service.

## Suggested MVP Routes

```text
GET    /health
POST   /login
GET    /notes
POST   /notes
GET    /notes/{id}
DELETE /notes/{id}
GET    /admin/usage
```

## Auth Model

`POST /login` returns a JWT containing basic user claims:

```json
{
  "sub": "user_123",
  "email": "user@example.com",
  "role": "user"
}
```

Protected requests use:

```text
Authorization: Bearer <jwt>
```

Routes that should be rate-limited by client application also require:

```text
x-api-key: <api-key>
```

## What This Project Teaches

- Difference between authentication and authorization.
- How API Gateway can behave like a middleware layer before the backend.
- Lambda Authorizer or Cognito Authorizer basics.
- API key and usage plan setup.
- Throttling, burst limit, and quota.
- Request validation before backend execution.
- Passing user context from API Gateway to backend.
- Debugging 401, 403, 429, and 5xx errors.

## Recommended Stack

For the smallest learning path:

```text
API Gateway REST API
Lambda Authorizer
Go backend on Lambda or EC2
DynamoDB or PostgreSQL
CloudWatch Logs
```

If you want to stay close to backend projects you already built, use Go Gin on EC2 first, then move the backend to Lambda later.

## Success Criteria

The project is done when you can demonstrate:

- Calling `/health` without auth works.
- Calling `/notes` without JWT returns `401` or `403`.
- Calling `/notes` with a valid JWT works.
- Calling `/admin/usage` with a normal user is blocked.
- Calling `/admin/usage` with an admin JWT works.
- Sending too many requests returns `429 Too Many Requests`.
- CloudWatch shows request logs and status codes.
