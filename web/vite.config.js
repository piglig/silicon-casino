import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  base: '/',
  server: {
    proxy: {
      '/api': 'http://localhost:8080'
    }
  },
  build: {
    outDir: path.resolve(__dirname, '../internal/web/static'),
    emptyOutDir: true
  }
})
