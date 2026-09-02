<script setup>
import { useGatewayStore } from '../stores/gateway'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const submitting = ref(false)
const errorMessage = ref('')

async function signIn() {
  errorMessage.value = ''
  submitting.value = true
  try {
    await gateway.signIn({ username: username.value, password: password.value })
    password.value = ''
    const redirect =
      typeof route.query.redirect === 'string' ? route.query.redirect : '/board'
    await router.replace(redirect)
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-slate-100 p-4 sm:p-6">
    <section
      class="grid w-full max-w-4xl overflow-hidden rounded-lg border border-slate-300 bg-white shadow-xl shadow-slate-300/40 lg:grid-cols-[0.9fr_1.1fr]"
    >
      <div class="bg-slate-950 p-7 text-slate-100 sm:p-9">
        <div class="flex items-center gap-3">
          <span
            class="grid h-10 w-10 place-items-center rounded-lg bg-cyan-300 text-lg text-slate-950"
            ><i class="pi pi-shield"
          /></span>
          <div>
            <p class="font-semibold">PushSDK gateway</p>
            <p class="text-xs text-slate-400">Administration console</p>
          </div>
        </div>
        <div class="mt-12">
          <p
            class="text-xs font-semibold uppercase tracking-[0.18em] text-cyan-300"
          >
            Restricted access
          </p>
          <h1 class="mt-2 text-3xl font-semibold tracking-tight">
            Gateway visibility and detail.
          </h1>
          <p class="mt-4 text-sm leading-6 text-slate-300">
            Inspect retained device payloads, terminal state, and durable
            gateway activity from one administration board.
          </p>
        </div>
        <dl class="mt-10 space-y-4 border-t border-white/10 pt-6 text-xs">
          <div>
            <dt class="font-medium text-cyan-200">Event archive</dt>
            <dd class="mt-1 leading-5 text-slate-400">
              Exact terminal source bytes are retained separately from activity.
            </dd>
          </div>
          <div>
            <dt class="font-medium text-cyan-200">Activity archive</dt>
            <dd class="mt-1 leading-5 text-slate-400">
              Protocol and administrative changes are stored before streaming.
            </dd>
          </div>
        </dl>
      </div>

      <div class="flex items-center p-7 sm:p-10">
        <div class="mx-auto w-full max-w-sm">
          <p
            class="text-xs font-semibold uppercase tracking-[0.18em] text-cyan-700"
          >
            Administrator authentication
          </p>
          <h2 class="mt-2 text-2xl font-semibold tracking-tight text-slate-950">
            Sign in
          </h2>
          <p class="mt-2 text-sm text-slate-600">
            Use the administrator credentials configured for this gateway.
          </p>
          <form class="mt-7 space-y-5" @submit.prevent="signIn">
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-700"
                >Username</span
              >
              <InputText
                v-model="username"
                class="w-full"
                autocomplete="username"
                required
              />
            </label>
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-700"
                >Password</span
              >
              <Password
                v-model="password"
                class="w-full"
                input-class="w-full"
                :feedback="false"
                toggle-mask
                autocomplete="current-password"
                required
              />
            </label>
            <Message
              v-if="errorMessage || gateway.bootstrapError"
              severity="error"
              :closable="false"
              >{{ errorMessage || gateway.bootstrapError }}</Message
            >
            <Button
              type="submit"
              class="w-full"
              label="Open administration console"
              icon="pi pi-arrow-right"
              icon-pos="right"
              :loading="submitting"
            />
          </form>
        </div>
      </div>
    </section>
  </main>
</template>
