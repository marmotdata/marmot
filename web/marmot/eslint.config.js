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
			'@typescript-eslint/no-explicit-any': 'warn',
			'svelte/no-at-html-tags': 'warn',
			'svelte/infinite-reactive-loop': 'warn',
			'svelte/no-immutable-reactive-statements': 'warn',
			'svelte/no-dom-manipulating': 'warn',
			'svelte/prefer-writable-derived': 'off',
			'svelte/no-reactive-functions': 'warn',
			'@typescript-eslint/no-unused-vars': [
				'warn',
				{
					argsIgnorePattern: '^_',
					varsIgnorePattern: '^_',
					caughtErrorsIgnorePattern: '^_|^e$|^err|^error'
				}
			],
			'no-empty': 'warn',
			'no-case-declarations': 'warn',
			'svelte/require-each-key': 'warn'
		}
	},
	{
		ignores: ['build/', '.svelte-kit/', 'dist/']
	}
);
