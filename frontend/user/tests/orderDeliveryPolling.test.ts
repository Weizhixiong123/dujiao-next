import assert from 'node:assert/strict'
import test from 'node:test'
import { isAwaitingDelivery, nextDeliveryPollDelay } from '../src/utils/orderDeliveryPolling.ts'

test('orders still being delivered keep refreshing, finished or unpaid orders do not', () => {
  for (const status of ['paid', 'fulfilling', 'partially_delivered']) {
    assert.equal(isAwaitingDelivery({ status }), true)
  }
  for (const status of ['pending_payment', 'delivered', 'completed', 'canceled', 'refunded', 'partially_refunded']) {
    assert.equal(isAwaitingDelivery({ status }), false)
  }
  assert.equal(isAwaitingDelivery(null), false)
  assert.equal(isAwaitingDelivery({ status: 'completed', children: [{ status: 'delivered' }, { status: 'fulfilling' }] }), true)
  assert.equal(isAwaitingDelivery({ status: 'completed', children: [{ status: 'delivered' }] }), false)
})

test('delivery polling slows down over time and eventually stops', () => {
  assert.equal(nextDeliveryPollDelay(0), 3_000)
  assert.equal(nextDeliveryPollDelay(Number.NaN), 3_000)
  assert.equal(nextDeliveryPollDelay(2 * 60_000), 10_000)
  assert.equal(nextDeliveryPollDelay(10 * 60_000), 30_000)
  assert.equal(nextDeliveryPollDelay(31 * 60_000), null)
})
