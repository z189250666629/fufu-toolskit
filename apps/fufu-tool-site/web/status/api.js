const SERVER_ERROR_MESSAGE = '服务暂时不可用，请稍后重试';

function isServerErrorStatus(status) {
  return Number.isInteger(status) && status >= 500;
}

export async function fetchJson(path, options = {}) {
  const { fetchImpl = globalThis.fetch, ...fetchOptions } = options;
  const headers = {
    ...(fetchOptions.headers || {})
  };
  const response = await fetchImpl(path, { ...fetchOptions, headers, cache: 'no-store' });
  let data = {};
  try {
    data = await response.json();
  } catch {
    if (response.ok) {
      const error = new Error('响应不是有效 JSON');
      error.status = response.status;
      error.data = {};
      throw error;
    }
  }
  if (!response.ok) {
    const serverError = isServerErrorStatus(response.status);
    const error = new Error(
      serverError
        ? SERVER_ERROR_MESSAGE
        : data.error || data.configError || `HTTP ${response.status}`
    );
    error.status = response.status;
    error.data = serverError ? {} : data;
    throw error;
  }
  return data;
}

export function postJson(path, body, options = {}) {
  return fetchJson(path, {
    ...options,
    method: 'POST',
    headers: { ...(options.headers || {}), 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
}
