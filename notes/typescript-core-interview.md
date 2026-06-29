# TypeScript Core Interview Notes
## Theo CV Bun/Node.js TypeScript, React TypeScript, backend API và data tooling

File này tập trung vào phần TypeScript/JavaScript runtime dễ bị hỏi khi CV có Bun/Node.js, React TypeScript, JavaScript crawler/tooling. Bỏ qua kiến thức cơ bản như biến, function, enum.

---

## 1. TypeScript trong CV backend/fullstack

Với CV này, TypeScript nên được trình bày như công cụ tăng độ an toàn khi xây API, service client, dashboard, crawler/data tooling, chứ không chỉ là "JavaScript có type".

Câu trả lời tốt:

> TypeScript giúp tôi mô hình hóa request/response, DTO, domain type, service contract và frontend props/state rõ ràng hơn. Ở backend Node.js, TS giảm lỗi runtime khi refactor API hoặc data pipeline. Nhưng TS chỉ bảo vệ ở compile-time; dữ liệu từ HTTP, DB, queue vẫn phải validate runtime.

---

## 2. JavaScript runtime, V8 và Node.js

Node.js là JavaScript runtime dựa trên V8, cung cấp thêm API ngoài browser như filesystem, network, process, crypto và module system.

Điểm cần nắm:

* JavaScript chạy trên một main thread cho execution stack.
* I/O bất đồng bộ được xử lý qua libuv, OS async API hoặc thread pool.
* V8 JIT compile JavaScript thành machine code tối ưu dần theo runtime profile.
* Node hợp với I/O-bound service, API gateway, BFF, crawler, tooling.
* CPU-bound task nặng có thể block event loop nếu chạy trực tiếp trên main thread.

---

## 3. Bun runtime và tooling

Bun là all-in-one toolkit cho JavaScript/TypeScript: runtime, package manager, script runner, test runner và bundler. Điểm khác Node.js quan trọng nhất là Bun dùng JavaScriptCore thay vì V8, và có transpiler native để chạy `.ts`, `.tsx`, JSX trực tiếp mà không cần `ts-node`/`tsx` trong nhiều workflow.

Khi CV ghi hay dùng Bun, nên nói theo hướng thực tế:

> Tôi dùng Bun chủ yếu để tăng tốc development workflow: chạy TypeScript trực tiếp, install dependency nhanh, chạy script/test nhanh, và build tool nhỏ gọn. Với production backend, tôi vẫn kiểm tra compatibility của dependency Node.js, observability, Docker image, CI/CD và behavior của runtime trước khi thay Node.js hoàn toàn.

### Bun so với Node.js

| Tiêu chí | Bun | Node.js |
|---|---|---|
| JS engine | JavaScriptCore | V8 |
| TypeScript | Chạy `.ts/.tsx` trực tiếp qua transpiler của Bun | Thường cần build bằng `tsc`, `tsx`, `ts-node`, SWC hoặc bundler |
| Package manager | Có `bun install`, lockfile riêng, cache nhanh | Dùng npm/yarn/pnpm |
| Test runner | Có `bun test`, Jest-like API | Thường dùng Jest/Vitest/node:test |
| Bundler | Có `bun build` | Thường dùng esbuild, swc, tsup, webpack, Vite |
| Compatibility | Hướng tới Node.js compatibility nhưng vẫn cần kiểm tra package cụ thể | Hệ sinh thái ổn định nhất |
| Điểm mạnh | Startup nhanh, DX tốt, ít tool rời | Maturity, ecosystem, production battle-tested |

Interview answer:

> Bun không chỉ là package manager. Nó là runtime thay Node.js trong một số case, đồng thời gom nhiều tool như package manager, test runner và bundler. Tôi thích Bun cho tool nội bộ, crawler, script, service nhỏ hoặc dev workflow. Với service business critical, tôi đánh giá compatibility và monitoring trước khi chọn Bun thay Node.js.

### Bun so với npm/yarn/pnpm

So sánh này dễ bị hỏi sai. `npm`, `yarn`, `pnpm` chủ yếu là package manager; Bun rộng hơn vì có runtime.

* `bun install` cạnh tranh với `npm install`, `yarn install`, `pnpm install`.
* `bun run` cạnh tranh với `npm run`/`yarn`.
* Nhưng `bun server.ts` là chạy bằng Bun runtime, không chỉ quản lý package.

Nên trả lời:

> Nếu chỉ nói install dependency thì Bun cạnh tranh với npm/yarn/pnpm. Nhưng Bun còn chạy code, test và bundle, nên so sánh đầy đủ hơn là Bun vs Node.js ecosystem toolchain.

