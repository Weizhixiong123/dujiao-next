import assert from 'node:assert/strict'
import test from 'node:test'
import { decodeEpayReturnState } from '../src/utils/epayReturnState.ts'

const pack = (query: string) => Buffer.from(query).toString('base64url')

test('epay return restores member, guest and recharge navigation context', () => {
  for (const context of [
    { biz_type: 'order', order_no: 'DJP-123', epay_return: '1' },
    { biz_type: 'order', order_no: 'GUEST-123', guest: '1', epay_return: '1' },
    { biz_type: 'recharge', recharge_no: 'WR123', session_id: 'session-123', epay_return: '1' },
  ]) {
    assert.deepEqual(decodeEpayReturnState(pack(new URLSearchParams(context).toString())), context)
  }
})

test('epay return state cannot supply payment status, credentials or navigation targets', () => {
  const state = pack('order_no=DJP123&status=paid&token=secret&redirect=https%3A%2F%2Fevil.example&__proto__=polluted')
  assert.deepEqual(decodeEpayReturnState(state), { order_no: 'DJP123' })
})

test('malformed, repeated and oversized epay state is ignored', () => {
  for (const value of [undefined, null, [], ['abc', 'def'], '!', 'a', 'A'.repeat(8193)]) {
    assert.deepEqual(decodeEpayReturnState(value), {})
  }
})
