# Multipart S3 Upload — Vite/React + Go

Implementation of the design in [`idea.md`](./idea.md): the browser uploads a large
file **directly to S3** using presigned part URLs. The Go backend only orchestrates
(init → presign → complete/abort) and never touches the file bytes.

```
┌──────────┐  1. init / presign / complete   ┌──────────┐
│ Browser  │ ──────────────────────────────► │ Go API   │ ──► S3 (control plane)
│ (React)  │                                  └──────────┘
│          │  2. PUT each part (presigned)    ┌──────────┐
│          │ ───────────────────────────────► │   S3     │   (data plane)
└──────────┘                                  └──────────┘
```

## Layout

```
backend/    Gin + aws-sdk-go-v2. Endpoints: /uploads/{init,presign-parts,complete,abort}
frontend/   Vite + React + TS. Slices the file, uploads parts in parallel with retry.
```

## Endpoints

| Method / Path            | Purpose                                                        |
| ------------------------ | ------------------------------------------------------------- |
| `POST /uploads/init`     | Generate object key + `CreateMultipartUpload` → `uploadId`    |
| `POST /uploads/presign-parts` | Presigned `UploadPart` URL for each requested part number |
| `POST /uploads/complete` | `CompleteMultipartUpload` with the collected `{partNumber,etag}` |
| `POST /uploads/abort`    | `AbortMultipartUpload` to drop orphaned parts                 |

## Running

### 1. Backend

```bash
cd backend
cp .env.example .env      # set S3_BUCKET, AWS_REGION; provide AWS credentials
go mod tidy
go run .                  # listens on :8080
```

Credentials come from the standard AWS chain (env vars, `~/.aws/credentials`, SSO,
or an IAM role) — never hardcode them.

### 2. Frontend

```bash
cd frontend
cp .env.example .env      # VITE_API_BASE=http://localhost:8080
npm install
npm run dev               # http://localhost:5173
```

## Required AWS setup

### S3 bucket CORS (mandatory)

Parts are `PUT` from the browser to S3 (cross-origin), and the client reads the
`ETag` **response header** — so the bucket must both allow `PUT` and expose `ETag`:

```json
[
  {
    "AllowedOrigins": ["http://localhost:5173"],
    "AllowedMethods": ["PUT"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3000
  }
]
```

Without `ExposeHeaders: ["ETag"]` the upload of each part succeeds but the client
cannot read the ETag, and `complete` fails.

### IAM policy for the backend

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:AbortMultipartUpload",
        "s3:ListMultipartUploadParts"
      ],
      "Resource": "arn:aws:s3:::YOUR_BUCKET/*"
    }
  ]
}
```

### Lifecycle rule — clean up incomplete uploads

Aborted/abandoned multipart uploads still cost storage. Add a lifecycle rule so S3
auto-deletes incomplete uploads:

```json
{
  "Rules": [
    {
      "ID": "abort-incomplete-mpu",
      "Status": "Enabled",
      "Filter": { "Prefix": "uploads/" },
      "AbortIncompleteMultipartUpload": { "DaysAfterInitiation": 3 }
    }
  ]
}
```

## Key constraints honored (from `idea.md`)

- Max **10,000 parts** per upload (validated in `presign-parts`).
- Each part **≥ 5 MiB** except the last — default `PART_SIZE_BYTES` is 10 MiB.
- Client uploads with **concurrency 4** and **retries each part up to 3×** with
  exponential backoff; a failed part is retried on its own, not the whole file.
- On fatal error or user abort, the client calls `/uploads/abort` (best effort).

## Retry & presigned-URL expiry

- The client presigns **all parts once** up front (one batched call), then uploads.
- Presigned URLs are short-lived (`PRESIGN_EXPIRY_SECONDS`, default **15 min**). On
  a large/slow upload the pre-fetched URL for a late part — or for a part being
  retried — can already be **expired**, which S3 rejects with
  `403 SignatureDoesNotMatch` / `AccessDenied`.
- To handle this, **every retry re-presigns that single part** (`resolveUrl(part,
  forceRefresh=true)` in `multipartUpload.ts`) before the `PUT`. So a failed part is
  retried on its own with a *fresh* URL — the whole file is never re-uploaded.
- Trade-off: raising `PRESIGN_EXPIRY_SECONDS` reduces re-presign calls but leaves
  valid signatures alive longer; the per-retry refresh is the robust default.

## Notes & caveats

> **Not tested against real S3.** This code compiles cleanly (`go build ./...` and
> `npm run build` both pass), but it has **not** been run end-to-end against a live
> bucket — that needs real AWS credentials + a bucket with the CORS/IAM/lifecycle
> config above, which isn't available in this environment. Before trusting it,
> smoke-test with a >5 MiB file (so it spans multiple parts) and watch the network
> tab: each part `PUT` should return `200` with an `ETag`, and `complete` should
> return the object location.

Other things to know:

- **CORS is the #1 gotcha.** If parts upload fine but `complete` fails with a
  missing-ETag error, the bucket is missing `ExposeHeaders: ["ETag"]`. The browser
  can send the `PUT` without CORS, but it can't *read the response header* without it.
- **Presigned `UploadPart` must not add headers the signature didn't cover.** The
  client sends the raw blob with no extra headers (no `Content-Type`, no ACL) so the
  signature matches. Adding headers the backend didn't presign → `SignatureDoesNotMatch`.
- **Bucket region must match `AWS_REGION`.** A region mismatch surfaces as
  `AuthorizationHeaderMalformed` or a 301 redirect on the part `PUT`.
- **No auth on the endpoints.** This demo omits authn/authz — in production, protect
  `/uploads/*` and verify the caller may write the generated key (see `idea.md`
  "Lưu ý vận hành": the backend owns permissions, key, uploadId, and state).
- **Empty files** are handled: a single zero-byte part is uploaded so `complete`
  has something to assemble.
- **Backend never sees file bytes** — only control-plane calls — so it won't time
  out or burn bandwidth on multi-GB uploads.
