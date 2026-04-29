import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import Icons from 'unplugin-icons/vite';
import { nodePolyfills } from 'vite-plugin-node-polyfills';
import { generateIconBundle } from './scripts/generate-icon-bundle.mjs';
import { dirname } from 'path';
import { fileURLToPath } from 'url';

function iconBundlePlugin() {
	const root = dirname(fileURLToPath(import.meta.url));
	return {
		name: 'icon-bundle',
		buildStart() {
			generateIconBundle(root);
		},
		handleHotUpdate({ file }: { file: string }) {
			if (file.endsWith('.svelte') && !file.includes('icon-bundle')) {
				generateIconBundle(root);
			}
		}
	};
}

export default defineConfig({
	plugins: [
		iconBundlePlugin(),
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
			'/.well-known': {
				target: 'http://localhost:8080',
				changeOrigin: true
			},
			'/oauth': {
				target: 'http://localhost:8080',
				changeOrigin: true
			},
			'/auth-providers': {
				target: 'http://localhost:8080',
				changeOrigin: true
			},
			'/auth': {
				target: 'http://localhost:8080',
				changeOrigin: true
			}
		}
	}
});
