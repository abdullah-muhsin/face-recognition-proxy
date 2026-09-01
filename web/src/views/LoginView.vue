<script setup>
import { useGatewayStore } from '../stores/gateway'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
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
    toast.add({
      severity: 'success',
      summary: 'Signed in',
      detail: 'The operator console is ready.',
      life: 3000,
    })
    const redirect =
      typeof route.query.redirect === 'string'
        ? route.query.redirect
        : '/overview'
    await router.replace(redirect)
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="min-h-screen bg-slate-50 px-4 py-8 sm:px-6 lg:px-8">
    <div
      class="mx-auto grid min-h-[calc(100vh-4rem)] max-w-6xl overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-2xl shadow-slate-200/60 lg:grid-cols-[1.1fr_0.9fr]"
    >
      <section
        class="order-2 flex flex-col justify-between bg-slate-950 p-8 text-white sm:p-12 lg:order-1"
      >
        <div>
          <div class="mb-12 flex items-center gap-3">
            <span
              class="grid h-11 w-11 place-items-center rounded-2xl bg-cyan-400 text-xl text-slate-950 shadow-lg shadow-cyan-400/20"
              ><i class="pi pi-bolt"
            /></span>
            <div>
              <p class="text-sm font-semibold tracking-wide text-cyan-300">
                PushSDK gateway
              </p>
              <p class="text-xs text-slate-400">Operations workspace</p>
            </div>
          </div>
          <p
            class="mb-4 text-sm font-semibold uppercase tracking-[0.22em] text-cyan-300"
          >
            Device operations
          </p>
          <h1
            class="max-w-md text-4xl font-semibold tracking-tight sm:text-5xl"
          >
            A clearer view of every device interaction.
          </h1>
          <p class="mt-6 max-w-lg text-base leading-7 text-slate-300">
            Monitor terminal health, review received attendance, and follow live
            gateway activity from one focused workspace.
          </p>
        </div>
        <div
          class="mt-12 grid gap-4 sm:grid-cols-3 lg:grid-cols-1 xl:grid-cols-3"
        >
          <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
            <i class="pi pi-shield mb-3 text-cyan-300" />
            <p class="text-sm font-semibold">Private by design</p>
            <p class="mt-1 text-xs leading-5 text-slate-400">
              The live feed contains metadata only.
            </p>
          </div>
          <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
            <i class="pi pi-wifi mb-3 text-cyan-300" />
            <p class="text-sm font-semibold">Live state</p>
            <p class="mt-1 text-xs leading-5 text-slate-400">
              Terminal connectivity is updated as events arrive.
            </p>
          </div>
          <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
            <i class="pi pi-database mb-3 text-cyan-300" />
            <p class="text-sm font-semibold">Durable records</p>
            <p class="mt-1 text-xs leading-5 text-slate-400">
              Accepted attendance is stored before the device is acknowledged.
            </p>
          </div>
        </div>
      </section>

      <section class="order-1 flex items-center p-6 sm:p-12 lg:order-2">
        <div class="mx-auto w-full max-w-sm">
          <div class="mb-8">
            <p class="text-sm font-semibold text-cyan-700">Secure sign in</p>
            <h2
              class="mt-2 text-3xl font-semibold tracking-tight text-slate-950"
            >
              Welcome back
            </h2>
            <p class="mt-2 text-sm leading-6 text-slate-500">
              Use the gateway operator credentials configured for this
              environment.
            </p>
          </div>
          <form class="space-y-5" @submit.prevent="signIn">
            <label class="block"
              ><span class="mb-2 block text-sm font-medium text-slate-700"
                >Username</span
              ><InputText
                v-model="username"
                class="w-full"
                autocomplete="username"
                required
            /></label>
            <label class="block"
              ><span class="mb-2 block text-sm font-medium text-slate-700"
                >Password</span
              ><Password
                v-model="password"
                class="w-full"
                input-class="w-full"
                :feedback="false"
                toggle-mask
                autocomplete="current-password"
                required
            /></label>
            <Message
              v-if="errorMessage || gateway.bootstrapError"
              severity="error"
              :closable="false"
              >{{ errorMessage || gateway.bootstrapError }}</Message
            >
            <Button
              type="submit"
              class="w-full"
              label="Open operator console"
              icon="pi pi-arrow-right"
              icon-pos="right"
              :loading="submitting"
            />
          </form>
          <p class="mt-8 text-center text-xs leading-5 text-slate-400">
            Gateway access is limited to authorised operators. Sessions use a
            same-site operator cookie.
          </p>
        </div>
      </section>
    </div>
  </main>
</template>
