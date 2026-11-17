<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useUserAuthStore } from '../../stores/userAuth'
import { useAppStore } from '../../stores/app'
import type { CaptchaPayload } from '../../api'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Alert, AlertDescription } from '../ui/alert'
import ImageCaptcha from '../captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../captcha/TurnstileCaptcha.vue'

const props = defineProps<{ disabled?: boolean }>()
const emit = defineEmits<{ authenticated: [] }>()
const { t } = useI18n()
const auth = useUserAuthStore()
const app = useAppStore()
const route = useRoute()
const id = useId()
const email = ref('')
const password = ref('')
const agreed = ref(false)
const code = ref('')
const inviteCode = ref(typeof route.query.invite === 'string' ? route.query.invite : '')
const registrationRequired = ref(false)
const emailVerificationRequired = ref(false)
const inviteCodeRequired = ref(false)
const totp = ref(false)
const recovery = ref(false)
const secondFactor = ref('')
const busy = ref(false)
const sending = ref(false)
const error = ref('')
const countdown = ref(0)
const codeSent = ref(false)
let timer: ReturnType<typeof setInterval> | undefined
const loginCaptcha = ref<CaptchaPayload>({})
const sendCaptcha = ref<CaptchaPayload>({})
const loginToken = ref('')
const sendToken = ref('')
const loginImage = ref<InstanceType<typeof ImageCaptcha> | null>(null)
const sendImage = ref<InstanceType<typeof ImageCaptcha> | null>(null)
const loginTurnstile = ref<InstanceType<typeof TurnstileCaptcha> | null>(null)
const sendTurnstile = ref<InstanceType<typeof TurnstileCaptcha> | null>(null)
const captchaConfig = computed(() => app.config?.captcha)
const provider = computed(() => captchaConfig.value?.provider || 'none')
const siteKey = computed(() => String(captchaConfig.value?.turnstile?.site_key || ''))
const needsLoginCaptcha = computed(() => provider.value !== 'none' && !!captchaConfig.value?.scenes?.login)
const needsSendCaptcha = computed(() => provider.value !== 'none' && !!captchaConfig.value?.scenes?.register_send_code)
const registrationEnabled = computed(() => app.config?.registration_enabled !== false)
// 两个场景的验证令牌都只能使用一次，同一时间只展示一个验证组件：
// 新用户在发送邮箱验证码前只显示发送验证，验证码发出后再显示提交所需的登录验证。
const showSendCaptcha = computed(() => emailVerificationRequired.value && needsSendCaptcha.value && !codeSent.value)
const showLoginCaptcha = computed(() => needsLoginCaptcha.value && !showSendCaptcha.value)

function captchaPayload(scene: 'login' | 'send'): CaptchaPayload | undefined {
  const needed = scene === 'login' ? needsLoginCaptcha.value : needsSendCaptcha.value
  if (!needed) return undefined
  return provider.value === 'turnstile'
    ? { turnstile_token: scene === 'login' ? loginToken.value : sendToken.value }
    : scene === 'login' ? loginCaptcha.value : sendCaptcha.value
}

function resetCaptcha(scene: 'login' | 'send') {
  if (scene === 'login') {
    loginImage.value?.refresh()
    loginTurnstile.value?.reset()
    loginToken.value = ''
  } else {
    sendImage.value?.refresh()
    sendTurnstile.value?.reset()
    sendToken.value = ''
  }
}

async function sendCode() {
  if (sending.value || countdown.value > 0 || busy.value) return
  error.value = ''
  if (needsSendCaptcha.value && codeSent.value) {
    // 重新发送需要新的发送验证令牌，先切回发送验证组件。
    codeSent.value = false
    return
  }
  sending.value = true
  try {
    await auth.sendVerifyCode({ email: email.value.trim(), purpose: 'register', invite_code: inviteCode.value.trim(), captcha_payload: captchaPayload('send') })
    codeSent.value = true
    countdown.value = 60
    if (timer) clearInterval(timer)
    timer = setInterval(() => { if (--countdown.value <= 0) clearInterval(timer) }, 1000)
  } catch (err: any) {
    error.value = err.message || t('auth.register.errors.sendCodeFailed')
  } finally {
    sending.value = false
    resetCaptcha('send')
  }
}

async function submit() {
  if (props.disabled || busy.value || sending.value) return
  error.value = ''
  busy.value = true
  try {
    if (totp.value) {
      await auth.verify2FA(recovery.value ? { recovery_code: secondFactor.value.trim() } : { code: secondFactor.value.trim() })
    } else {
      const result = await auth.loginOrRegister({ email: email.value.trim(), password: password.value, agreement_accepted: agreed.value, code: code.value.trim(), invite_code: inviteCode.value.trim(), remember_me: false, captcha_payload: captchaPayload('login') })
      if (result.registrationRequired) {
        registrationRequired.value = true
        emailVerificationRequired.value = !!result.emailVerificationRequired
        inviteCodeRequired.value = !!result.inviteCodeRequired
        return
      }
      if (result.requiresTotp) {
        totp.value = true
        password.value = ''
        return
      }
    }
    password.value = ''
    if (auth.isAuthenticated) emit('authenticated')
  } catch (err: any) {
    error.value = err.message || t('auth.login.error')
  } finally {
    busy.value = false
    resetCaptcha('login')
  }
}

