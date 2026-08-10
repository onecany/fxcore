import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// FXcore-web Vite 配置（React 19 + TS；后端托管构建产物时可复用）
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // 本地开发代理到 fxcore 后端（默认 :8080）
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:8080', ws: true },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        // 代码分割：第三方大块独立分包，业务代码更新不影响 vendor/charts 长缓存
        manualChunks: {
          vendor: ['react', 'react-dom', 'react-router', 'swr', 'zustand', 'axios'],
          charts: ['lightweight-charts'],
        },
      },
    },
  },
});
