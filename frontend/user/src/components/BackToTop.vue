<template>
  <Transition
    enter-active-class="transition duration-300 ease-out"
    enter-from-class="opacity-0 scale-90"
    enter-to-class="opacity-100 scale-100"
    leave-active-class="transition duration-200 ease-in"
    leave-from-class="opacity-100 scale-100"
    leave-to-class="opacity-0 scale-90"
  >
    <Button
      v-if="visible"
      variant="secondary"
      size="icon"
      @click="scrollToTop"
      class="pointer-events-auto size-11 rounded-full border shadow-lg transition-all hover:shadow-xl hover:-translate-y-0.5 [&_svg]:size-5"
      :aria-label="t('common.backToTop')"
    >
      <ChevronUp />
    </Button>
  </Transition>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronUp } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const { t } = useI18n()
const visible = ref(false)

const onScroll = () => {
  visible.value = window.scrollY > 400
}

const scrollToTop = () => {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

onMounted(() => window.addEventListener('scroll', onScroll, { passive: true }))
onUnmounted(() => window.removeEventListener('scroll', onScroll))
</script>
