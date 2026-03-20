export async function withRetry<T>(
  fn: () => Promise<T>,
  maxRetries = 3,
  delayMs = 1000
): Promise<T> {
  let last: unknown
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn()
    } catch (e) {
      last = e
      if (i === maxRetries - 1) break
      console.warn(`Request failed, retrying (${i + 1}/${maxRetries})...`)
      await new Promise((r) => setTimeout(r, delayMs * 2 ** i))
    }
  }
  throw last instanceof Error ? last : new Error(String(last))
}
