(function(root) {
  const JSON_CONTENT_TYPE = /(^|[/+])json($|;)/i;

  class SafeApiError extends Error {
    constructor(message) {
      super(message);
      this.name = 'SafeApiError';
      this.safeForUser = true;
    }
  }

  function contentType(response) {
    return String(response?.headers?.get?.('content-type') || '');
  }

  function isJsonResponse(response) {
    return JSON_CONTENT_TYPE.test(contentType(response));
  }

  async function readResponseJson(response, fallbackMessage) {
    if (!isJsonResponse(response)) {
      throw new SafeApiError(fallbackMessage);
    }
    try {
      return await response.json();
    } catch {
      throw new SafeApiError(fallbackMessage);
    }
  }

  async function readApiJson(response, options = {}) {
    const serverErrorMessage = options.serverErrorMessage || '请求失败，请稍后重试';
    const clientErrorMessage = options.clientErrorMessage || serverErrorMessage;
    const data = await readResponseJson(response, serverErrorMessage);
    if (response.ok) {
      return data;
    }
    if (response.status >= 500) {
      throw new SafeApiError(serverErrorMessage);
    }
    const message = String(data?.error || '').trim() || clientErrorMessage;
    throw new SafeApiError(message);
  }

  function safeErrorMessage(error, fallbackMessage) {
    if (error?.safeForUser && error.message) {
      return error.message;
    }
    return fallbackMessage;
  }

  root.activityApi = {
    readApiJson,
    safeErrorMessage,
  };
})(globalThis);
