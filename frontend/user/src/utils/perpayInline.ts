import { amountToCents } from './money.ts'

export function perpayCollectionAmount(payment: any): string | null {
  if (payment?.provider_type !== 'epay' || payment?.interaction_mode !== 'qr' || payment?.perpay_inline !== true || payment?.currency !== 'CNY') return null
  const value = payment.collection_amount
  if (typeof value !== 'string' || !/^\d+\.\d{2}$/.test(value)) return null
  const cents = amountToCents(value)
  const original = amountToCents(payment.payable_amount)
  return cents !== null && cents > 0 && original !== null && cents >= original ? value : null
}
