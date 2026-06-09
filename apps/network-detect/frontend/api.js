export async function fetchJson(path, options = {}) {
  const { fetchImpl = globalThis.fetch, ...fetchOptions } = options;
  const headers = {
    ...(fetchOptions.headers || {})
  };
  const response = await fetchImpl(path, { ...fetchOptions, headers, cache: 'no-store' });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(data.error || data.configError || `HTTP ${response.status}`);
    error.status = response.status;
    error.data = data;
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
