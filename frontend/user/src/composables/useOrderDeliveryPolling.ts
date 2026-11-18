import { onUnmounted, watch, type Ref } from 'vue'
import { isAwaitingDelivery, nextDeliveryPollDelay } from '../utils/orderDeliveryPolling'

/**
 * 订单详情页：已付款但未交付完成时自动静默刷新，交付完成或离开页面后停止。
 * 页面在后台时跳过请求，回到前台立即刷新一次。
 */
export function useOrderDeliveryPolling(order: Ref<any>, reload: () => Promise<void>) {
  let timer: number | null = null
  let startedAt = 0
  let inFlight = false

  const clear = () => {
    if (timer !== null) {
      window.clearTimeout(timer)
      timer = null
    }
  }

  const tick = async () => {
    timer = null
    if (!isAwaitingDelivery(order.value)) return
    if (!document.hidden && !inFlight) {
      inFlight = true
      try {
        await reload()
      } finally {
        inFlight = false
      }
    }
    schedule()
  }

  const schedule = () => {
    clear()
    if (!isAwaitingDelivery(order.value)) {
      startedAt = 0
      return
    }
    if (!startedAt) startedAt = Date.now()
    const delay = nextDeliveryPollDelay(Date.now() - startedAt)
    if (delay === null) return
    timer = window.setTimeout(() => { void tick() }, delay)
  }

  const onVisibilityChange = () => {
    if (document.hidden || timer === null || inFlight) return
    clear()
    void tick()
  }

  watch(() => isAwaitingDelivery(order.value), () => {
    if (timer === null && !inFlight) schedule()
  }, { immediate: true })

  document.addEventListener('visibilitychange', onVisibilityChange)

  onUnmounted(() => {
    clear()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })
}
