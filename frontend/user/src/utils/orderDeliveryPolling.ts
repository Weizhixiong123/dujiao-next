// 订单已付款但尚未交付完成时（如第三方 API 异步采购发货），详情页需自动刷新，
// 否则用户必须手动刷新才能看到卡密。轮询频率随等待时间递减，超时后停止。
const AWAITING_DELIVERY_STATUSES = new Set(['paid', 'fulfilling', 'partially_delivered'])

export function isAwaitingDelivery(order: any): boolean {
  if (!order) return false
  if (AWAITING_DELIVERY_STATUSES.has(String(order.status || ''))) return true
  const children = Array.isArray(order.children) ? order.children : []
  return children.some((child: any) => AWAITING_DELIVERY_STATUSES.has(String(child?.status || '')))
}

export function nextDeliveryPollDelay(elapsedMs: number): number | null {
  if (!Number.isFinite(elapsedMs) || elapsedMs < 0) return 3_000
  if (elapsedMs < 60_000) return 3_000
  if (elapsedMs < 5 * 60_000) return 10_000
  if (elapsedMs < 30 * 60_000) return 30_000
  return null
}
