import { average } from './utils.js';

export function summarizeProbeAttempts(attempts) {
  const okAttempts = attempts.filter((item) => item.ok);
  return {
    ok: okAttempts.length > 0,
    successRate: attempts.length ? okAttempts.length / attempts.length : 0,
    averageMs: average(okAttempts.map((item) => item.ms)),
    lastError: attempts.at(-1)?.error || ''
  };
}
