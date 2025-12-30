<script lang="ts">
	import { onMount } from 'svelte';
	import { IconLoader, typeIconMap, providerIconMap } from '$lib/iconloader';
	import type { ComponentType, SvelteComponent } from 'svelte';
	import Icon from '@iconify/svelte';

	export let assetType: string | undefined = undefined;
	export let providers: string[] = [];
	export let size: 'sm' | 'md' | 'lg' = 'md';

	const sizeClasses = {
		sm: 'w-4 h-4',
		md: 'w-5 h-5',
		lg: 'w-6 h-6'
	};

	let iconComponent: ComponentType<SvelteComponent> | null = null;
	let iconUrl: string | null = null;
	let iconClass = '';
	let isDark = false;

	$: sizeClass = sizeClasses[size];

	onMount(() => {
		isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
		loadIcon();

		const darkModeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
		const handleChange = (e: MediaQueryListEvent) => {
			isDark = e.matches;
			loadIcon();
		};
		darkModeMediaQuery.addEventListener('change', handleChange);
		return () => darkModeMediaQuery.removeEventListener('change', handleChange);
	});

	async function loadIcon() {
		// Priority: provider icon > type icon > default
		const primaryProvider = providers?.[0];

		// Try provider icon first
		if (primaryProvider) {
			const formattedProvider = primaryProvider.toLowerCase().replace(/_/g, '-');
			if (providerIconMap[formattedProvider]) {
				const iconConfig = providerIconMap[formattedProvider];
				iconComponent = isDark && iconConfig.dark ? iconConfig.dark : iconConfig.default;
				iconClass = iconConfig.class || '';
				return;
			}

			// Try loading custom provider icon
			try {
				const loader = IconLoader.getInstance();
				const result = await loader.loadIcon(primaryProvider, isDark);
				if (typeof result === 'string') {
					iconUrl = result;
					iconComponent = null;
				} else {
					iconComponent = result.component;
					iconClass = result.class || '';
				}
				return;
			} catch (error) {
				console.debug(`No custom icon for provider: ${primaryProvider}`);
			}
		}

		// Fall back to type icon
		if (assetType) {
			const formattedType = assetType.toLowerCase().replace(/_/g, '-');
			if (typeIconMap[formattedType]) {
				const iconConfig = typeIconMap[formattedType];
				iconComponent = isDark && iconConfig.dark ? iconConfig.dark : iconConfig.default;
				iconClass = iconConfig.class || '';
				return;
			}
		}

		// Default fallback
		iconUrl = null;
		iconComponent = null;
	}
</script>

{#if iconComponent}
	<svelte:component this={iconComponent} class="{sizeClass} {iconClass}" aria-hidden="true" />
{:else if iconUrl}
	<img src={iconUrl} alt="" class="{sizeClass} object-contain" aria-hidden="true" />
{:else}
	<Icon
		icon="mdi:database"
		class="{sizeClass} text-gray-600 dark:text-gray-400"
		aria-hidden="true"
	/>
{/if}
