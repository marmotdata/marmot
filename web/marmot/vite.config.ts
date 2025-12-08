import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import Icons from 'unplugin-icons/vite';
import { nodePolyfills } from 'vite-plugin-node-polyfills';

export default defineConfig({
  plugins: [
    sveltekit(),
    Icons({
      compiler: 'svelte',
      autoInstall: true
    }),
    nodePolyfills({
      protocolImports: true
    })
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/auth-providers': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '^/auth/(?!callback).*': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
});
