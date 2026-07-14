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

### CPU-bound vs I/O-bound

Phân biệt được hai nhóm này là nền tảng để trả lời mọi câu hỏi về event loop, worker thread và scale.

CPU-bound (main thread phải tính toán, event loop bị chiếm):

* Resize/crop image.
* Encode/transcode video.
* Generate PDF, render template nặng.
* `bcrypt` với cost cao (10+ rounds).
* Nén/giải nén file 10GB.
* OCR, parse/diff data lớn trong memory.
* `JSON.parse` một payload vài trăm MB.

I/O-bound (main thread chỉ chờ, event loop vẫn rảnh để làm việc khác):

* Query Postgres/MongoDB.
* Redis get/set.
* Publish/consume RabbitMQ.
* HTTP call sang service khác.
* Upload/download S3.
* Đọc/ghi filesystem (async).

Interview answer:

> Node.js scale rất tốt cho I/O-bound vì trong lúc chờ network/disk, event loop tiếp tục xử lý request khác. Với CPU-bound thì main thread bị chiếm, nên tôi đẩy sang worker_threads, queue worker hoặc service riêng.

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

Về benchmark, đừng trả lời kiểu:

> "Benchmark trên mạng nói Bun nhanh hơn Node mấy lần."

Interviewer muốn nghe cách bạn tự đo trên workload thật:

```text
Benchmark trên chính workload của project

  Throughput      1000 req/s có giữ được không?
  Latency         p50 / p95 / p99
  Memory          RSS/heap khi chạy dài (leak?)
  CPU             usage lúc peak
  Startup         thời gian boot container (quan trọng khi autoscale)
  Integration     test với DB/Redis/RabbitMQ thật
```

Điểm mấu chốt: benchmark hello-world không nói gì về service có ORM, JSON serialization lớn, TLS, connection pool và middleware chain thật.

---

## 4. Event loop

Event loop là cơ chế giúp Node.js xử lý nhiều I/O concurrent mà không cần một thread cho mỗi request.

Hình dung luồng chạy:

```text
Call Stack                       <- sync code chạy tới khi stack rỗng
    │
    │   console.log("A")
    │   console.log("D")
    ▼
Microtask Queue                  <- vét sạch sau MỖI lần stack rỗng
    │
    │   process.nextTick(...)    <- ưu tiên cao nhất (queue riêng)
    │   Promise.then(...)
    │   queueMicrotask(...)
    ▼
Macrotask (phase của event loop)
        setTimeout / setInterval   (timers)
        I/O callback               (poll)
        setImmediate               (check)
```

Quy tắc nhớ: **sync → nextTick → promise microtask → macrotask**. Giữa mỗi macrotask, microtask queue lại được vét sạch.

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

Bản khó hơn, có `process.nextTick` (câu này rất hay gặp):

```ts
console.log(1);

setTimeout(() => console.log(2), 0);

Promise.resolve().then(() => {
  console.log(3);
});

process.nextTick(() => {
  console.log(4);
});

console.log(5);
```

Output:

```text
1
5
4
3
2
```

Giải thích từng bước:

1. `1` và `5` là sync code, chạy ngay trên call stack.
2. `4` — `process.nextTick` có queue riêng, được vét **trước** promise microtask.
3. `3` — promise microtask chạy sau nextTick queue, vẫn trước khi event loop sang phase tiếp theo.
4. `2` — `setTimeout` là macrotask ở phase timers, chạy cuối cùng.

Hệ quả thực tế: `process.nextTick` đệ quy vô hạn sẽ **starve** event loop, timer và I/O callback không bao giờ chạy được.

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

### Lỗi quên `await` — cực kỳ hay gặp

```ts
async function createOrder() {
  saveOrder(); // ❌ trả về Promise<void>, không ai chờ
}
```

Điều gì xảy ra:

* `createOrder()` resolve **ngay lập tức**, trước khi `saveOrder()` xong.
* Nếu `saveOrder()` throw, lỗi thành **unhandled rejection**, không vào `try/catch` của caller.
* Test pass ở local (DB nhanh) nhưng flaky/mất data trên production.
* Nếu đây là HTTP handler, response 200 trả về trước khi ghi DB xong.

Đúng phải là:

```ts
async function createOrder() {
  await saveOrder(); // ✅
}
```

Câu chốt khi phỏng vấn:

> `async` chỉ đảm bảo function trả về Promise, nó **không tự động chờ** các Promise bên trong. Muốn chờ thì phải `await` (hoặc `return` promise đó).

Bật rule `no-floating-promises` của `@typescript-eslint` để compiler bắt lỗi này giúp.

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

### Concurrent ≠ parallel

```ts
await Promise.all([
  fetchUser(),
  fetchOrders(),
  fetchCoupons(),
]);
```

Giả sử mỗi API mất 1 giây.

Sequential (`await` từng cái):

```text
fetchUser     |----1s----|
fetchOrders               |----1s----|
fetchCoupons                          |----1s----|

Tổng = 3s
```

Concurrent (`Promise.all`):

```text
fetchUser     |----1s----|
fetchOrders   |----1s----|
fetchCoupons  |----1s----|

Tổng ≈ 1s
```

Nhưng phải nói rõ:

