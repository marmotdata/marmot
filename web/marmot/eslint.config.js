import prettier from 'eslint-config-prettier';
import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import ts from 'typescript-eslint';

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
		ignores: ['build/', '.svelte-kit/', 'dist/']
	}
);
