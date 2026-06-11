export class APIError extends Error {
  status: number;
  data: unknown;

  constructor(message: string, status: number, data: unknown = {}) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.data = data;
  }
}

function safeMessage(error: unknown, fallback = '请求失败，请稍后重试') {
  if (error instanceof APIError) return error.message;
  if (error instanceof Error && error.message) return fallback;
  return fallback;
}

export async function fetchJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    cache: 'no-store',
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.headers ?? {})
    }
  });

  let body: unknown = {};
  try {
    body = await response.json();
  } catch {
    body = {};
  }

  if (!response.ok) {
    const maybeMessage = body && typeof body === 'object' && 'error' in body
      ? String((body as { error?: unknown }).error ?? '')
      : '';
    const message = response.status >= 500
      ? '服务暂时不可用，请稍后重试'
      : maybeMessage || `请求失败（${response.status}）`;
    throw new APIError(message, response.status, response.status >= 500 ? {} : body);
  }

  return body as T;
}

export async function sendJSON<T>(path: string, method: string, payload?: unknown): Promise<T> {
  return fetchJSON<T>(path, {
    method,
    headers: {
      'Content-Type': 'application/json'
    },
    body: payload === undefined ? undefined : JSON.stringify(payload)
  });
}

export function messageFromError(error: unknown, fallback?: string) {
  return safeMessage(error, fallback);
}
