<script setup>
const router = useRouter()
</script>

<template>
  <main
    class="grid min-h-screen place-items-center bg-slate-50 p-6 text-center"
  >
    <div class="max-w-md">
      <span
        class="grid mx-auto h-14 w-14 place-items-center rounded-2xl bg-slate-950 text-xl text-cyan-300"
        ><i class="pi pi-compass"
      /></span>
      <p
        class="mt-7 text-sm font-semibold uppercase tracking-[0.2em] text-cyan-700"
      >
        Page not found
      </p>
      <h1 class="mt-3 text-3xl font-semibold tracking-tight text-slate-950">
        This console route does not exist.
      </h1>
      <p class="mt-3 text-sm leading-6 text-slate-500">
        Return to the gateway board from the administration navigation.
      </p>
      <Button
        label="Open gateway board"
        icon="pi pi-arrow-right"
        class="mt-7"
        @click="router.push('/board')"
      />
    </div>
  </main>
</template>