> JavaScript vẫn chỉ có **một execution thread**. `Promise.all` không tạo thêm thread nào. Nó chỉ phát cả 3 request đi rồi trong lúc chờ network, event loop tiếp tục xử lý việc khác. Đó là **concurrency**, không phải **parallelism**.

Hệ quả: nếu 3 việc đó là CPU-bound (hash, resize ảnh) thì `Promise.all` **không** giúp nhanh hơn — vẫn 3s, vì main thread phải tính lần lượt.

### `Promise.all` fail fast

```ts
await Promise.all([
  Promise.resolve(1),
  Promise.reject(new Error("boom")),
  Promise.resolve(3),
]);
// -> reject NGAY với Error("boom")
```

Rất nhiều người hiểu sai chỗ này. Cần nắm:

* Promise trả về reject **ngay** khi có một cái reject đầu tiên, không chờ các cái còn lại.
* Nhưng Promise thứ 3 **vẫn đang chạy** — nó không bị cancel. Request HTTP vẫn bay đi, row vẫn được ghi vào DB.
* Nếu Promise còn lại reject sau đó mà không ai catch → unhandled rejection warning.

Nói cách khác: fail fast là fail fast ở chỗ **chờ kết quả**, không phải hủy công việc. JavaScript không có cancel Promise built-in — muốn hủy thật phải dùng `AbortController`.

### `Promise.allSettled` — ví dụ crawl

Crawl 100 website:

```text
website A  ❌ timeout
website B  ✅
website C  ❌ 403
website D  ✅
...
```

Dùng `Promise.all` → chỉ cần website A fail là **mất luôn** kết quả của 99 site còn lại.

Dùng `Promise.allSettled` → lấy được kết quả của cả 100 site, rồi tự quyết định site nào retry, site nào bỏ.

```ts
const results = await Promise.allSettled(urls.map(crawl));

const ok = results.filter((r) => r.status === "fulfilled").map((r) => r.value);
const failed = results
  .map((r, i) => ({ r, url: urls[i] }))
  .filter(({ r }) => r.status === "rejected");
```

Quy tắc chọn: **tất cả kết quả đều bắt buộc → `Promise.all`. Best-effort/batch job → `Promise.allSettled`.**

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

### Ví dụ production: gửi email cho 100.000 user

Sai:

```ts
await Promise.all(users.map(sendEmail));
```

```text
100.000 user
    ↓
100.000 request bắn đi cùng lúc
    ↓
SMTP provider rate-limit / block IP
connection pool cạn
memory tăng vọt vì 100.000 promise + 100.000 response buffer
    ↓
Service chết
```

Đúng — giới hạn concurrency:

```ts
import pLimit from "p-limit";

const limit = pLimit(10); // tối đa 10 request cùng lúc

await Promise.all(
  users.map((user) => limit(() => sendEmail(user))),
);
```

Lưu ý: `users.map(...)` vẫn tạo 100.000 promise object, nhưng `p-limit` đảm bảo chỉ 10 cái **thực sự chạy** cùng lúc. Nếu input lớn tới mức chính mảng promise cũng nặng, hãy chuyển sang stream/queue thay vì giữ hết trong memory.

Chọn concurrency bao nhiêu? Đừng đoán — lấy theo giới hạn nhỏ nhất của downstream: DB connection pool size, rate limit của API, số worker của SMTP.

---

## 9. TypeScript type system nâng cao

TypeScript dùng structural typing: type tương thích nếu shape tương thích, không cần cùng tên class/interface.

```ts
type HasId = { id: string };

const user = { id: "u1", name: "Ninh" };
const entity: HasId = user; // OK
```

Ví dụ dễ gây bất ngờ: hai interface **khác tên hoàn toàn** vẫn gán được cho nhau.

```ts
interface User {
  id: string;
}

interface Employee {
  id: string;
}

const u: User = { id: "1" };

const e: Employee = u; // ✅ compile OK
```

TypeScript so **shape**, không so **tên**. Đây gọi là structural typing (duck typing), khác với Java/C# dùng nominal typing (phải `implements` đúng interface mới gán được).

Khi cần chặt hơn, dùng branded type để giả lập nominal typing:

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };

declare const userId: UserId;
const orderId: OrderId = userId; // ❌ Type 'UserId' is not assignable to 'OrderId'
```

Rất hữu ích khi codebase có nhiều id kiểu `string` dễ truyền nhầm chỗ.

Điểm phỏng vấn:

* TS kiểm tra ở compile-time, runtime không còn type.
* Dữ liệu external phải validate bằng zod/class-validator/joi hoặc custom parser.
* Structural typing tiện cho object literal/API DTO nhưng có thể gây nhận nhầm shape nếu type quá rộng.

---

## 10. `unknown` vs `any`

`any` tắt type checking. Dùng nhiều sẽ làm TypeScript mất ý nghĩa.

`unknown` bắt buộc narrow trước khi dùng.

So sánh trực quan:

```ts
let x: any = 123;

x.toUpperCase();
// compile: OK  ✅
// runtime : TypeError: x.toUpperCase is not a function  💥
```

```ts
let x: unknown = 123;

x.toUpperCase();
// compile: ❌ Property 'toUpperCase' does not exist on type 'unknown'
```

`any` đẩy lỗi sang runtime (production). `unknown` bắt lỗi ngay lúc compile — bug được phát hiện ở CI thay vì lúc 2h sáng.

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

### Custom type guard

Khi check phức tạp hơn `typeof`/`in`, viết type guard riêng. Chữ ký `x is User` là "type predicate" — nó dạy TypeScript cách narrow.

```ts
function isUser(x: unknown): x is User {
  return (
    typeof x === "object" &&
    x !== null &&
    typeof (x as User).id === "string"
  );
}

