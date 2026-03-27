# Authentication & Stateful Session Flow

This document describes the stateful JWT architecture implemented in the `go-template` project. It ensures security through short-lived access tokens and longer-lived, revocable refresh tokens.

## 1. Login Flow

```mermaid
sequenceDiagram
    participant Client
    participant Controller
    participant Service
    participant Database
    participant Redis

    Client->>Controller: POST /login (email, password)
    Controller->>Service: Login(ctx, email, password)
    Service->>Database: GetUserByEmail(email)
    Database-->>Service: User Record (Hashed Pwd)
    Service->>Service: Bcrypt Compare (Password, Hash)
    
    rect rgb(200, 230, 255)
    Note over Service,Redis: Stateful Token Generation
    Service->>Service: Generate AccessToken (jti_acc, 1h)
    Service->>Redis: SetSession(userID, jti_acc, "valid")
    Service->>Service: Generate RefreshToken (jti_ref, 7d)
    Service->>Redis: SetSession(userID, jti_ref, "valid")
    end

    Service-->>Controller: AuthResponse {Access, Refresh}
    Controller-->>Client: 200 OK (Tokens)
```

## 2. Refresh Token Flow

This flow allows the client to get a new **Access Token** without asking for credentials again.

*   **Validation**: The server checks the signature of the Refresh Token AND validates its `JTI` against Redis.
*   **Rotation**: The existing Refresh Token remains valid (or can be rotated), but a new Access Token is always issued with a fresh `jti`.

## 3. Evict User Flow

Used for security incidents (revoking access immediately).

```mermaid
sequenceDiagram
    participant Admin
    participant Controller
    participant Service
    participant Redis

    Admin->>Controller: POST /evict/:id (Auth required)
    Controller->>Service: EvictUser(ctx, userID)
    Service->>Redis: Scan session:userID:*
    Service->>Redis: DEL all matching keys
    Service-->>Controller: Success
    Controller-->>Admin: 200 OK
    
    Note over Client,Redis: Any subsequent request by evicted user fails at AuthMiddleware
```

## 4. Middleware Enforcement

On **every** protected request:
1.  **Parse**: JWT signature is verified.
2.  **Expiration**: Standard JWT expiration check.
3.  **State Check**: The middleware calls `redis.IsSessionValid(userID, jti)`. 
4.  **Action**: If the key is missing in Redis (deleted via `/logout` or `/evict`), the request is rejected with `401 Unauthorized`.
