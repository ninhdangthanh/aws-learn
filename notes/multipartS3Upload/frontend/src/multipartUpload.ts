import { api, type CompletedPart, type PresignedPart } from "./api";

export interface UploadOptions {
  concurrency?: number; // how many parts to upload in parallel
  maxRetries?: number; // per-part retry attempts on failure
  signal?: AbortSignal; // cancel the whole upload
  onProgress?: (loaded: number, total: number) => void;
  onPartStatus?: (partNumber: number, status: PartStatus) => void;
}

export type PartStatus = "pending" | "uploading" | "done" | "error";

interface PartPlan {
  partNumber: number;
  start: number;
  end: number;
}

/**
 * Orchestrates a full S3 multipart upload from the browser:
 * init -> presign -> parallel PUT of each part -> complete.
 * On any fatal error (or an external abort) it best-effort aborts the upload so
 * S3 doesn't keep orphaned parts around.
 */
export async function multipartUpload(file: File, opts: UploadOptions = {}) {
  const concurrency = opts.concurrency ?? 4;
  const maxRetries = opts.maxRetries ?? 3;

  const { key, uploadId, partSize } = await api.init(
    file.name,
    file.size,
    file.type || "application/octet-stream",
  );

  // Slice the file into byte ranges. Part numbers are 1-based per the S3 API.
  const plans: PartPlan[] = [];
  for (let start = 0, n = 1; start < file.size; start += partSize, n++) {
    plans.push({ partNumber: n, start, end: Math.min(start + partSize, file.size) });
  }
  // Empty file: still needs a single (empty) part so complete has something.
  if (plans.length === 0) {
    plans.push({ partNumber: 1, start: 0, end: 0 });
  }

  try {
    // Ask the backend for one presigned URL per part, in a single batch.
    const { parts: presigned } = await api.presignParts(
      key,
      uploadId,
      plans.map((p) => p.partNumber),
    );
    const urlByPart = new Map<number, string>(
      presigned.map((p: PresignedPart) => [p.partNumber, p.url]),
    );

    // Shared progress accounting across all concurrent parts.
    const loadedPerPart = new Array<number>(plans.length).fill(0);
    const total = file.size;
    const emitProgress = () => {
      if (!opts.onProgress) return;
      opts.onProgress(
        loadedPerPart.reduce((a, b) => a + b, 0),
        total,
      );
    };

    const completed: CompletedPart[] = new Array(plans.length);

    // A simple worker pool: workers pull the next index off a shared cursor.
    let cursor = 0;
    const worker = async () => {
      while (true) {
        if (opts.signal?.aborted) throw new DOMException("aborted", "AbortError");
        const idx = cursor++;
        if (idx >= plans.length) return;

        const plan = plans[idx];
        const url = urlByPart.get(plan.partNumber);
        if (!url) throw new Error(`missing presigned URL for part ${plan.partNumber}`);

        opts.onPartStatus?.(plan.partNumber, "uploading");
        const blob = file.slice(plan.start, plan.end);
        const etag = await putPartWithRetry(url, blob, maxRetries, opts.signal, (loaded) => {
          loadedPerPart[idx] = loaded;
          emitProgress();
        });

        loadedPerPart[idx] = plan.end - plan.start;
        emitProgress();
        completed[idx] = { partNumber: plan.partNumber, etag };
        opts.onPartStatus?.(plan.partNumber, "done");
      }
    };

    await Promise.all(Array.from({ length: Math.min(concurrency, plans.length) }, worker));

    const result = await api.complete(key, uploadId, completed);
    return result;
  } catch (err) {
    // Best-effort cleanup so incomplete parts don't linger and accrue cost.
    await api.abort(key, uploadId).catch(() => {});
    throw err;
  }
}

/** PUT a single part with retries + exponential backoff. Returns the ETag. */
async function putPartWithRetry(
  url: string,
  blob: Blob,
  maxRetries: number,
  signal: AbortSignal | undefined,
  onProgress: (loaded: number) => void,
): Promise<string> {
  let attempt = 0;
  while (true) {
    try {
      return await putPart(url, blob, signal, onProgress);
    } catch (err) {
      if (signal?.aborted) throw err;
      attempt++;
      if (attempt > maxRetries) throw err;
      onProgress(0); // reset this part's progress before retrying
      await sleep(300 * 2 ** (attempt - 1));
    }
  }
}

/**
 * Uploads one part via XMLHttpRequest so we get real upload progress events
 * (fetch() can't report request-body upload progress). Reads the ETag from the
 * S3 response header — which requires the bucket CORS to expose "ETag".
 */
function putPart(
  url: string,
  blob: Blob,
  signal: AbortSignal | undefined,
  onProgress: (loaded: number) => void,
): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url, true);

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded);
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        const etag = xhr.getResponseHeader("ETag");
        if (!etag) {
          reject(new Error("missing ETag header — check the bucket CORS ExposeHeaders"));
          return;
        }
        resolve(etag);
      } else {
        reject(new Error(`part upload failed: HTTP ${xhr.status}`));
      }
    };
    xhr.onerror = () => reject(new Error("network error during part upload"));
    xhr.onabort = () => reject(new DOMException("aborted", "AbortError"));

    if (signal) {
      if (signal.aborted) {
        xhr.abort();
        return;
      }
      signal.addEventListener("abort", () => xhr.abort(), { once: true });
    }

    xhr.send(blob);
  });
}

function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}
