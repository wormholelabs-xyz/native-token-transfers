// HTTP with a timeout and a response size cap. All Canton and disclosure-service calls use it.
export const HTTP_REQUEST_TIMEOUT_MS = 15_000;
export const HTTP_RESPONSE_MAX_BYTES = 4 * 1024 * 1024;

export interface HttpBounds {
  timeoutMs: number;
  maxBytes: number;
}

export interface BoundedJsonResponse {
  status: number;
  text: string;
  json: unknown;
}

export const DEFAULT_HTTP_BOUNDS: HttpBounds = {
  timeoutMs: HTTP_REQUEST_TIMEOUT_MS,
  maxBytes: HTTP_RESPONSE_MAX_BYTES,
};

async function readBodyWithCap(
  response: Response,
  maxBytes: number
): Promise<string> {
  if (!response.body) return "";
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let receivedBytes = 0;
  // Ends when the stream ends or the cap is passed.
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    receivedBytes += value.length;
    if (receivedBytes > maxBytes) {
      await reader.cancel();
      throw new Error(`response body is larger than ${maxBytes} bytes`);
    }
    chunks.push(value);
  }
  return Buffer.concat(chunks).toString("utf8");
}

/** fetch with AbortSignal.timeout and a body cap. `json` is undefined for an empty or non-JSON body. */
export async function fetchJsonBounded(
  url: string,
  init: RequestInit,
  bounds: HttpBounds = DEFAULT_HTTP_BOUNDS
): Promise<BoundedJsonResponse> {
  if (bounds.timeoutMs <= 0 || bounds.maxBytes <= 0)
    throw new Error("HTTP bounds must be positive");
  const timeoutSignal = AbortSignal.timeout(bounds.timeoutMs);
  try {
    const response = await fetch(url, {
      ...init,
      signal: timeoutSignal,
      redirect: "error",
    });
    const declaredLength = response.headers.get("content-length");
    if (declaredLength !== null && Number(declaredLength) > bounds.maxBytes) {
      await response.body?.cancel();
      throw new Error(
        `Content-Length ${declaredLength} is above the ${bounds.maxBytes}-byte cap`
      );
    }
    const text = await readBodyWithCap(response, bounds.maxBytes);
    let json: unknown;
    try {
      json = text.length > 0 ? JSON.parse(text) : undefined;
    } catch {
      json = undefined;
    }
    return { status: response.status, text, json };
  } catch (err) {
    if (timeoutSignal.aborted) {
      throw new Error(
        `${init.method ?? "GET"} ${new URL(url).origin}${new URL(url).pathname} timed out after ${bounds.timeoutMs} ms`
      );
    }
    throw err;
  }
}

/** First 200 characters of an error body for a message. */
export function errorBodyExcerpt(text: string): string {
  return text.length > 200 ? `${text.slice(0, 200)}...` : text;
}
