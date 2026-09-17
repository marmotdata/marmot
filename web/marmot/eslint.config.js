import prettier from 'eslint-config-prettier';
import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import ts from 'typescript-eslint';
import i18next from 'eslint-plugin-i18next';
import noUntranslatedStrings from './eslint-rules/no-untranslated-strings.js';

// Local plugin housing Marmot-specific rules from ./eslint-rules.
const marmot = {
	rules: {
		'no-untranslated-strings': noUntranslatedStrings
	}
};

export default ts.config(
	js.configs.recommended,
	...ts.configs.recommended,
	...svelte.configs['flat/recommended'],
	prettier,
	...svelte.configs['flat/prettier'],
	{
		languageOptions: {
			globals: {
				...globals.browser,
				...globals.node
			}
		}
	},
	{
		files: ['**/*.svelte'],

		languageOptions: {
			parserOptions: {
				parser: ts.parser
			}
		}
	},
	{
		rules: {
			'@typescript-eslint/no-explicit-any': 'error',
			'svelte/no-at-html-tags': 'error',
			'svelte/infinite-reactive-loop': 'error',
			'svelte/no-immutable-reactive-statements': 'error',
			'svelte/no-dom-manipulating': 'error',
			'svelte/prefer-writable-derived': 'off',
			'svelte/no-reactive-functions': 'error',
			'@typescript-eslint/no-unused-vars': [
				'error',
				{
					argsIgnorePattern: '^_',
					varsIgnorePattern: '^_',
					caughtErrorsIgnorePattern: '^_|^e$|^err|^error'
				}
			],
			'no-empty': 'error',
			'no-case-declarations': 'error',
			'svelte/require-each-key': 'error',
			'svelte/no-navigation-without-resolve': 'error',
			'svelte/prefer-svelte-reactivity': 'error'
		}
	},
	{
		files: ['**/*.svelte'],
		plugins: { marmot },
		rules: {
			'marmot/no-untranslated-strings': 'error'
		}
	},
	{
		// Hand-written TS only: the Paraglide output is generated and the icon bundles are data.
		files: ['src/**/*.ts'],
		ignores: ['src/lib/paraglide/**', 'src/lib/icon-*', '**/*.d.ts'],
		plugins: { i18next },
		rules: {
			'i18next/no-literal-string': 'warn'
		}
	},
	{
		ignores: ['build/', '.svelte-kit/', 'dist/', 'src/lib/paraglide/']
	}
);
