import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

const src = (dir: string) => fileURLToPath(new URL(`./src/${dir}`, import.meta.url))

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],

  build: {
    outDir: '../web/out',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom', 'react-router-dom'],
        },
      },
    },
  },

  resolve: {
    alias: {
      // FSD-слои доступны без относительных путей
      '@app':      src('app'),
      '@pages':    src('pages'),
      '@widgets':  src('widgets'),
      '@features': src('features'),
      '@entities': src('entities'),
      '@shared':   src('shared'),
      '@test':     src('test'),
    },
  },

  server: {
    port: 3000,
    proxy: {
      // Все /api/* и /webhook/* проксируются на Go-сервер во время разработки
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/webhook': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },

  test: {
    environment: 'jsdom',
    setupFiles: ['./src/setupTests.ts'],
    exclude: ['**/node_modules/**', '**/e2e/**'],
  },
})