### Bun so với ts-node/tsx

`ts-node`/`tsx` giúp chạy TypeScript trên Node.js. Bun chạy TypeScript trực tiếp bằng runtime/transpiler của Bun.

Trade-off:

* Bun ít setup hơn cho script/tool.
* `tsx` vẫn giữ runtime Node.js nên compatibility giống production Node hơn.
* Nếu production chạy Node, dùng `tsx` trong dev có thể ít lệch runtime hơn Bun.
* Nếu production chạy Bun, dùng Bun end-to-end giúp dev/prod gần nhau hơn.

### Bun so với Vite

Vite chủ yếu là frontend dev server/bundler ecosystem, tối ưu DX cho browser app. Bun có bundler và runtime server-side.

* React/Vue app: Vite vẫn rất mạnh vì plugin ecosystem, HMR, framework integration.
* Backend/tooling/script: Bun thường gọn hơn.
* Có thể dùng chung: `bun install`, `bun run dev`, nhưng project frontend vẫn chạy Vite.

### Khi nào dùng Bun?

Phù hợp:

* Script TypeScript, crawler, ETL nhỏ.
* Internal tool.
* API service nhỏ cần startup nhanh.
* Monorepo muốn install/script nhanh.
* Test nhanh với `bun test` nếu assertion/mocking đủ nhu cầu.

Cẩn thận:

* Dependency dùng Node native addon hoặc API Node chưa tương thích hoàn toàn.
* Framework/plugin giả định runtime Node.js.
* Observability agent/APM chưa hỗ trợ Bun tốt.
* Production team đã chuẩn hóa Node.js image, debugging, profiling.
* Cần behavior giống AWS Lambda/managed runtime Node.js.

### Bun production checklist

Trước khi chạy backend production bằng Bun:

* Chạy integration test với DB/Redis/RabbitMQ/HTTP client thật hoặc test container.
* Kiểm tra package compatibility, đặc biệt package native.
* Kiểm tra Docker image, signal handling, graceful shutdown.
* Kiểm tra logging, tracing, metrics, APM.
* So sánh latency/startup/memory bằng workload của mình, không chỉ benchmark hello-world.
* Có fallback plan nếu dependency hoặc runtime behavior lệch Node.js.

---

## 4. Event loop

Event loop là cơ chế giúp Node.js xử lý nhiều I/O concurrent mà không cần một thread cho mỗi request.

Các nhóm phase quan trọng:

* Timers: callback của `setTimeout`, `setInterval`.
* Pending callbacks: một số callback hệ thống.
* Poll: nhận I/O event, chạy I/O callbacks.
* Check: callback của `setImmediate`.
* Close callbacks: callback khi socket/handle đóng.

Microtask queue:

* `Promise.then/catch/finally`
* `queueMicrotask`
* `process.nextTick` trong Node có ưu tiên rất cao.

Thứ tự thường hỏi:

```ts
console.log("A");

setTimeout(() => console.log("B"), 0);

Promise.resolve().then(() => console.log("C"));

console.log("D");
```

Output:

```text
A
D
C
B
```

Giải thích: sync code chạy trước, microtask Promise chạy sau call stack hiện tại, timer callback chạy ở phase sau.

---

## 5. Event loop bị block

Event loop bị block khi main thread bận chạy CPU-bound task hoặc synchronous I/O lâu.

Ví dụ xấu:

```ts
const data = fs.readFileSync("large.json", "utf8");
JSON.parse(data);
```

Hậu quả:

* Request khác không được xử lý kịp.
* Latency spike.
* Health check có thể timeout.
* WebSocket/stream bị delay.

Cách xử lý:

* Dùng async I/O.
* Chia nhỏ batch.
* Dùng worker_threads cho CPU-bound.
* Đẩy job nặng sang queue/worker service.
* Scale process bằng cluster/container replicas.
* Theo dõi event loop lag.

Interview answer:

> Node.js không phải không scale vì single-thread. Nó scale tốt cho I/O-bound nếu không block event loop. Với CPU-bound hoặc ETL nặng, tôi tách sang worker thread, queue worker hoặc service riêng.

---

## 6. Callback, Promise và async/await

Callback là function được gọi sau khi async operation hoàn thành.

Vấn đề callback:

* Callback hell.
* Error-first convention dễ quên handle error.
* Control flow phức tạp khi nhiều bước phụ thuộc nhau.

Promise giúp biểu diễn giá trị tương lai:

* `pending`
* `fulfilled`
* `rejected`

`async/await` là syntax trên Promise, giúp code đọc giống sync hơn.

