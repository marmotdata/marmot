import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	// Consult https://svelte.dev/docs/kit/integrations
	// for more information about preprocessors
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

	kit: {
		// adapter-auto only supports some environments, see https://svelte.dev/docs/kit/adapter-auto for a list.
		// If your environment is not supported, or you settled on a specific environment, switch out the adapter.
		// See https://svelte.dev/docs/kit/adapters for more information about adapters.
		adapter: adapter({
			// default options are shown. On some platforms
			// these options are set automatically — see below
			pages: 'build',
			assets: 'build',
			fallback: 'index.html',
			precompress: false,
			strict: true
		}),
		paths: {
			base: ''
		},
		alias: {
			$components: 'src/components'
		}
	}
};

export default config;
