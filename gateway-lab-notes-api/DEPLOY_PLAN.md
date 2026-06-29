# Deploy Plan

This deployment plan focuses on AWS API Gateway learning. Start with the simplest path, then add production-like pieces gradually.

## 1. Target Architecture

Recommended first version:

```text
Client / curl / Postman
  -> API Gateway REST API
  -> Lambda Authorizer
  -> Backend service
  -> Database
  -> CloudWatch Logs
```

Backend options:

```text
Option A:
  API Gateway -> Lambda backend -> DynamoDB

Option B:
  API Gateway -> EC2 Go service -> PostgreSQL
```

For learning API Gateway deeply, choose Option A first. For practicing traditional backend deployment, choose Option B.

## 2. AWS Resources

Use this namespace:

```text
Project = gateway-lab-notes-api
Env     = dev
Region  = ap-southeast-1
```

Suggested resources:

```text
API Gateway REST API: gateway-lab-notes-api-dev
Stage:                dev
Lambda authorizer:    gateway-lab-notes-authorizer-dev
Backend Lambda:       gateway-lab-notes-backend-dev
DynamoDB table:       gateway-lab-notes-dev
CloudWatch log group: /aws/apigateway/gateway-lab-notes-api-dev
```

## 3. Build Backend

Backend should expose:

```text
GET    /health
POST   /login
GET    /notes
POST   /notes
GET    /notes/{id}
DELETE /notes/{id}
GET    /admin/usage
```

Environment variables:

```text
APP_ENV=dev
JWT_SECRET=<demo-secret>
NOTES_TABLE=gateway-lab-notes-dev
```

For local testing:

```bash
curl http://localhost:8080/health
```

## 4. Create API Gateway REST API

Create a REST API with these resources:

```text
/health
/login
/notes
/notes/{id}
/admin/usage
```

Methods:

```text
GET    /health       public
POST   /login        public
GET    /notes        protected
POST   /notes        protected + API key required
GET    /notes/{id}   protected
DELETE /notes/{id}   protected + API key required
GET    /admin/usage  protected
```

Deploy to stage:

```text
dev
```

## 5. Create Lambda Authorizer

The authorizer should:

- Read `Authorization` header.
- Require `Bearer <jwt>`.
- Validate JWT signature.
- Extract user claims.
- Return allow/deny policy.
- Pass context values to backend:

```text
user_id
email
role
```

Routes using authorizer:

```text
/notes
/notes/{id}
/admin/usage
```

Routes without authorizer:

```text
/health
/login
```

## 6. Configure API Keys And Usage Plans

Create API keys:

```text
gateway-lab-free-client
gateway-lab-pro-client
```

Create usage plans:

```text
Free:
  Rate: 2 req/sec
  Burst: 5
  Quota: 100 requests/day

Pro:
  Rate: 20 req/sec
  Burst: 50
  Quota: 10000 requests/day
```

Attach API keys to usage plans.

Enable `API Key Required` on write routes first:

```text
POST   /notes
DELETE /notes/{id}
```

Later, try requiring API keys for all `/notes` routes.

## 7. Enable Logs

Enable CloudWatch logs for the `dev` stage.

Log fields to include:

```text
requestId
ip
caller
user
requestTime
httpMethod
resourcePath
status
protocol
responseLength
integrationLatency
```

This helps debug:

```text
401/403 -> auth/authz problem
429     -> rate limit or quota
502     -> backend integration problem
504     -> backend timeout
```

## 8. Test Commands

Health:

```bash
curl https://<api-id>.execute-api.ap-southeast-1.amazonaws.com/dev/health
```

Login:

```bash
curl -X POST https://<api-id>.execute-api.ap-southeast-1.amazonaws.com/dev/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password"}'
```

Protected route:

```bash
curl https://<api-id>.execute-api.ap-southeast-1.amazonaws.com/dev/notes \
  -H "Authorization: Bearer <jwt>"
```

API key route:

```bash
curl -X POST https://<api-id>.execute-api.ap-southeast-1.amazonaws.com/dev/notes \
  -H "Authorization: Bearer <jwt>" \
  -H "x-api-key: <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"title":"API Gateway note","body":"Learning throttling and authorizers"}'
```

Rate limit test:

```bash
for i in $(seq 1 20); do
  curl -s -o /dev/null -w "%{http_code}\n" \
    https://<api-id>.execute-api.ap-southeast-1.amazonaws.com/dev/notes \
    -H "Authorization: Bearer <jwt>" \
    -H "x-api-key: <api-key>"
done
```

Expected result: some requests should return `429` after the limit is exceeded.

## 9. Debug Checklist

If request returns `401`:

- Missing `Authorization` header.
- Invalid JWT format.
- Expired token.

If request returns `403`:

- Authorizer denied request.
- Missing API key on a route that requires it.
- User role is not allowed.

If request returns `429`:

- Usage plan throttle exceeded.
- Daily/monthly quota exceeded.

If request returns `502`:

- Backend integration response is malformed.
- Lambda crashed.
- EC2 backend returned an unexpected response.

If request returns `504`:

- Backend timed out.
- Integration timeout was reached.

## 10. Cleanup

To avoid ongoing costs, delete resources when done:

- API Gateway REST API.
- Lambda functions.
- DynamoDB table.
- CloudWatch log groups if no longer needed.
- EC2/RDS resources if using Option B.
