import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@semi-css': fileURLToPath(
        new URL('./node_modules/@douyinfe/semi-ui/dist/css/semi.css', import.meta.url)
      )
    }
  },
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': 'http://127.0.0.1:18880'
    }
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined;
          }
          if (id.includes('@visactor')) {
            return 'vendor-vchart';
          }
          if (id.includes('@douyinfe/semi-ui') || id.includes('@douyinfe/semi-icons')) {
            return 'vendor-semi';
          }
          return 'vendor-core';
        }
      }
    }
  }
});
