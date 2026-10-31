import typography from '@tailwindcss/typography';
import preset from '@marmotdata/tokens/tailwind-preset';

/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	presets: [preset],
	darkMode: 'class',
	theme: {
		extend: {
			colors: {
				// Bridge for screens not yet rebuilt on the design system: the old palettes resolve to the Earth & survey neutrals and status hues, so nothing off-palette shows while pages move to the mm-* tokens. New code uses the mm-* colours from the preset.
				white: '#ffffff',
				gray: {
					50: '#f6f3ef',
					100: '#efebe6',
					200: '#e1ddd8',
					300: '#ccc7c1',
					400: '#948d86',
					500: '#6f6a65',
					600: '#595450',
					700: '#36322f',
					800: '#1c1916',
					900: '#151210'
				},
				'earthy-green': {
					50: '#f1f6f2',
					100: '#e6efe8',
					200: '#cfe0d2',
					300: '#b4cfb9',
					400: '#8fb594',
					500: '#6f9676',
					600: '#4d7455',
					700: '#4d7455',
					800: '#3d5c44',
					900: '#2f4734'
				},
				'earthy-brown': {
					50: '#fdfbfa',
					100: '#f6f3ef',
					200: '#efebe6',
					300: '#e1ddd8',
					400: '#ccc7c1',
					500: '#948d86',
					600: '#6f6a65',
					700: '#595450',
					800: '#36322f',
					900: '#191511'
				},
				'earthy-yellow': {
					50: '#fbf6ea',
					100: '#f8f0dc',
					200: '#f0e2bf',
					300: '#e6cf98',
					400: '#d4a75a',
					500: '#c2963f',
					600: '#a27825',
					700: '#85611a',
					800: '#6c4f15',
					900: '#553e10'
				},
				'earthy-terracotta': {
					50: '#fcede2',
					100: '#fcede2',
					200: '#f6d6c1',
					300: '#efb895',
					400: '#eb9666',
					500: '#b05019',
					600: '#b05019',
					700: '#b05019',
					800: '#97400d',
					900: '#7d340a'
				},
				'earthy-blue': {
					50: '#eef3f6',
					100: '#e4ecf0',
					200: '#cbd8e0',
					300: '#aec3cf',
					400: '#93aab8',
					500: '#6c8594',
					600: '#506775',
					700: '#506775',
					800: '#3f525d',
					900: '#303f48'
				}
			},
			borderRadius: {
				// Radii follow role (4 controls, 6 panels and buttons, 10 dialogs), so the old larger steps collapse onto them.
				lg: '6px',
				xl: '6px',
				'2xl': '10px',
				'3xl': '10px',
				dialog: 'var(--radius-lg)'
			}
		}
	},
	plugins: [typography]
};
