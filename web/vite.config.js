import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/app/',
  plugins: [vue(), tailwindcss()],
  build: { outDir: 'dist', emptyOutDir: true },
})