function handle(input: unknown) {
  if (isUser(input)) {
    input.id; // ✅ TS biết input là User trong block này
  }
}
```

Cảnh báo hay bị hỏi lại: type predicate là **lời hứa của developer**, TS không verify logic bên trong. Viết ẩu như dưới đây thì compile vẫn pass nhưng runtime vẫn crash:

```ts
function isUser(x: any): x is User {
  return true; // 💥 nói dối compiler
}
```

Vì thế với data từ HTTP/queue, nên dùng zod thay vì tự viết type guard — zod vừa validate runtime vừa infer type luôn.

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

### Declaration merging — điểm khác biệt hay bị hỏi nhất

`interface` cùng tên sẽ **tự động merge**:

```ts
interface User {
  id: string;
}

interface User {
  name: string;
}

// => User = { id: string; name: string }
const u: User = { id: "1", name: "Ninh" }; // ✅
```

`type` thì không:

```ts
type User = { id: string };
type User = { name: string };
// ❌ Error: Duplicate identifier 'User'
```

Vì sao điều này quan trọng trong backend? Vì đó chính là cách bạn mở rộng type của thư viện bên thứ ba:

```ts
// Thêm field user vào Request của Express sau khi auth middleware chạy
declare global {
  namespace Express {
    interface Request {
      user?: { id: string; role: string };
    }
  }
}
```

Không có declaration merging thì `req.user` sẽ luôn báo lỗi type. Đây là lý do `interface` được ưu tiên cho public API/contract, còn `type` cho union và composition.

Interview answer:

> Tôi thường dùng interface cho contract service/repository và type cho union, DTO transform hoặc utility type. Interface có declaration merging nên hợp để augment type của library như Express `Request`; type thì không merge được nhưng biểu diễn union/mapped/conditional tốt hơn.

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

### Generic Repository — use case backend thực tế nhất

Viết một lần, dùng cho mọi entity:

```ts
interface Repository<T> {
  findById(id: string): Promise<T | null>;
  create(data: Omit<T, "id">): Promise<T>;
  update(id: string, patch: Partial<T>): Promise<T>;
  delete(id: string): Promise<void>;
}
```

Dùng lại:

```ts
Repository<User>
Repository<Order>
Repository<Product>
```

Giá trị thật của generics ở đây: `userRepo.findById()` trả về `User | null` chứ không phải `any`, nên IDE autocomplete đúng field và refactor an toàn. Nếu không có generic, bạn hoặc phải copy-paste 3 interface gần như giống hệt, hoặc phải dùng `any` và mất hết type safety.

Ràng buộc generic bằng `extends` khi cần đảm bảo shape tối thiểu:

```ts
interface Repository<T extends { id: string }> {
  findById(id: string): Promise<T | null>;
}
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

Bảng "utility nào cho tình huống nào" — nhớ nhanh khi phỏng vấn:

| Utility | Use case thực tế | Ví dụ |
|---|---|---|
| `Partial<T>` | PATCH API, update một phần | `updateUser(id, patch: Partial<User>)` |
| `Pick<T, K>` | DTO, chỉ nhận field cần thiết | `Pick<User, "email" \| "name">` |
| `Omit<T, K>` | Ẩn field nhạy cảm khỏi response | `Omit<User, "passwordHash">` |
| `Record<K, V>` | Cache map, lookup table, config theo key | `Record<string, CachedUser>` |
| `Required<T>` | Config sau khi merge default | `Required<AppConfig>` |
| `Readonly<T>` | Immutable contract, tránh mutate nhầm | `Readonly<OrderEvent>` |
| `ReturnType<T>` | Lấy type từ factory/service có sẵn | `ReturnType<typeof createClient>` |
| `Awaited<T>` | Unwrap Promise | `Awaited<ReturnType<typeof getUser>>` |

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

### Ví dụ khiến interviewer gật đầu

Client gửi lên:

```json
{
  "age": "20"
}
```

Code TypeScript:

```ts
interface User {
  age: number;
}

app.post("/users", (req, res) => {
  const user = req.body as User; // ❌ "as" chỉ là lời hứa, không kiểm tra gì
  res.json({ age: user.age.toFixed(2) });
});
```

Compile: **pass**. Runtime: `"20".toFixed is not a function` → **crash**.

Vì sao? `req.body` thực chất là `any`. `as User` chỉ nói với compiler "tin tôi đi", nó **không sinh ra một dòng code kiểm tra nào**. Toàn bộ type bị xóa sạch khi transpile sang JavaScript.

Câu chốt:

> TypeScript **không validate JSON**. Type bị erase lúc compile. Ranh giới hệ thống (HTTP body, query param, queue message, webhook, response của API bên thứ ba, kết quả raw từ DB) đều là `unknown` cho tới khi được parse bằng schema.

Cách đúng — parse, đừng cast:

```ts
const CreateUserSchema = z.object({
  age: z.number().int().positive(),
});

const dto = CreateUserSchema.parse(req.body); // throw nếu age là "20"
dto.age.toFixed(2); // ✅ giờ mới an toàn
```

