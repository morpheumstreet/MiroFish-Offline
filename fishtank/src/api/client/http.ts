import type { ApiData, ApiEnvelope } from '../schemas/envelope'

export type HttpClientOptions = {
  baseURL: string
  timeoutMs?: number
}

/**
 * API base URL for `fetch`:
 * - Empty string → same origin as the page (`/api/...`). Use when Flask (or any server) serves
 *   the built SPA and the API together — works for any host, port, or reverse proxy.
 * - Set `BUN_PUBLIC_API_BASE_URL` at build time for a fixed API origin (e.g. `bun run dev` on a
 *   separate port from the backend — use `fishtank/.env` from `.env.example`).
 */
function resolveBaseURL(): string {
  const env = import.meta.env
  const explicit = env?.BUN_PUBLIC_API_BASE_URL
  if (explicit != null && String(explicit).trim() !== '') {
    return String(explicit).replace(/\/$/, '')
  }

  const proto =
    typeof globalThis.location?.protocol === 'string' ? globalThis.location.protocol : ''
  if (proto === 'http:' || proto === 'https:') {
    return ''
  }

  return ''
}

function joinURL(base: string, path: string): string {
  const b = base.replace(/\/$/, '') || ''
  const p = path.startsWith('/') ? path : `/${path}`
  if (!b) return p
  return `${b}${p}`
}

/** Absolute URL for logs/errors when base is relative (production same-origin). */
function displayRequestUrl(requestUrl: string): string {
  if (requestUrl.startsWith('http://') || requestUrl.startsWith('https://')) return requestUrl
  if (typeof globalThis.location?.origin === 'string') return `${globalThis.location.origin}${requestUrl}`
  return requestUrl
}

function isTransportFailure(e: unknown): boolean {
  const name = e instanceof Error ? e.name : ''
  const msg = e instanceof Error ? e.message : String(e)
  return (
    name === 'NetworkError' ||
    (name === 'TypeError' && /fetch|network|load failed/i.test(msg)) ||
    /NetworkError|Failed to fetch|Load failed|network error/i.test(msg)
  )
}

function normalizeFetchFailure(e: unknown, requestUrl: string): Error {
  if (!isTransportFailure(e)) return e instanceof Error ? e : new Error(String(e))
  const original = e instanceof Error ? e.message : String(e)
  const target = displayRequestUrl(requestUrl)
  return new Error(
    `${original} — Could not reach the API at ${target}. ` +
      'If the UI is served by Flask/Docker, ensure the backend is running and use the same origin URL (not file://). ' +
      'If you use `bun run dev`, set `BUN_PUBLIC_API_BASE_URL` in fishtank/.env to your API base (see fishtank/.env.example).'
  )
}

function appendQuery(
  path: string,
  query?: Record<string, string | number | boolean | null | undefined>
): string {
  if (!query) return path
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null) continue
    sp.set(k, String(v))
  }
  const q = sp.toString()
  return q ? `${path}?${q}` : path
}

async function readBody(res: Response): Promise<unknown> {
  const text = await res.text()
  if (!text) return {}
  try {
    return JSON.parse(text) as unknown
  } catch {
    throw new Error(text.slice(0, 200) || `HTTP ${res.status}`)
  }
}

function assertEnvelope<T extends ApiData>(json: unknown, httpStatus: number): ApiEnvelope<T> {
  if (typeof json !== 'object' || json === null) {
    throw new Error(`Invalid JSON response (HTTP ${httpStatus})`)
  }
  const o = json as ApiEnvelope<T>
  if (o.success === false) {
    const msg = o.error ?? o.message ?? 'Error'
    console.error('API Error:', msg)
    throw new Error(msg)
  }
  return o
}

/**
 * Minimal typed fetch wrapper: one execution path, envelope unwrap, timeout, logging on transport errors.
 */
export class HttpClient {
  readonly baseURL: string
  private readonly timeoutMs: number

  constructor(options?: Partial<HttpClientOptions> & { baseURL?: string }) {
    this.baseURL = (options?.baseURL ?? resolveBaseURL()).replace(/\/$/, '')
    this.timeoutMs = options?.timeoutMs ?? 300_000
  }

  private async request(path: string, init: RequestInit): Promise<Response> {
    const url = joinURL(this.baseURL, path)
    const controller = new AbortController()
    const t = setTimeout(() => controller.abort(), this.timeoutMs)
    try {
      const res = await fetch(url, { ...init, signal: controller.signal })
      return res
    } catch (e) {
      if ((e as Error)?.name === 'AbortError') {
        console.error('Request timeout')
        throw new Error('Request timeout')
      }
      console.error('Response error:', e)
      throw normalizeFetchFailure(e, url)
    } finally {
      clearTimeout(t)
    }
  }

  async get<T extends ApiData = ApiData>(
    path: string,
    query?: Record<string, string | number | boolean | null | undefined>
  ): Promise<ApiEnvelope<T>> {
    const p = appendQuery(path, query)
    const res = await this.request(p, { method: 'GET' })
    const json = await readBody(res)
    if (!res.ok) {
      const env = typeof json === 'object' && json !== null ? (json as ApiEnvelope<T>) : null
      const msg = env?.error ?? env?.message ?? `HTTP ${res.status}`
      console.error('API Error:', msg)
      throw new Error(msg)
    }
    return assertEnvelope<T>(json, res.status)
  }

  async post<T extends ApiData = ApiData>(path: string, body?: unknown): Promise<ApiEnvelope<T>> {
    const res = await this.request(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body !== undefined ? JSON.stringify(body) : undefined
    })
    const json = await readBody(res)
    if (!res.ok) {
      const env = typeof json === 'object' && json !== null ? (json as ApiEnvelope<T>) : null
      const msg = env?.error ?? env?.message ?? `HTTP ${res.status}`
      console.error('API Error:', msg)
      throw new Error(msg)
    }
    return assertEnvelope<T>(json, res.status)
  }

  /** multipart/form-data; do not set Content-Type (boundary is set by the runtime). */
  async postForm<T extends ApiData = ApiData>(path: string, formData: FormData): Promise<ApiEnvelope<T>> {
    const res = await this.request(path, { method: 'POST', body: formData })
    const json = await readBody(res)
    if (!res.ok) {
      const env = typeof json === 'object' && json !== null ? (json as ApiEnvelope<T>) : null
      const msg = env?.error ?? env?.message ?? `HTTP ${res.status}`
      console.error('API Error:', msg)
      throw new Error(msg)
    }
    return assertEnvelope<T>(json, res.status)
  }
}

export function defaultBaseURL(): string {
  return resolveBaseURL()
}