function backToPassword() {
  auth.clearChallenge()
  totp.value = false
  secondFactor.value = ''
  error.value = ''
}

watch(email, () => {
  registrationRequired.value = false
  emailVerificationRequired.value = false
  inviteCodeRequired.value = false
  code.value = ''
  codeSent.value = false
  if (timer) clearInterval(timer)
  countdown.value = 0
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  if (totp.value) auth.clearChallenge()
})
</script>

<template>
  <form data-purchase-account class="flex flex-col gap-4 rounded-xl border bg-muted/40 p-4" @submit.prevent="submit">
    <fieldset class="flex min-w-0 flex-col gap-4" :disabled="busy || sending">
      <legend class="mb-3 font-semibold">{{ t('purchaseAccount.title') }}</legend>
      <p class="text-sm text-muted-foreground">{{ t(registrationEnabled ? 'purchaseAccount.hint' : 'purchaseAccount.loginOnly') }}</p>
      <template v-if="!totp">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="flex flex-col gap-2" :for="`${id}-email`">
            {{ t('purchaseAccount.email') }}
            <Input :id="`${id}-email`" v-model="email" type="email" autocomplete="username" required :placeholder="t('purchaseAccount.emailPlaceholder')" />
          </label>
          <label class="flex flex-col gap-2" :for="`${id}-password`">
            {{ t('purchaseAccount.password') }}
            <Input :id="`${id}-password`" v-model="password" type="password" autocomplete="current-password" required :placeholder="t('purchaseAccount.passwordPlaceholder')" />
          </label>
        </div>
        <p v-if="registrationRequired" role="status" class="text-sm text-muted-foreground">{{ t('purchaseAccount.completeRegistration') }}</p>
        <label v-if="inviteCodeRequired" class="flex flex-col gap-2" :for="`${id}-invite`">
          {{ t('purchaseAccount.inviteCode') }}
          <Input :id="`${id}-invite`" v-model="inviteCode" required autocomplete="off" />
        </label>
        <template v-if="emailVerificationRequired">
          <ImageCaptcha v-if="showSendCaptcha && provider === 'image'" ref="sendImage" v-model="sendCaptcha" @config-stale="app.loadConfig(true)" />
          <TurnstileCaptcha v-if="showSendCaptcha && provider === 'turnstile'" ref="sendTurnstile" v-model="sendToken" :site-key="siteKey" />
          <label class="flex flex-col gap-2" :for="`${id}-code`">
            {{ t('purchaseAccount.emailCode') }}
            <div class="flex gap-2">
              <Input :id="`${id}-code`" v-model="code" required autocomplete="one-time-code" />
              <Button type="button" variant="outline" :disabled="sending || countdown > 0" @click="sendCode">{{ countdown > 0 ? `${countdown}s` : t('purchaseAccount.sendCode') }}</Button>
            </div>
          </label>
        </template>
        <label v-if="registrationEnabled" class="flex items-start gap-2 text-sm text-muted-foreground">
          <input v-model="agreed" type="checkbox" class="mt-1 accent-primary" :required="registrationRequired" />
          <span>{{ t('auth.register.agreementPrefix') }} <router-link to="/privacy" target="_blank" rel="noopener noreferrer" class="text-primary underline">{{ t('footer.privacy') }}</router-link> {{ t('auth.register.agreementAnd') }} <router-link to="/terms" target="_blank" rel="noopener noreferrer" class="text-primary underline">{{ t('footer.terms') }}</router-link></span>
        </label>
        <ImageCaptcha v-if="showLoginCaptcha && provider === 'image'" ref="loginImage" v-model="loginCaptcha" @config-stale="app.loadConfig(true)" />
        <TurnstileCaptcha v-if="showLoginCaptcha && provider === 'turnstile'" ref="loginTurnstile" v-model="loginToken" :site-key="siteKey" />
        <router-link to="/auth/forgot" class="text-sm text-primary underline">{{ t('auth.login.forgot') }}</router-link>
      </template>
      <template v-else>
        <label class="flex flex-col gap-2" :for="`${id}-totp`">
          {{ t(recovery ? 'purchaseAccount.recoveryCode' : 'purchaseAccount.totpCode') }}
          <Input :id="`${id}-totp`" v-model="secondFactor" required autocomplete="one-time-code" />
        </label>
        <div class="flex flex-wrap gap-2">
          <Button type="button" variant="outline" @click="recovery = !recovery; secondFactor = ''">{{ t(recovery ? 'purchaseAccount.useTotp' : 'purchaseAccount.useRecovery') }}</Button>
          <Button type="button" variant="ghost" @click="backToPassword">{{ t('purchaseAccount.back') }}</Button>
        </div>
      </template>
    </fieldset>
    <Alert v-if="error" variant="destructive" role="alert"><AlertDescription>{{ error }}</AlertDescription></Alert>
    <Button type="submit" class="w-full" :disabled="disabled || busy || sending">{{ t(busy ? 'purchaseAccount.processing' : 'purchaseAccount.continue') }}</Button>
  </form>
</template>