Khẩu quyết: **"Parse, don't validate"** — biến dữ liệu lạ thành typed object ngay tại boundary, phần còn lại của service không bao giờ phải nghi ngờ nữa.

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

### Error phân tầng

Mỗi tầng chỉ throw loại lỗi thuộc về ngôn ngữ của tầng đó. Tầng dưới **không** được biết gì về HTTP status.

```text
Controller  ──> HTTP layer      : map error -> 404 / 409 / 500
     ↑
Service     ──> Business layer  : throw NotFoundError, ConflictError, InsufficientStockError
     ↑
Repository  ──> Data layer      : throw DatabaseError, UniqueConstraintError
```

```ts
// Repository — chỉ biết về DB
async findById(id: string): Promise<User | null> {
  try {
    return await db.user.findUnique({ where: { id } });
  } catch (err) {
    throw new DatabaseError("query failed", { cause: err });
  }
}

// Service — chỉ biết về business rule
async getUser(id: string): Promise<User> {
  const user = await this.repo.findById(id);
  if (!user) throw new NotFoundError("user not found");
  return user;
}

// Controller — chỉ biết về HTTP
try {
  const user = await service.getUser(id);
  res.json(user);
} catch (err) {
  if (err instanceof NotFoundError) return res.status(404).json({ message: err.message });
  if (err instanceof ConflictError) return res.status(409).json({ message: err.message });
  next(err); // -> error middleware -> 500 + log stack
}
```

Vì sao phải tách? Vì service còn được gọi từ queue consumer, cron job, CLI — những nơi **không có HTTP**. Nếu service throw `HttpError(404)` thì tầng domain bị dính chặt vào web framework, không tái sử dụng và không unit test sạch được.

Nguyên tắc bổ sung:

* Dùng `{ cause: err }` để giữ nguyên stack gốc, đừng nuốt lỗi.
* Lỗi 5xx thì log full stack; lỗi 4xx thì log ở mức warn, đừng làm nhiễu alert.
* Không bao giờ trả message lỗi raw từ DB về client (lộ schema).

---

## 19. Module system: CommonJS vs ESM

CommonJS:

```ts
const fs = require("fs");

module.exports = { createServer };
```

* Load **sync** — `require` chạy ngay tại dòng đó.
* Có thể `require` động trong if/function.
* Truyền thống trong Node.js.

ESM:

```ts
import fs from "fs";

export { createServer };
```

* Load **async**, được phân tích tĩnh trước khi chạy.
* Chính vì tĩnh nên bundler biết code nào không dùng → tree-shaking tốt hơn.
* Có top-level await.
* Ngày càng phổ biến; Bun và Node hiện đại đều ưu tiên ESM.

Điểm mấu chốt hay bị hỏi: **ESM biết trước import graph, CJS thì không**. Đó là gốc rễ của cả tree-shaking lẫn việc CJS không `require` được ESM một cách đồng bộ.

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

### Graceful shutdown

Timeline — nhớ theo thứ tự này là trả lời được:

```text
SIGTERM  (K8s/Docker gửi khi rolling update, scale down)
   ↓
Stop accepting request       server.close(), fail readiness probe
   ↓
Finish current request       chờ in-flight request xong (có deadline)
   ↓
Stop RabbitMQ consumer       channel.cancel() — không nhận message mới,
                             nhưng ack nốt message đang xử lý
   ↓
Close DB / Redis / MQ        flush connection pool
   ↓
Exit process                 process.exit(0)
```

```ts
process.on("SIGTERM", async () => {
  server.close(); // ngừng nhận connection mới

  const timeout = setTimeout(() => process.exit(1), 15_000); // deadline cứng

  await consumer.cancel();   // ngừng nhận message mới
  await inFlightJobs.drain(); // chờ job đang chạy
  await db.end();
  await redis.quit();

  clearTimeout(timeout);
  process.exit(0);
});
```

Vì sao thứ tự này quan trọng:

* Đóng DB **trước** khi request in-flight xong → request đang chạy crash, user thấy 500.
* Không `cancel()` consumer → RabbitMQ vẫn đẩy message vào lúc app sắp chết → message bị nack/redeliver vô ích.
* Không có deadline cứng → nếu một request treo, pod không bao giờ chết, K8s phải `SIGKILL` sau grace period và bạn mất hết in-flight work.
* Nên fail readiness probe **trước** `server.close()` vài giây, để load balancer kịp rút pod ra khỏi pool.

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

### Flow RabbitMQ

```text
Producer
   ↓  publish(routingKey, message)
Exchange          (direct / topic / fanout — quyết định route đi đâu)
   ↓  binding
Queue             (message nằm đây, chờ consumer rảnh)
   ↓  deliver (prefetch giới hạn số message đang xử lý)
Consumer
   ↓  xử lý xong
Ack               (báo broker: xóa message đi được rồi)
```

### Ack trước hay ack sau? — câu backend rất hay hỏi

Ack **trước** khi xử lý:

```text
Nhận message
   ↓
Ack ngay          <- broker xóa message khỏi queue
   ↓
Xử lý...
   ↓
Consumer crash 💥
   ↓
MẤT MESSAGE       <- broker tưởng xong rồi, không redeliver
```

Ack **sau** khi xử lý xong:

```text
Nhận message
   ↓
Xử lý...
   ↓
Consumer crash 💥
   ↓
Không ack -> broker redeliver cho consumer khác  ✅ AN TOÀN
```

```ts
channel.consume(queue, async (msg) => {
  if (!msg) return;
  try {
    const event = OrderEventSchema.parse(JSON.parse(msg.content.toString()));
    await handleOrder(event);
    channel.ack(msg); // ✅ ack SAU khi xử lý thành công
  } catch (err) {
    channel.nack(msg, false, false); // -> DLQ, không requeue vô hạn
  }
}, { noAck: false });
```

Đánh đổi phải nói ra: ack-sau cho bạn **at-least-once delivery** — message có thể được xử lý **nhiều lần** (crash sau khi ghi DB nhưng trước khi ack). Vì vậy handler **bắt buộc phải idempotent**: dùng event id + bảng `processed_events`, hoặc `INSERT ... ON CONFLICT DO NOTHING`.

> Không có exactly-once trong distributed system. Chỉ có at-least-once + idempotency.

Production concerns:

* Validate JSON message runtime (message từ queue cũng là `unknown`).
* Idempotency key/event id.
* `prefetch` để một consumer không ôm quá nhiều message cùng lúc.
* Retry with backoff.
* DLQ cho message poison (parse fail, business invalid) — đừng requeue vô hạn.
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

## 23. Memory leak trong Node.js

Chủ đề rất hay bị đào ở level 3–5 năm, vì nó tách người "đã vận hành production" khỏi người "chỉ mới viết feature".

Node có GC, nhưng GC chỉ dọn được object **không còn ai reference tới**. Memory leak trong Node = bạn vô tình giữ reference mãi mãi.

### 5 nguyên nhân kinh điển

**1. Global cache không bao giờ dọn**

```ts
const cache: Record<string, User> = {};

app.get("/users/:id", async (req, res) => {
  cache[req.params.id] = await getUser(req.params.id); // ❌ chỉ lớn lên, không bao giờ nhỏ đi
  res.json(cache[req.params.id]);
});
```

Fix: dùng LRU cache có `max` size và TTL (`lru-cache`), hoặc đẩy sang Redis.

**2. Closure giữ reference lớn**

```ts
function createHandler() {
  const hugeBuffer = fs.readFileSync("500mb.bin"); // 500MB

  return () => {
    console.log(hugeBuffer.length); // closure giữ nguyên 500MB sống mãi
  };
}
```

Fix: chỉ capture đúng thứ cần (`const size = hugeBuffer.length` rồi capture `size`).

**3. EventEmitter không remove listener**

```ts
socket.on("message", handler); // ❌ mỗi connection add thêm, không bao giờ gỡ
```

Fix: `socket.off("message", handler)` khi disconnect, hoặc dùng `once`. Dấu hiệu nhận biết: warning `MaxListenersExceededWarning: Possible EventEmitter memory leak detected`.

**4. Timer không clear**

```ts
setInterval(() => poll(), 1000); // ❌ giữ closure + giữ process sống mãi
```

Fix: giữ handle và `clearInterval` lúc shutdown. Đây cũng là lý do process không chịu exit khi `SIGTERM`.

**5. Đọc file lớn bằng `readFile` thay vì stream**

```ts
const data = await fs.promises.readFile("2gb.csv"); // ❌ nạp 2GB vào RAM
```

Fix: dùng stream (xem mục 24).

### Cách phát hiện

* Theo dõi **RSS và heap used** theo thời gian. Leak = đường đi lên đều, không tụt xuống sau GC. Chỉ nhìn một thời điểm thì không kết luận được gì.
* `node --inspect` rồi mở `chrome://inspect` → **Memory** tab.
* Chụp **heap snapshot** ở 2 thời điểm cách nhau (ví dụ sau 10 phút chịu tải), rồi so sánh bằng chế độ **Comparison** — object nào tăng liên tục chính là thủ phạm.
* Expose metric `process.memoryUsage()` (`heapUsed`, `rss`, `external`) lên Prometheus/Grafana.
* Dấu hiệu production: pod bị OOMKilled và restart theo chu kỳ đều đặn.

Interview answer:

> Tôi xác nhận leak bằng cách theo dõi heap used tăng đơn điệu qua nhiều chu kỳ GC, chứ không kết luận từ một lần đo RSS cao. Sau đó chụp hai heap snapshot cách nhau và diff để tìm object nào tăng liên tục. Nguyên nhân hay gặp nhất trong project của tôi là cache không giới hạn size và listener không được gỡ.

---

## 24. Streams và xử lý file lớn (production)

Nếu CV có crawler, ETL, import/export CSV, inventory sync thì gần như chắc chắn bị hỏi.

Vấn đề:

```ts
const data = await fs.promises.readFile("10gb.csv"); // 💥 nạp 10GB vào RAM
```

Node có giới hạn heap (mặc định ~1.5–4GB tùy version), và kể cả không giới hạn thì cũng không có lý do gì phải giữ cả file trong memory.

Giải pháp — xử lý theo từng chunk:

```ts
import { pipeline } from "node:stream/promises";

await pipeline(
  fs.createReadStream("10gb.csv"),
  csvParser(),
  transformToOrder(),
  fs.createWriteStream("out.ndjson"),
);
```

Vì sao stream thắng:

