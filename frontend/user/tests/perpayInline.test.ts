import assert from 'node:assert/strict'
import test from 'node:test'
import { perpayCollectionAmount } from '../src/utils/perpayInline.ts'

const payment = { provider_type: 'epay', interaction_mode: 'qr', perpay_inline: true, currency: 'CNY', payable_amount: '0.10', collection_amount: '0.13' }

test('PerPay shows actual payable amount while preserving order settlement value', () => {
  assert.equal(perpayCollectionAmount(payment), '0.13')
  assert.equal(payment.payable_amount, '0.10')
})

test('other gateways, redirect payments and invalid amounts cannot use PerPay display metadata', () => {
  for (const patch of [
    { provider_type: 'official' }, { interaction_mode: 'redirect' }, { perpay_inline: false }, { currency: 'USD' },
    { collection_amount: '0.09' }, { collection_amount: '0.00' }, { collection_amount: '0.131' }, { collection_amount: 'NaN' },
  ]) assert.equal(perpayCollectionAmount({ ...payment, ...patch }), null)
})
