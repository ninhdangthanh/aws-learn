CREATE TABLE IF NOT EXISTS idempotency_keys (
    id BIGSERIAL PRIMARY KEY,

    -- Scope rất quan trọng trong multi-tenant/user system.
    -- Cùng một raw key "abc" của user A không nên đụng user B.
    scope TEXT NOT NULL,
    key TEXT NOT NULL,

    -- Hash của phần payload quan trọng.
    -- Nếu cùng key nhưng payload khác, backend phải reject 409 Conflict.
    request_hash TEXT NOT NULL,

    -- processing: request đầu tiên đã nhận quyền xử lý side effect.
    -- succeeded: side effect xong, retry có thể trả lại response cũ.
    -- failed: lỗi deterministic có thể được cache lại tùy policy.
    status TEXT NOT NULL CHECK (status IN ('processing', 'succeeded', 'failed')),

    response_status INT,
    response_body JSONB,
    resource_type TEXT,
    resource_id TEXT,

    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,

    UNIQUE (scope, key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at
    ON idempotency_keys (expires_at);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    sku TEXT NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL DEFAULT 'created',

    -- Đây là lớp bảo vệ cuối cho side effect.
    -- Nếu code bị bug và cố insert order lần hai cùng operation,
    -- DB vẫn không cho duplicate side effect lọt qua.
    idempotency_scope TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (idempotency_scope, idempotency_key)
);

CREATE TABLE IF NOT EXISTS processed_events (
    event_id TEXT NOT NULL,
    consumer_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('processing', 'succeeded', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Một event có thể được nhiều consumer khác nhau xử lý độc lập.
    -- Vì vậy dedup key nên là (consumer_name, event_id), không chỉ event_id.
    PRIMARY KEY (consumer_name, event_id)
);

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    operation TEXT NOT NULL CHECK (operation IN ('add_payment', 'pay_off', 'pass_order_refund')),
    order_id TEXT NOT NULL,
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    idempotency_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Redis là fast-path cache có TTL, không phải nguồn sự thật bền vững:
    -- restart/eviction có thể xoá key trước khi client kịp retry.
    -- Với tiền, constraint này ở Postgres mới là lưới an toàn cuối chặn
    -- double-charge/double-refund, kể cả khi cache Redis đã mất.
    UNIQUE (operation, idempotency_key)
);

CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL,
    event_id TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- External side effect thật như email/SMS không rollback được.
    -- Ở lab này notification là DB row, nên ta vẫn thêm unique constraint
    -- để minh họa "side effect cũng phải tự idempotent".
    UNIQUE (event_id)
);

