<template>
  <div class="bg-secondary border rounded-2xl p-4">
    <div class="text-xs uppercase tracking-wider text-muted-foreground">{{ t('payment.payableAmountLabel') }}</div>
    <div class="mt-1 text-2xl font-bold text-foreground">{{ payableAmountDisplay }}</div>
    <div v-if="perpayExactAmount" class="mt-3 flex gap-2 rounded-xl border border-warning/50 bg-warning/10 p-3 text-sm text-warning">
      <AlertTriangle class="mt-0.5 h-4 w-4 flex-none" aria-hidden="true" />
      <div class="space-y-1">
        <p class="font-semibold">{{ t('payment.perpayExactAmountWarning', { amount: `¥${perpayExactAmount}` }) }}</p>
        <p class="text-xs">{{ t('payment.perpayExactAmountReason') }}</p>
      </div>
    </div>
    <p v-else-if="paymentResult.perpay_inline" class="mt-2 text-sm text-warning">{{ t('payment.perpayExactAmountTip') }}</p>
    <div class="mt-4 space-y-2 text-xs">
      <div class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ perpayExactAmount ? t('payment.orderAmountLabel') : t('orderDetail.amountTotal') }}</span>
        <span class="font-semibold text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</span>
      </div>
      <div v-if="hasDiscountAmount(order.discount_amount)" class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</span>
        <span class="font-medium text-destructive">{{ formatDiscountMoney(order.discount_amount, order.currency) }}</span>
      </div>
      <div v-if="hasDiscountAmount(order.promotion_discount_amount)" class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ t('orderDetail.promotionDiscountLabel') }}</span>
        <span class="font-medium text-destructive">{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</span>
      </div>
      <div v-if="hasDiscountAmount(order.wholesale_discount_amount)" class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ t('orderDetail.amountWholesaleDiscount') }}</span>
        <span class="font-medium text-success">{{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}</span>
      </div>
      <div v-if="hasDiscountAmount(order.member_discount_amount)" class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ t('orderDetail.amountMemberDiscount') }}</span>
        <span class="font-medium text-warning">{{ formatDiscountMoney(order.member_discount_amount, order.currency) }}</span>
      </div>
      <div v-if="customerFeeApplied" class="flex items-center justify-between gap-4 text-warning">
        <span>{{ t('payment.feeAmountLabel') }}</span>
        <span class="font-semibold">{{ customerFeeAmountDisplay }}</span>
      </div>
      <div v-if="paymentResult.wallet_paid_amount !== undefined" class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ t('payment.walletDeductLabel') }}</span>
        <span class="font-medium text-foreground">{{ walletPaidDisplay }}</span>
      </div>
      <div v-if="paymentResult.online_pay_amount !== undefined" class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ t('payment.onlinePayLabel') }}</span>
        <span class="font-medium text-foreground">{{ onlinePayDisplay }}</span>
      </div>
    </div>
    <div v-if="showCountdown || pollingActive" class="mt-4 border-t pt-3 text-xs">
      <div v-if="showCountdown" class="flex items-center justify-between gap-4">
        <span class="text-muted-foreground">{{ t('payment.countdownLabel') }}</span>
        <span class="font-mono font-medium text-foreground">{{ countdownText }}</span>
      </div>
      <div v-if="pollingActive" class="mt-2 text-muted-foreground">{{ t('payment.pollingHint') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle } from 'lucide-vue-next'
import { perpayCollectionAmount } from '../../utils/perpayInline'

const props = defineProps<{
  order: any
  paymentResult: any
  customerFeeApplied: boolean
  customerFeeAmountDisplay: string
  payableAmountDisplay: string
  walletPaidDisplay: string
  onlinePayDisplay: string
  showCountdown: boolean
  countdownText: string
  pollingActive: boolean
  formatMoney: (amount?: string, currency?: string) => string
  formatDiscountMoney: (amount?: string, currency?: string) => string
  hasDiscountAmount: (amount?: string) => boolean
}>()

const { t } = useI18n()
const perpayExactAmount = computed(() => perpayCollectionAmount(props.paymentResult))
</script>
