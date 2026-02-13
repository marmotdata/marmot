import typography from '@tailwindcss/typography';

/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	darkMode: 'class',
	theme: {
		extend: {
			colors: {
				gray: {
					50: '#F9F9F9',
					100: '#ECECEC',
					200: '#DFDFDF',
					300: '#CCCCCC',
					400: '#B0B0B0',
					500: '#8F8F8F',
					600: '#696969',
					700: '#4D4D4D',
					800: '#2E2E2E',
					900: '#1A1A1A'
				},
				'earthy-green': {
					50: '#f9faf8',
					100: '#f4f7f5',
					200: '#e8f0e8',
					300: '#d1dfd1',
					400: '#b9cdb9',
					500: '#8ba88b',
					600: '#75927f',
					700: '#607b60',
					800: '#4a674a',
					900: '#35593b'
				},
				'earthy-brown': {
					50: '#fefcfb',
					100: '#f9f7f5',
					200: '#f0ece8',
					300: '#e2d9d1',
					400: '#c6b4a2',
					500: '#b8a28b',
					600: '#a98f75',
					700: '#997c60',
					800: '#8a6a4a',
					900: '#7b5935'
				},
				'earthy-yellow': {
					50: '#fefdf8',
					100: '#fdf8f2',
					200: '#fbf1e5',
					300: '#f6e6c9',
					400: '#ecd292',
					500: '#e7c776',
					600: '#ddb65c',
					700: '#d4a442',
					800: '#cb9228',
					900: '#c2810f'
				},
				'earthy-terracotta': {
					50: '#fff6f3',
					100: '#ffe9e1',
					200: '#ffd7c8',
					300: '#ffbea5',
					400: '#ffa382',
					500: '#ff8a66',
					600: '#f5704a',
					700: '#d25a30',
					800: '#c74624',
					900: '#9d361a'
				},
				'earthy-blue': {
					50: '#f9fafb',
					100: '#f4f7f9',
					200: '#e8edf0',
					300: '#d1dbe2',
					400: '#a2b4c6',
					500: '#8ba0b8',
					600: '#758c9f',
					700: '#607985',
					800: '#4a656f',
					900: '#355159'
				}
			}
		}
	},
	plugins: [typography]
};