```ts
async function createOrder(input: CreateOrderInput): Promise<Order> {
  const user = await userRepo.findById(input.userId);
  if (!user) throw new NotFoundError("user not found");
  return orderRepo.create(input);
}
```

Lưu ý:

* `await` chỉ chờ Promise, không làm CPU-bound tự chạy song song.
* Quên `await` có thể tạo unhandled rejection hoặc return Promise ngoài ý muốn.
* `try/catch` bắt lỗi trong `await`.
* Với tác vụ độc lập, dùng `Promise.all` thay vì await tuần tự.

---

## 7. Promise.all, allSettled, race, any

`Promise.all`:

* Chạy concurrent nhiều Promise.
* Fail fast khi một Promise reject.
* Hợp khi tất cả kết quả đều bắt buộc.

`Promise.allSettled`:

* Chờ tất cả fulfilled/rejected.
* Hợp cho batch job, crawl nhiều URL, gửi notification nhiều target.

`Promise.race`:

* Lấy Promise đầu tiên settled.
* Có thể dùng làm timeout wrapper, nhưng cần cẩn thận cancel operation thật sự.

`Promise.any`:

* Lấy Promise đầu tiên fulfilled.
* Chỉ reject khi tất cả reject.

```ts
const [profile, metrics] = await Promise.all([
  userClient.getProfile(userId),
  analyticsClient.getMetrics(userId),
]);
```

Trade-off:

* Concurrent call tăng throughput nhưng có thể tăng tải lên downstream.
* Cần timeout, retry, circuit breaker hoặc concurrency limit.

---

## 8. Concurrency limit trong Bun/Node.js

Không nên `Promise.all` trên hàng nghìn item nếu mỗi item gọi DB/API ngoài.

Rủi ro:

* Cạn connection pool.
* Rate limit từ dependency.
* Memory tăng.
* Retry storm.

Pattern:

* Batch theo chunk.
* Dùng queue.
* Dùng concurrency limiter như `p-limit`.
* Đẩy sang RabbitMQ/worker.

```ts
for (const batch of chunks(items, 50)) {
  await Promise.all(batch.map(processItem));
}
```

---

## 9. TypeScript type system nâng cao

TypeScript dùng structural typing: type tương thích nếu shape tương thích, không cần cùng tên class/interface.

```ts
type HasId = { id: string };

const user = { id: "u1", name: "Ninh" };
const entity: HasId = user; // OK
```

Điểm phỏng vấn:

* TS kiểm tra ở compile-time, runtime không còn type.
* Dữ liệu external phải validate bằng zod/class-validator/joi hoặc custom parser.
* Structural typing tiện cho object literal/API DTO nhưng có thể gây nhận nhầm shape nếu type quá rộng.

---

## 10. `unknown` vs `any`

`any` tắt type checking. Dùng nhiều sẽ làm TypeScript mất ý nghĩa.

`unknown` bắt buộc narrow trước khi dùng.

```ts
function parse(input: unknown) {
  if (typeof input !== "string") {
    throw new Error("input must be string");
  }

  return input.toUpperCase();
}
```

Interview answer:

> Với dữ liệu từ HTTP body, queue message hoặc crawler, tôi ưu tiên `unknown` ở boundary, validate runtime, rồi chuyển thành typed DTO/domain object. Tôi tránh `any` trừ khi bridge với library quá dynamic.

---

## 11. Type narrowing

Narrowing là quá trình TypeScript thu hẹp type dựa trên check.

Cách narrow:

* `typeof`
* `instanceof`
* `in`
* discriminated union
* custom type guard

```ts
type Event =
  | { type: "order.created"; orderId: string }
  | { type: "payment.failed"; paymentId: string; reason: string };

function handle(event: Event) {
  switch (event.type) {
    case "order.created":
      return event.orderId;
    case "payment.failed":
      return event.reason;
  }
}
```

Discriminated union rất hợp cho queue events, state machine, UI state, sync status.

---

## 12. Exhaustiveness checking

Dùng `never` để bắt thiếu case khi union mở rộng.

```ts
function assertNever(x: never): never {
  throw new Error(`unexpected value: ${JSON.stringify(x)}`);
}

function handle(event: Event) {
  switch (event.type) {
    case "order.created":
      return event.orderId;
    case "payment.failed":
      return event.reason;
    default:
      return assertNever(event);
  }
}
```

Điểm ăn điểm:

> Với event-driven system, discriminated union + exhaustiveness check giúp tránh quên handler khi thêm event type mới.

---

## 13. Interface vs type

Cả `interface` và `type` đều mô tả shape object.

Dùng `interface` khi:

