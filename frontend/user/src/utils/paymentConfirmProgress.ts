// 「我已完成付款」后的确认进度：网关到账确认时长不可预知，进度按时间渐近逼近上限，
// 不会在确认前走满；确认成功后才到 100%。
export const CONFIRM_PROGRESS_CAP = 95
export const CONFIRM_SLOW_AFTER_MS = 60_000

export function confirmProgressPercent(elapsedMs: number, paid: boolean): number {
  if (paid) return 100
  if (!Number.isFinite(elapsedMs) || elapsedMs <= 0) return 0
  return Math.min(CONFIRM_PROGRESS_CAP, CONFIRM_PROGRESS_CAP * (1 - Math.exp(-elapsedMs / 10_000)))
}
