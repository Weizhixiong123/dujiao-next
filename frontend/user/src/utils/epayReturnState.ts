// This state restores navigation context only. Payment status comes from the
// authenticated order APIs and the server's verified gateway callback.
const allowedKeys = new Set(['biz_type', 'order_no', 'recharge_no', 'guest', 'epay_return', 'session_id'])

export function decodeEpayReturnState(value: unknown): Record<string, string> {
  if (typeof value !== 'string' || value.length > 8192 || !/^[A-Za-z0-9_-]+$/.test(value)) return {}
  try {
    const query = new URLSearchParams(atob(value.replace(/-/g, '+').replace(/_/g, '/')))
    const result: Record<string, string> = {}
    for (const [key, item] of query) {
      if (allowedKeys.has(key)) result[key] = item
    }
    return result
  } catch {
    return {}
  }
}