* Public object shape.
* Class implements.
* Muốn declaration merging.

Dùng `type` khi:

* Union/intersection.
* Utility type.
* Mapped/conditional type.
* Alias phức tạp.

```ts
interface UserRepository {
  findById(id: string): Promise<User | null>;
}

type OrderEvent =
  | { type: "order.created"; orderId: string }
  | { type: "order.cancelled"; orderId: string; reason: string };
```

Interview answer:

> Tôi thường dùng interface cho contract service/repository và type cho union, DTO transform hoặc utility type.

---

## 14. Generics

Generics giúp giữ type safety khi viết abstraction dùng lại.

Use cases thực tế:

* API response wrapper.
* Pagination result.
* Repository helper.
* Event envelope.
* Form/table component trong React.

```ts
type Page<T> = {
  items: T[];
  nextCursor?: string;
};

type EventEnvelope<TPayload> = {
  id: string;
  type: string;
  payload: TPayload;
  occurredAt: string;
};
```

Tránh generics khi chỉ có một use case hoặc làm type quá khó đọc.

---

## 15. Utility types

Các utility type hay dùng trong backend/frontend:

* `Partial<T>`: update DTO, patch object.
* `Required<T>`: bắt buộc tất cả field.
* `Pick<T, K>`: chọn field.
* `Omit<T, K>`: bỏ field.
* `Record<K, V>`: map typed key-value.
* `Readonly<T>`: immutable contract.
* `ReturnType<T>`: lấy return type từ function.
* `Awaited<T>`: unwrap Promise.

```ts
type CreateUserDTO = Pick<User, "email" | "name">;
type UpdateUserDTO = Partial<CreateUserDTO>;
type UserPublic = Omit<User, "passwordHash">;
```

Rủi ro:

* Dùng utility quá nhiều có thể làm DTO phụ thuộc domain model quá chặt.
* API contract public nên rõ ràng, không nên auto derive quá mức từ DB entity.

---

## 16. Conditional type và mapped type

Mapped type biến đổi field của object type.

```ts
type Nullable<T> = {
  [K in keyof T]: T[K] | null;
};
```

Conditional type chọn type dựa trên điều kiện.

```ts
type ApiResult<T> = T extends Error
  ? { ok: false; error: string }
  : { ok: true; data: T };
```

Nên biết concept để đọc code/library, nhưng không cần khoe quá sâu nếu vị trí backend không yêu cầu type-level programming.

---

## 17. Runtime validation

TypeScript không validate runtime. Khi nhận data từ HTTP/queue/crawler/DB raw, cần validate.

```ts
const CreateOrderSchema = z.object({
  userId: z.string().uuid(),
  items: z.array(z.object({
    productId: z.string(),
    quantity: z.number().int().positive(),
  })),
});

const dto = CreateOrderSchema.parse(req.body);
```

Interview answer:

> TypeScript type chỉ tồn tại lúc compile. Với API backend, tôi validate runtime ở boundary rồi mới đưa data vào service/domain. Điều này đặc biệt quan trọng với webhook, queue message và crawler data.

---

## 18. Error handling trong Bun/Node.js

Các nguồn lỗi:

* Throw sync error.
* Promise rejection.
* Callback error-first.
* Stream error event.
* Process-level uncaught exception/unhandled rejection.

Best practices:

* Dùng centralized error middleware trong Express/Nest/Fastify.
* Không swallow error trong async function.
* Map domain error sang HTTP status.
* Log kèm request id/correlation id.
* Shutdown graceful với lỗi process-level nghiêm trọng.

```ts
try {
  await service.createOrder(dto);
} catch (err) {
  if (err instanceof NotFoundError) {
    throw new HttpError(404, err.message);
  }
  throw err;
}
```

---

## 19. Module system: CommonJS vs ESM

CommonJS:

* `require`, `module.exports`
* Load sync
* Truyền thống trong Node.js

ESM:

* `import`, `export`
* Hỗ trợ tree-shaking tốt hơn trong bundler
* Có top-level await
* Ngày càng phổ biến

Vấn đề hay gặp:

* Mixing CJS/ESM gây lỗi import.
* `default import` khác nhau tùy `esModuleInterop`.
* Tooling tsconfig/package.json phải thống nhất.

---

## 20. Bun/Node.js service production

Những điểm CV backend dễ bị hỏi:

* Request validation.
* Timeout cho HTTP client/DB/Redis.
* Connection pool.
* Graceful shutdown.
* Structured logging.
* Metrics: request latency, error rate, event loop lag.
* Health/readiness endpoint.
* Rate limit và auth middleware.
* Idempotency cho order/payment/sync.

