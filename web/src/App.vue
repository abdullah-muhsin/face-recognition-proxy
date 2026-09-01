<script setup>
import { useGatewayStore } from './stores/gateway'

const gateway = useGatewayStore()

onMounted(() => {
  gateway.initialize()
})
onBeforeUnmount(() => {
  gateway.dispose()
})
</script>

<template>
  <Toast position="top-right" />
  <main
    v-if="!gateway.initialized"
    class="grid min-h-screen place-items-center bg-slate-50 px-6"
  >
    <div class="flex items-center gap-3 text-sm font-medium text-slate-600">
      <ProgressSpinner stroke-width="4" class="h-6 w-6" />
      Preparing operator console
    </div>
  </main>
  <RouterView v-else />
</template>
