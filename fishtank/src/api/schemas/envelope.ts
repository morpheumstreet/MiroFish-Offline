/**
 * Shared contract with the Flask JSON API (`jsonify({ success, data?, error? })`).
 * Default `T` is a loose object map so UI code can read known keys without noise; specialize `T` per endpoint for real safety.
 */
export type ApiData = Record<string, any>

export type ApiEnvelope<T extends ApiData = ApiData> = {
  success: boolean
  data?: T
  error?: string
  message?: string
  traceback?: string
}

export function isApiFailure(e: ApiEnvelope): e is ApiEnvelope & { success: false } {
  return e.success === false
}
