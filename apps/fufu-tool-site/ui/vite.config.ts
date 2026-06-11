import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  root: fileURLToPath(new URL('.', import.meta.url)),
  plugins: [react(), tailwindcss()],
  build: {
    outDir: '../ui-dist',
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/fufu-ui-[hash].js',
        chunkFileNames: 'assets/fufu-ui-[hash].js',
        assetFileNames: 'assets/fufu-ui-[hash][extname]'
      }
    }
  },
  server: {
    host: '127.0.0.1',
    port: 5174
  }
});
