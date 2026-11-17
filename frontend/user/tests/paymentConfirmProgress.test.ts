import assert from 'node:assert/strict'
import test from 'node:test'
import { CONFIRM_PROGRESS_CAP, confirmProgressPercent } from '../src/utils/paymentConfirmProgress.ts'

test('confirm progress grows over time but never completes before payment is confirmed', () => {
  assert.equal(confirmProgressPercent(0, false), 0)
  assert.equal(confirmProgressPercent(-1, false), 0)
  assert.equal(confirmProgressPercent(Number.NaN, false), 0)
  const at5s = confirmProgressPercent(5_000, false)
  const at20s = confirmProgressPercent(20_000, false)
  assert.ok(at5s > 20 && at5s < at20s)
  assert.ok(confirmProgressPercent(10 * 60_000, false) <= CONFIRM_PROGRESS_CAP)
})

test('confirm progress completes only once payment is confirmed', () => {
  assert.equal(confirmProgressPercent(0, true), 100)
  assert.equal(confirmProgressPercent(3_000, true), 100)
})
