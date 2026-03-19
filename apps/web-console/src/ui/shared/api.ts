export type ApiErrorLikeBody = {
  message?: unknown;
  error?: unknown;
  detail?: unknown;
  details?: unknown;
  [key: string]: unknown;
};

export class ApiError extends Error {
  readonly status: number;
  readonly statusText: string;
  readonly url: string;
  readonly body: unknown;

  constructor(args: {
    message: string;
    status: number;
    statusText: string;
    url: string;
    body: unknown;
  }) {
    super(args.message);
    this.name = "ApiError";
    this.status = args.status;
    this.statusText = args.statusText;
    this.url = args.url;
    this.body = args.body;
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

async function parseResponseBody(resp: Response): Promise<unknown> {
  if (resp.status === 204 || resp.status === 205) return null;

  const contentType = resp.headers.get("Content-Type") ?? "";
  const isJson = contentType.toLowerCase().includes("application/json");

  if (isJson) {
    try {
      return await resp.json();
    } catch {
      return null;
    }
  }

  try {
    const text = await resp.text();
    return text.length > 0 ? text : null;
  } catch {
    return null;
  }
}

function extractApiErrorMessage(
  body: unknown,
  status: number,
  statusText: string,
): string {
  if (typeof body === "string" && body.trim()) {
    return body.trim();
  }

  if (isObject(body)) {
    const candidateKeys = ["message", "error", "detail", "details"] as const;

    for (const key of candidateKeys) {
      const value = body[key];
      if (typeof value === "string" && value.trim()) {
        return value.trim();
      }
    }
  }

  if (statusText.trim()) {
    return `${status} ${statusText}`;
  }

  return `Request failed (${status})`;
}

export async function apiFetch<T = unknown>(
  url: string,
  init: RequestInit = {},
): Promise<T> {
  const resp = await fetch(url, init);
  const body = await parseResponseBody(resp);

  if (!resp.ok) {
    throw new ApiError({
      message: extractApiErrorMessage(body, resp.status, resp.statusText),
      status: resp.status,
      statusText: resp.statusText,
      url,
      body,
    });
  }

  return body as T;
}

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error && error.message) return error.message;
  if (typeof error === "string" && error.trim()) return error.trim();
  return "Unknown error";
}
