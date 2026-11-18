<template>
  <Popover v-if="qrCodeUrl">
    <PopoverTrigger as-child>
      <Button
        type="button"
        variant="ghost"
        class="pointer-events-auto h-11 rounded-full bg-zinc-900 px-4 font-semibold text-white shadow-lg hover:bg-zinc-800 hover:text-white"
      >
        <MessageCircle aria-hidden="true" />
        {{ t('shopContact.title') }}
      </Button>
    </PopoverTrigger>
    <PopoverContent side="top" align="end" :side-offset="10" :aria-label="t('shopContact.title')"
      class="w-64 max-w-[calc(100vw-2rem)] rounded-2xl p-4 text-center">
      <p class="text-sm font-semibold">{{ t('shopContact.scan') }}</p>
      <img v-if="!imageFailed" :src="qrCodeUrl" :alt="t('shopContact.qrCode')"
        class="mt-3 w-full rounded-lg bg-white object-contain" @error="imageFailed = true" />
      <p v-else class="mt-3 text-sm text-muted-foreground" role="status">{{ t('shopContact.unavailable') }}</p>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessageCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useAppStore } from '../stores/app'
import { getImageUrl } from '../utils/image'

const { t } = useI18n()
const app = useAppStore()
const qrCodeUrl = computed(() => {
  if (app.isResellerTenant) return ''
  const path = app.config?.contact?.wechat_qr_code
  return typeof path === 'string' && path.startsWith('/uploads/') ? getImageUrl(path) : ''
})
const imageFailed = ref(false)
watch(qrCodeUrl, () => { imageFailed.value = false })
</script>