Graceful shutdown:

* Stop nhận request mới.
* Chờ request hiện tại trong timeout.
* Stop queue consumer.
* Close DB/Redis/RabbitMQ connection.
* Exit process.

---

## 21. TypeScript với RabbitMQ/event-driven

Nên mô hình hóa event bằng discriminated union hoặc typed envelope.

```ts
type DomainEvent =
  | { type: "order.created"; payload: { orderId: string } }
  | { type: "order.cancelled"; payload: { orderId: string; reason: string } };

type EventEnvelope<T extends DomainEvent> = {
  id: string;
  occurredAt: string;
  event: T;
};
```

Production concerns:

* Validate JSON message runtime.
* Idempotency key/event id.
* Retry with backoff.
* DLQ.
* Ack sau khi xử lý thành công.
* Không xử lý message CPU-heavy trên event loop nếu làm block service.

---

## 22. TypeScript với React trong CV

Vì CV ghi React TypeScript nhưng role chính là backend, chỉ cần chắc các điểm practical:

* Props/state typing.
* API response type.
* Form validation.
* Discriminated union cho UI loading/error/success state.
* Không tin type từ backend nếu chưa validate/handle error.

```ts
type AsyncState<T> =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "success"; data: T }
  | { status: "error"; message: string };
```

---

## 23. Câu hỏi phỏng vấn dễ gặp

### Event loop là gì?

Event loop là cơ chế Node.js dùng để xử lý async I/O trên một main thread. Sync code chạy trước, sau đó microtask như Promise, rồi các callback ở phase như timer/I/O/check. Nếu main thread bị CPU-bound task block thì request khác cũng bị delay.

### Bun khác Node.js ở đâu?

Bun là runtime/toolkit all-in-one, dùng JavaScriptCore và có sẵn package manager, test runner, bundler, TypeScript transpiler. Node.js dùng V8 và có ecosystem production mature hơn. Bun rất tốt cho DX, script, crawler, service nhỏ; với backend critical cần kiểm tra compatibility, observability và dependency trước.

### Bun có phải chỉ là npm/yarn/pnpm nhanh hơn không?

Không. `bun install` cạnh tranh với package manager, nhưng Bun còn là runtime để chạy code, script runner, test runner và bundler. So sánh đầy đủ hơn là Bun vs Node.js + npm/pnpm + tsx/ts-node + Jest/Vitest + bundler.

### Khi nào không nên dùng Bun production?

Khi dependency hoặc framework phụ thuộc Node API/native addon chưa tương thích, APM/monitoring chưa hỗ trợ tốt, môi trường deploy chuẩn hóa Node.js, hoặc service quá critical mà team chưa có kinh nghiệm debug/profiling Bun.

### Promise khác callback thế nào?

Callback truyền function để gọi sau, dễ callback hell và khó compose. Promise biểu diễn kết quả tương lai, chain được, kết hợp được bằng `Promise.all/allSettled/race`, và `async/await` giúp control flow rõ hơn.

### `Promise.all` có chạy song song không?

Nó chạy concurrent các async operation đã tạo, nhưng không biến CPU-bound JavaScript thành parallel. Nếu các Promise là I/O thì Node có thể chờ nhiều I/O cùng lúc. Nếu CPU-bound thì vẫn block main thread trừ khi dùng worker thread/process khác.

### Vì sao TypeScript vẫn cần runtime validation?

Vì type bị xóa khi compile sang JavaScript. Data từ request body, queue message, webhook, crawler hoặc DB raw có thể sai shape. Cần validate ở boundary bằng schema/parser rồi mới dùng type an toàn trong service.

### `unknown` khác `any` thế nào?

`any` bỏ qua type checking. `unknown` vẫn nhận mọi giá trị nhưng bắt buộc kiểm tra/narrow trước khi dùng. Với dữ liệu external, `unknown` an toàn hơn.

### Interface khác type thế nào?

Interface hợp mô tả object contract và có declaration merging. Type hợp cho union, intersection, mapped/conditional type. Trong backend, interface hay dùng cho repository/service contract; type hay dùng cho DTO/event union.

### Bun/Node.js xử lý CPU-bound task thế nào?

Không chạy CPU-bound lâu trên event loop. Có thể dùng worker_threads, child process, queue worker, service riêng, batch nhỏ hơn hoặc chuyển sang ngôn ngữ/runtime phù hợp hơn.

### Làm sao tránh overload downstream khi crawl/gọi nhiều API?

Không `Promise.all` không giới hạn trên toàn bộ input. Dùng concurrency limit, batch, timeout, retry có backoff, rate limit và circuit breaker nếu cần.