* **Memory không đổi**: chỉ giữ một chunk (~64KB) tại một thời điểm, xử lý file 10MB hay 10GB đều tốn RAM như nhau.
* **Backpressure**: nếu writer chậm hơn reader, stream tự động bảo reader chậm lại. Đây là ý interviewer muốn nghe nhất — nếu không có backpressure, buffer sẽ phình ra cho tới khi OOM.
* **Time to first byte thấp**: bắt đầu xử lý ngay từ byte đầu, không phải chờ load hết.

Dùng `pipeline()` (từ `node:stream/promises`) thay vì `.pipe()` thủ công: `pipeline` tự động cleanup và propagate error trên toàn chain, còn `.pipe()` thì error ở một stage sẽ **không** tự đóng các stage khác → leak file descriptor.

4 loại stream cần biết: `Readable`, `Writable`, `Duplex`, `Transform` (đọc + biến đổi + ghi — dùng nhiều nhất trong ETL).

Interview answer:

> Với file lớn hoặc response HTTP lớn, tôi dùng stream để memory footprint không phụ thuộc kích thước dữ liệu, và dựa vào backpressure để producer không đẩy nhanh hơn khả năng consumer xử lý. Tôi dùng `pipeline()` thay vì `.pipe()` để error handling và cleanup được lo tự động.

### `readFile` hay stream?

| Dùng `readFile` | Dùng stream |
|---|---|
| `config.json`, `package.json` | CSV import 5GB |
| File vài KB–vài MB, biết chắc kích thước | Log 20GB, video, export |
| Cần cả object trong memory để xử lý | File do **user upload** (không kiểm soát được size) |

Ranh giới thực dụng: nếu kích thước file do người khác quyết định, luôn dùng stream.

### Backpressure

Đây là ý interviewer thích nhất.

```text
Read file    100 MB/s   ──┐
                          │  chênh 10x
Mongo insert  10 MB/s   ──┘

Không có backpressure -> phần dư dồn vào buffer -> RAM tăng dần -> OOM
```

Stream tự xử: khi Writable báo "buffer đầy", Readable **tự pause**, đến khi writer tiêu thụ xong mới `resume`. Điều kiện để cơ chế này hoạt động: bạn phải nối stream bằng `pipe()`/`pipeline()`, hoặc tôn trọng giá trị trả về của `write()`. Nếu bạn tự `.on("data")` rồi `await insert(...)` bên trong mà không `pause()`, backpressure **mất tác dụng** — đó là lỗi hay gặp nhất.

Cách an toàn nhất: dùng `for await`, nó tự động tôn trọng backpressure.

```ts
for await (const row of parser) {
  await handle(row); // stream tự pause trong lúc await
}
```

### CSV import — flow chuẩn

```text
inventory.csv
   ↓  createReadStream        (chunk 64KB, RAM không đổi)
   ↓  csv-parser              (parse thành object)
   ↓  validate (zod)          (row hỏng -> skip hoặc throw)
   ↓  transform -> DTO
   ↓  gom batch 1000 dòng
   ↓  bulkWrite / COPY        (1 lần gọi DB cho 1000 record)
   ↓  done
```

```ts
import { createReadStream } from "node:fs";
import csv from "csv-parser";

const batch: Order[] = [];

for await (const row of createReadStream("inventory.csv").pipe(csv())) {
  batch.push(RowSchema.parse(row));

  if (batch.length >= 1000) {
    await db.collection("inventory").bulkWrite(toBulkOps(batch));
    batch.length = 0; // reset, giữ RAM phẳng
  }
}

if (batch.length) await db.collection("inventory").bulkWrite(toBulkOps(batch));
```

Hai điểm phải nói ra:

* **Đừng insert từng dòng.** 1 triệu dòng = 1 triệu round-trip DB. Gom batch 500–1000 rồi `bulkWrite` (Mongo) / `COPY` hoặc multi-row `INSERT ... ON CONFLICT` (Postgres) — nhanh hơn hàng chục lần.
* **Đừng tự `split(",")`.** `"Coke, 330ml",10` sẽ parse sai ngay. Dùng `csv-parser`, `fast-csv` hoặc `csv-parse` — chúng xử lý quoted field, escape, newline trong cell.

### TXT / log — đọc từng dòng

```ts
import readline from "node:readline";

const rl = readline.createInterface({
  input: createReadStream("app.log"),
  crlfDelay: Infinity,
});

for await (const line of rl) {
  process(line);
}
```

`readline` lo việc gom chunk thành dòng hoàn chỉnh (một dòng có thể bị cắt ngang giữa 2 chunk 64KB — tự xử lý chỗ này rất dễ sai).

### JSON lớn

JSON không stream tự nhiên được vì phải đọc hết mới đóng ngoặc. File nhỏ thì `readFile` + `JSON.parse` là đủ; file vài GB thì dùng `stream-json` để emit từng object trong mảng.

Tốt hơn hết: nếu bạn **kiểm soát format**, dùng **NDJSON** (mỗi dòng một JSON object) thay vì một mảng JSON khổng lồ — khi đó đọc bằng `readline` là xong.

### XLSX — chỗ nhiều người sai

```ts
xlsx.readFile("500mb.xlsx"); // 💥 parse cả workbook vào RAM
```

XLSX là zip chứa XML, nên `readFile` thường ngốn RAM gấp **nhiều lần** kích thước file. Production dùng streaming reader:

```ts
import ExcelJS from "exceljs";

const reader = new ExcelJS.stream.xlsx.WorkbookReader("big.xlsx", {});

for await (const worksheet of reader) {
  for await (const row of worksheet) {
    handle(row.values);
  }
}
```

Ghi cũng vậy: dùng `ExcelJS.stream.xlsx.WorkbookWriter` (flush từng row xuống disk) thay vì `Workbook` (giữ hết trong RAM rồi mới ghi).

### Ghi file — export

Sai:

```ts
const rows = await db.find().toArray(); // 10 triệu record vào RAM
await fs.promises.writeFile("out.csv", rows.map(toCsv).join("\n"));
```

Đúng — stream thẳng từ DB cursor ra file/HTTP response:

```ts
await pipeline(
  db.collection("orders").find().stream(), // cursor là Readable
  toCsvTransform(),
  createWriteStream("out.csv"),           // hoặc res (HTTP response)
);
```

Với export cho user download, `pipeline(cursor, csv, res)` cho phép trả byte đầu tiên ngay, không cần buffer cả file — user không bị timeout chờ.

Tương tự với log: dùng một `WriteStream` mở sẵn rồi `write()` nhiều lần, đừng gọi `appendFile()` mỗi dòng (mỗi lần là một lần `open`/`write`/`close` syscall).

### Error handling: fail-fast hay skip?

Quyết định theo nghiệp vụ, và nói rõ lý do:

| | Fail-fast (throw + rollback) | Skip & collect (ghi lại row lỗi, chạy tiếp) |
|---|---|---|
| Khi nào | Import product, đơn hàng, dữ liệu tài chính | Crawler, analytics, log import |
| Vì sao | Dữ liệu phải nhất quán, import một nửa còn tệ hơn không import | Mất vài row không sao, dừng cả job mới là thảm họa |

Với file 100.000 dòng mà dòng 5.812 hỏng, cách thường dùng trong ETL là **skip + ghi row lỗi vào bảng `import_errors`**, cuối job trả report kiểu "99.812 dòng thành công, 188 dòng lỗi, tải file lỗi tại đây". Người dùng sửa file lỗi rồi import lại chỉ phần đó.

Bổ sung: nếu job có thể chạy lại giữa chừng, hãy lưu **checkpoint** (đã xử lý tới dòng nào) để không import trùng.

### Concurrency

Nếu mỗi row phải gọi API ngoài, đừng `Promise.all` cả nghìn row (xem mục 8) — dùng `p-limit(10)` hoặc xử lý theo batch. Riêng phần ghi DB thì batch đã đủ, thêm concurrency chỉ làm cạn connection pool.

### Câu hỏi kinh điển: "Import CSV 5GB, 20 triệu record thì làm sao?"

> Em không dùng `readFile()` vì nó nạp toàn bộ file vào heap và gần như chắc chắn OOM. Em dùng `createReadStream()` nối với parser như `csv-parser`/`fast-csv` để đọc tuần tự từng record, validate bằng zod rồi map sang DTO. Em gom 500–1000 record thành một batch và dùng `bulkWrite` (Mongo) hoặc `COPY`/multi-row insert (Postgres) thay vì insert từng dòng. Nhờ backpressure, nếu DB ghi chậm hơn tốc độ đọc thì stream tự pause nên memory giữ phẳng. Toàn bộ pipeline bọc trong `stream.pipeline()` để error propagate và stream được đóng đúng cách. Row lỗi thì em ghi vào bảng `import_errors` và chạy tiếp thay vì dừng cả job, kèm checkpoint để retry được. Nếu cần nhanh hơn nữa, em đẩy từng batch sang queue cho worker xử lý song song.

---

## 25. Worker Threads vs Child Process

Cả hai đều để thoát khỏi giới hạn single-thread, nhưng dùng cho mục đích khác nhau.

| | Worker Threads | Child Process |
|---|---|---|
| Memory | Chung process, chia sẻ được qua `SharedArrayBuffer` | Process riêng, memory tách biệt hoàn toàn |
| Chi phí khởi tạo | Nhẹ hơn (~vài ms) | Nặng hơn (~vài chục ms, cả V8 instance mới) |
| Giao tiếp | `postMessage` (structured clone) + `SharedArrayBuffer` (zero-copy) | IPC / stdio — phải serialize, chậm hơn |
| Cách ly lỗi | Worker crash có thể ảnh hưởng process | Crash hoàn toàn cô lập |
| Hợp cho | **CPU-bound trong JS**: hash, resize ảnh, parse data lớn | **Chạy chương trình khác**: `ffmpeg`, `python`, shell script |

Quy tắc chọn:

* Việc nặng nhưng **vẫn là JavaScript/TypeScript** → `worker_threads`.
* Cần gọi **binary/ngôn ngữ khác** hoặc cần cách ly tuyệt đối → `child_process` (`spawn`).
* Cần tận dụng nhiều CPU core cho **HTTP server** → `cluster` (hoặc đơn giản hơn: chạy nhiều container replica, để orchestrator lo).

```ts
import { Worker } from "node:worker_threads";

function hashPassword(password: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const worker = new Worker("./hash-worker.js", { workerData: { password } });
    worker.on("message", resolve);
    worker.on("error", reject);
  });
}
```

