<template>
  <footer
    class="relative bg-card/90 text-muted-foreground border-t overflow-hidden">
    <div class="container mx-auto px-4 py-6 sm:py-8">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-col gap-2">
          <router-link to="/" class="inline-flex w-fit items-center gap-2 font-bold text-foreground">
            <img v-if="brandLogo" :src="brandLogo" alt="" class="h-7 max-w-28 object-contain" />
            <span v-else aria-hidden="true"
              class="flex size-7 items-center justify-center rounded-md bg-primary text-xs font-bold text-primary-foreground">
              {{ brandInitial }}
            </span>
            {{ brandSiteName }}
          </router-link>
          <p class="max-w-sm text-sm leading-relaxed text-muted-foreground">
            {{ brandDescription || t('footer.description') }}
          </p>
        </div>

        <nav :aria-label="t('footer.quickLinks')" class="flex flex-wrap gap-x-5 gap-y-2 text-sm">
          <router-link
            v-for="item in quickLinks"
            :key="item.path"
            :to="item.path"
            class="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground"
          >
            <component :is="item.icon" class="size-4 shrink-0 opacity-70" aria-hidden="true" />
            {{ t(item.label) }}
          </router-link>
        </nav>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Home, LayoutGrid } from 'lucide-vue-next'
import { useAppStore } from '../stores/app'
import { getImageUrl } from '../utils/image'

const { t } = useI18n()
const appStore = useAppStore()

const config = computed(() => appStore.config)

const brandSiteName = computed(() => {
  const siteName = config.value?.brand?.site_name
  return typeof siteName === 'string' && siteName.trim() ? siteName.trim() : 'GPT 速充'
})

const brandDescription = computed(() => {
  const desc = config.value?.brand?.site_description
  if (desc && typeof desc === 'object') {
    const val = desc[appStore.locale] || desc['zh-CN'] || ''
    return typeof val === 'string' ? val.trim() : ''
  }
  return ''
})

const brandInitial = computed(() => {
  return brandSiteName.value.charAt(0).toUpperCase()
})

const brandLogo = computed(() => {
  const raw = String(config.value?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : '/dj.svg'
})

const isListMode = computed(() => config.value?.template_mode === 'list')

const quickLinks = computed(() => {
  const items = [
    { path: '/', label: 'nav.home', icon: Home },
  ]
  if (!isListMode.value) {
    items.push({ path: '/products', label: 'nav.products', icon: LayoutGrid })
  }
  return items
})

</script>
