import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // The Go server mounts the UI under /app (consts.URLPathUI).
  base: '/app/',
  build: {
    outDir: 'dist',
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:4533',
      '/auth': 'http://127.0.0.1:4533',
    },
  },
})
