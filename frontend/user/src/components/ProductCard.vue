<template>
  <Card
    class="group relative overflow-hidden flex flex-col h-full w-full max-w-48 rounded-2xl transition-all theme-slide-up"
    :class="isSoldOut(product)
      ? 'cursor-default border-destructive/30'
      : 'cursor-pointer hover:-translate-y-1 hover:border-primary/30 hover:shadow-lg'"
    :style="{ animationDelay: `${index * animationStep}ms` }"
    @click="$emit('click', product.slug)">
    <!-- Image Area -->
    <div class="overflow-hidden bg-muted relative shrink-0">
      <img v-if="displayImageSrc && !imageErrored" :src="displayImageSrc"
        :alt="getLocalizedText(product.title)" loading="lazy" decoding="async"
        class="block h-auto w-full transition-[filter]"
        :class="[
          displayImageSrc === productImageSrc ? 'aspect-[auto_1/1]' : 'aspect-square object-contain',
          isSoldOut(product) ? 'grayscale opacity-70' : '',
        ]"
        @error="handleImageError" />
      <div v-else class="w-full aspect-square flex items-center justify-center text-muted-foreground" role="img"
        :aria-label="getLocalizedText(product.title)">
        <ImageIcon class="w-8 h-8 md:w-12 md:h-12" :stroke-width="1.5" aria-hidden="true" />
      </div>

      <!-- Tags -->
      <div v-if="!isSoldOut(product) && product.tags && product.tags.length > 0"
        class="absolute top-2 right-2 md:top-4 md:right-4 z-20 flex flex-wrap gap-1 md:gap-2 justify-end">
        <span v-for="(tag, tagIndex) in product.tags.slice(0, maxTags)" :key="tagIndex"
          class="inline-flex items-center rounded-md border border-white/25 bg-black/55 px-2 md:px-3 py-0.5 md:py-1 text-xs font-medium text-white backdrop-blur-sm">
          {{ tag }}
        </span>
      </div>
    </div>

    <!-- Content Area -->
    <div class="relative z-20 flex flex-1 flex-col gap-2 p-3">
      <div class="flex items-center justify-between gap-1.5"
        :class="{ 'hidden md:flex': !product.category?.name && !isSoldOut(product) }">
        <span class="min-w-0 truncate text-xs font-medium text-muted-foreground">
          {{ getLocalizedText(product.category?.name) }}
        </span>
        <Badge
          class="shrink-0"
          :class="{ 'hidden md:inline-flex': !isSoldOut(product) }"
          size="xs"
          :variant="isSoldOut(product) ? 'danger' : getStockBadgeVariant(product.stock_status)"
        >
          {{ isSoldOut(product) ? t('products.stockStatus.outOfStock') : getStockStatusLabel(product) }}
        </Badge>
      </div>
      <h3 class="line-clamp-2 text-sm font-bold leading-snug text-foreground md:text-base">
        {{ getLocalizedText(product.title) }}
      </h3>

      <!-- Badges -->
      <div class="flex flex-wrap items-center gap-1">
        <Badge
          class="hidden md:inline-flex"
          size="xs"
          :variant="product.purchase_type === 'guest' ? 'warning' : 'success'"
        >
          <UserPlus v-if="product.purchase_type === 'guest'" class="h-3 w-3" />
          <Lock v-else class="h-3 w-3" />
          {{ getPurchaseTypeLabel(product.purchase_type) }}
        </Badge>

        <Badge
          size="xs"
          :variant="product.fulfillment_type === 'auto' ? 'info' : 'neutral'"
        >
          <Zap v-if="product.fulfillment_type === 'auto'" class="hidden size-3 md:block" />
          <Pencil v-else class="hidden size-3 md:block" />
          {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
        </Badge>
      </div>

      <p v-if="description" class="hidden text-xs leading-relaxed text-muted-foreground md:line-clamp-2">
        {{ description }}
      </p>

      <div class="mt-auto flex items-center justify-between gap-2 border-t pt-2">
        <div class="flex min-w-0 flex-col">
          <span class="sr-only">{{ t('products.price') }}</span>
          <span
            v-if="exclusivePrice !== null"
            class="theme-price-sm text-amber-600 dark:text-amber-300"
          >
            {{ formatPrice(exclusivePrice, siteCurrency) }}
          </span>
          <span
            v-else-if="hasPromotionPrice(product)"
            class="theme-price-sm theme-price-promotion"
            :aria-label="t('products.promotionPriceAria', { price: formatPrice(getPromotionPriceAmount(product), siteCurrency) })"
          >
            {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
          </span>
          <span
            v-else
            class="theme-price-sm"
            :aria-label="t('products.priceAria', { price: formatPrice(product.price_amount, siteCurrency) })"
          >
            {{ formatPrice(product.price_amount, siteCurrency) }}
          </span>
          <div v-if="exclusivePrice !== null" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <span class="hidden md:inline text-xs text-muted-foreground opacity-80 line-through">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
            <Badge variant="warning" size="xs">
              {{ t('products.memberPriceTag') }}
            </Badge>
          </div>
          <div v-else-if="hasPromotionPrice(product)" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <span
              class="hidden md:inline text-xs text-muted-foreground opacity-80 line-through"
              :aria-label="t('products.originalPriceAria', { price: formatPrice(product.price_amount, siteCurrency) })"
            >{{ formatPrice(product.price_amount, siteCurrency) }}</span>
            <Badge variant="danger" size="xs">
              {{ t('products.promotionTag') }}
            </Badge>
          </div>
          <div v-else-if="hasWholesalePrices(product)" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <Badge variant="success" size="xs">
              {{ t('products.wholesaleTag') }}
            </Badge>
          </div>
          <div v-else-if="hasPromotionRules(product)" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <Badge variant="warning" size="xs">
              {{ t('products.promotionBadge') }}
            </Badge>
          </div>
        </div>

        <Button
          v-if="isSoldOut(product)"
          type="button"
          variant="ghost"
          size="icon"
          class="size-8 shrink-0"
          :aria-label="t('products.viewDetails')"
          @click.stop="$emit('click', product.slug)"
        >
          <ArrowRight class="size-4" />
        </Button>
        <Button
          v-else
          type="button"
          variant="outline"
          size="icon"
          class="size-8 shrink-0"
          :aria-label="t('products.quickBuyAria')"
          @click.stop="$emit('quickBuy', product)"
        >
          <ShoppingCart class="size-4" />
        </Button>
      </div>
    </div>
  </Card>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, ref, watch } from 'vue'
import { ArrowRight, Image as ImageIcon, Lock, Pencil, ShoppingCart, UserPlus, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../utils/image'
import { useLocalized, useProductLabels } from '../composables/useProduct'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

const props = withDefaults(defineProps<{
  product: any
  index?: number
  maxTags?: number
  animationStep?: number
}>(), {
  index: 0,
  maxTags: 2,
  animationStep: 50,
})

defineEmits<{
  click: [slug: string]
  quickBuy: [product: any]
}>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { getPurchaseTypeLabel, getFulfillmentTypeLabel, getStockBadgeVariant, getStockStatusLabel, isSoldOut, hasPromotionPrice, getPromotionPriceAmount, hasPromotionRules, hasWholesalePrices, getCardExclusivePrice } = useProductLabels()
const exclusivePrice = computed(() => getCardExclusivePrice(props.product))
const description = computed(() => getLocalizedText(props.product.description))

const imageErrored = ref(false)
const attemptIdx = ref(0)
const productImageSrc = computed(() => getFirstImageUrl(props.product?.images))

const imageCandidates = computed<string[]>(() => {
  const arr: string[] = []
  const primary = productImageSrc.value
  if (primary) arr.push(primary)
  const categoryIcon = props.product?.category?.icon
  if (categoryIcon) {
    const resolved = getImageUrl(categoryIcon)
    if (resolved && resolved !== primary) arr.push(resolved)
  }
  return arr
})

const displayImageSrc = computed(() => imageCandidates.value[attemptIdx.value] ?? '')

watch(imageCandidates, () => {
  attemptIdx.value = 0
  imageErrored.value = false
}, { deep: true })

const handleImageError = () => {
  if (attemptIdx.value < imageCandidates.value.length - 1) {
    attemptIdx.value++
  } else {
    imageErrored.value = true
  }
}
</script>