Lưu ý production: **đừng tạo worker mới cho mỗi request** — chi phí khởi tạo sẽ ăn hết lợi ích. Dùng worker pool (`piscina`) với số worker ≈ số CPU core.

Interview answer:

> Worker threads hợp khi task nặng vẫn viết bằng JS và cần chia sẻ memory rẻ; child process hợp khi cần chạy chương trình độc lập hoặc cần cách ly crash. Cả hai đều nên đi kèm pool — tạo mới mỗi request thì overhead khởi tạo còn đắt hơn chính task cần chạy. Nếu task nặng và chờ được thì tôi ưu tiên đẩy hẳn sang queue worker để service API luôn giữ latency thấp.

---

## 26. Câu hỏi phỏng vấn dễ gặp

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

### `process.nextTick` khác `Promise.then` và `setTimeout` thế nào?

`process.nextTick` có queue riêng, được vét trước cả promise microtask. `Promise.then` là microtask, chạy sau khi call stack rỗng nhưng trước macrotask. `setTimeout` là macrotask ở phase timers, chạy sau cùng. Thứ tự: sync → nextTick → promise → timer. `nextTick` đệ quy vô hạn sẽ starve event loop.

### Quên `await` thì chuyện gì xảy ra?

Function trả về ngay lập tức trước khi công việc xong, và nếu Promise đó reject thì thành unhandled rejection chứ không vào `try/catch` của caller. `async` chỉ đảm bảo function trả về Promise, nó không tự chờ Promise bên trong. Bật ESLint rule `no-floating-promises` để bắt lỗi này.

### `Promise.all` reject rồi thì các Promise còn lại có bị hủy không?

Không. `Promise.all` fail fast ở chỗ **ngừng chờ kết quả**, nhưng các Promise còn lại vẫn tiếp tục chạy — request HTTP vẫn bay đi, row vẫn được ghi. JavaScript không có cancel Promise built-in; muốn hủy thật phải dùng `AbortController`.

### Vì sao interface có declaration merging còn type thì không? Khi nào cần?

Interface cùng tên tự merge; `type` cùng tên báo lỗi duplicate identifier. Cần merging khi augment type của library — ví dụ thêm `req.user` vào `Express.Request` sau khi auth middleware chạy.

### Node.js bị memory leak thì debug thế nào?

Xác nhận leak bằng heap used tăng đơn điệu qua nhiều chu kỳ GC (không kết luận từ một lần đo RSS). Sau đó chụp hai heap snapshot cách nhau qua `node --inspect` + Chrome DevTools và diff để tìm object tăng liên tục. Nguyên nhân hay gặp: cache không giới hạn size, listener không remove, `setInterval` không clear, closure giữ buffer lớn.

### Vì sao phải dùng stream thay vì `readFile` cho file lớn?

`readFile` nạp toàn bộ file vào heap — file 10GB thì OOM. Stream xử lý theo chunk nên memory không phụ thuộc kích thước file, và có backpressure để reader tự chậm lại khi writer không theo kịp. Dùng `pipeline()` thay `.pipe()` để error propagate và cleanup tự động trên cả chain. Nguyên tắc: nếu kích thước file do người khác quyết định (user upload) thì luôn dùng stream.

### Import CSV 5GB, 20 triệu record thì làm thế nào?

`createReadStream` + `csv-parser` đọc tuần tự từng row, validate bằng zod, gom batch 500–1000 record rồi `bulkWrite` (Mongo) / `COPY` (Postgres) — không insert từng dòng vì đó là 20 triệu round-trip DB. Backpressure giữ memory phẳng khi DB ghi chậm hơn tốc độ đọc. Bọc bằng `stream.pipeline()`. Row lỗi thì ghi vào `import_errors` và chạy tiếp, kèm checkpoint để retry được.

### Đọc XLSX 500MB bằng `xlsx.readFile()` được không?

Không. XLSX là zip chứa XML nên parse cả workbook thường ngốn RAM gấp nhiều lần kích thước file. Dùng `ExcelJS.stream.xlsx.WorkbookReader` để duyệt worksheet → row, và `WorkbookWriter` (flush từng row) khi export, thay vì `Workbook` giữ hết trong RAM.

### Dùng `.on("data")` rồi `await insert()` bên trong có sao không?

Có — đó là lỗi hay gặp nhất. `.on("data")` không chờ hàm async của bạn, nên reader vẫn bơm data tiếp trong lúc DB đang ghi, backpressure mất tác dụng và RAM phình dần. Dùng `for await (const row of stream)` — nó tự pause stream trong lúc `await`.

### Worker threads hay child process?

Worker threads khi task nặng vẫn là JavaScript và cần chia sẻ memory rẻ (`SharedArrayBuffer`). Child process khi cần chạy chương trình khác (`ffmpeg`, python) hoặc cần cách ly crash tuyệt đối. Cả hai đều nên dùng pool — tạo mới mỗi request thì overhead khởi tạo còn đắt hơn task.

### Ack trước hay ack sau khi xử lý message?

Ack sau. Ack trước mà consumer crash giữa chừng thì mất message vĩnh viễn vì broker đã xóa nó. Ack sau cho at-least-once delivery, đổi lại message có thể được xử lý nhiều lần, nên handler bắt buộc phải idempotent (event id + bảng `processed_events`, hoặc `ON CONFLICT DO NOTHING`).
