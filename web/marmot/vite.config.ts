import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import adapter from '@sveltejs/adapter-static';
import { paraglideVitePlugin } from '@inlang/paraglide-js';
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

export default defineConfig(({ command }) => ({
	plugins: [
		iconBundlePlugin(),
		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide',
			strategy: ['localStorage', 'preferredLanguage', 'baseLocale'],
			// One module per message tree-shakes best for production, but the dev server serves modules unbundled, so it would fetch thousands of them on every page load. One module per locale keeps dev to a handful of requests.
			outputStructure: command === 'build' ? 'message-modules' : 'locale-modules'
		}),
		sveltekit({
			preprocess: vitePreprocess(),
			compilerOptions: {
				// mm-* elements are focusable controls with their own keyboard handling, so the static-element a11y warnings Svelte raises for a click handler on an unknown tag do not apply to them.
				warningFilter: (warning) =>
					!(
						(warning.code === 'a11y_click_events_have_key_events' ||
							warning.code === 'a11y_no_static_element_interactions') &&
						/<mm-[a-z-]+/.test(warning.frame ?? '')
					)
			},
			adapter: adapter({
				pages: 'build',
				assets: 'build',
				fallback: 'index.html',
				precompress: false,
				strict: true
			}),
			paths: {
				base: ''
			}
		}),
		nodePolyfills({
			protocolImports: true,
			// Rolldown's runtime imports createRequire from node:module, so stubbing it out breaks the build, and nothing in the browser bundle needs it.
			exclude: ['module']
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
}));
