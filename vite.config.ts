import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // In dev, proxy all backend routes to the Go server on :8080.
    // In production, frontend and API share the same origin (no proxy needed).
    proxy: {
      '/health':    { target: 'http://localhost:8080', changeOrigin: true },
      '/platforms': { target: 'http://localhost:8080', changeOrigin: true },
      '/oauth':     { target: 'http://localhost:8080', changeOrigin: true },
      '/batches':   { target: 'http://localhost:8080', changeOrigin: true },
      '/jobs':      { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
