<template>
  <div class="w-full rounded-xl border bg-card p-4 text-left">
    <template v-if="!confirming">
      <p class="text-xs text-muted-foreground">{{ t('payment.confirmProgress.idleHint') }}</p>
      <Button type="button" class="mt-3 w-full font-semibold" @click="emit('confirm')">
        <CircleCheck class="h-4 w-4" aria-hidden="true" />
        {{ t('payment.confirmProgress.action') }}
      </Button>
    </template>

    <template v-else>
      <div class="flex items-center gap-2 text-sm font-semibold text-foreground" aria-live="polite">
        <CircleCheck v-if="paid" class="h-4 w-4 text-success" aria-hidden="true" />
        <Loader2 v-else class="h-4 w-4 animate-spin text-primary" aria-hidden="true" />
        <span>{{ paid ? t('payment.confirmProgress.paidTitle') : t('payment.confirmProgress.checkingTitle') }}</span>
      </div>

      <div
        class="relative mt-3 h-2 w-full overflow-hidden rounded-full bg-secondary"
        role="progressbar"
        :aria-valuenow="Math.round(percent)"
        aria-valuemin="0"
        aria-valuemax="100"
      >
        <div
          class="relative h-full overflow-hidden rounded-full transition-[width] duration-500 ease-out"
          :class="paid ? 'bg-success' : 'bg-primary'"
          :style="{ width: `${percent}%` }"
        >
          <div v-if="!paid" class="confirm-shimmer absolute inset-0"></div>
        </div>
      </div>

      <ol class="mt-4 grid grid-cols-3 gap-2 text-center text-xs">
        <li v-for="(step, idx) in steps" :key="step.key" class="flex flex-col items-center gap-1.5">
          <span
            class="flex h-6 w-6 items-center justify-center rounded-full border transition-colors"
            :class="stepState(idx) === 'done'
              ? 'border-success bg-success text-white'
              : stepState(idx) === 'active'
                ? 'border-primary text-primary'
                : 'text-muted-foreground'"
          >
            <Check v-if="stepState(idx) === 'done'" class="h-3.5 w-3.5" aria-hidden="true" />
            <Loader2 v-else-if="stepState(idx) === 'active'" class="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
            <span v-else>{{ idx + 1 }}</span>
          </span>
          <span :class="stepState(idx) === 'pending' ? 'text-muted-foreground' : 'text-foreground'">{{ step.label }}</span>
        </li>
      </ol>

      <p class="mt-3 text-xs text-muted-foreground">
        {{ paid ? t('payment.confirmProgress.paidHint') : slow ? t('payment.confirmProgress.slowHint') : t('payment.confirmProgress.checkingHint') }}
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, CircleCheck, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { CONFIRM_SLOW_AFTER_MS, confirmProgressPercent } from '../../utils/paymentConfirmProgress'

const props = defineProps<{
  confirming: boolean
  paid: boolean
}>()

const emit = defineEmits<{ confirm: [] }>()

const { t } = useI18n()

const startedAt = ref(0)
const elapsed = ref(0)
let timer: number | null = null

const stopTimer = () => {
  if (timer !== null) {
    window.clearInterval(timer)
    timer = null
  }
}

watch(
  () => props.confirming,
  (confirming) => {
    stopTimer()
    elapsed.value = 0
    if (!confirming) return
    startedAt.value = Date.now()
    timer = window.setInterval(() => {
      elapsed.value = Date.now() - startedAt.value
    }, 250)
  },
  { immediate: true },
)

watch(() => props.paid, (paid) => { if (paid) stopTimer() })

onBeforeUnmount(stopTimer)

const percent = computed(() => confirmProgressPercent(elapsed.value, props.paid))
const slow = computed(() => !props.paid && elapsed.value >= CONFIRM_SLOW_AFTER_MS)

const steps = computed(() => [
  { key: 'paid', label: t('payment.confirmProgress.stepPaid') },
  { key: 'confirm', label: t('payment.confirmProgress.stepConfirm') },
  { key: 'deliver', label: t('payment.confirmProgress.stepDeliver') },
])

const stepState = (idx: number): 'done' | 'active' | 'pending' => {
  if (props.paid) return idx <= 1 ? 'done' : 'active'
  if (idx === 0) return 'done'
  return idx === 1 ? 'active' : 'pending'
}
</script>

<style scoped>
.confirm-shimmer {
  background: linear-gradient(90deg, transparent 0%, rgb(255 255 255 / 0.45) 50%, transparent 100%);
  transform: translateX(-100%);
  animation: confirm-shimmer 1.4s ease-in-out infinite;
}

@keyframes confirm-shimmer {
  to {
    transform: translateX(100%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .confirm-shimmer {
    animation: none;
  }
}
</style>
