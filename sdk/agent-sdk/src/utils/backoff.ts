export function computeBackoffMs(attempt: number, baseMs: number, maxMs: number, jitter: boolean): number {
  const exponential = Math.min(maxMs, baseMs * 2 ** Math.max(0, attempt-1));
  if (!jitter) {
    return exponential;
  }
  const spread = Math.max(1, Math.floor(exponential * 0.2));
  return exponential - spread + Math.floor(Math.random() * spread * 2);
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
