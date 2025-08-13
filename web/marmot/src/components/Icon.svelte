<script lang="ts">
	import { onMount } from 'svelte';
	import { IconLoader, type IconResult } from '$lib/iconloader.ts';

	export let name: string;
	export let showLabel: boolean = true;
	export let size: 'xs' | 'sm' | 'md' | 'lg' = 'md';

	const sizeClasses = {
		xs: 'w-1 h-1',
		sm: 'w-6 h-6',
		md: 'w-8 h-8',
		lg: 'w-12 h-12'
	};

	let iconResult: IconResult | null = null;
	let isComponent = false;
	let componentClass = '';
	let mounted = false;

	async function updateIcon() {
		if (!mounted || !name) return;
		const isDark = document.documentElement.classList.contains('dark');

		try {
			iconResult = await IconLoader.getInstance().loadIcon(name, isDark);
			isComponent = typeof iconResult !== 'string' && 'component' in iconResult;

			if (isComponent && typeof iconResult !== 'string') {
				componentClass = iconResult.class || '';
			}
		} catch (error) {
			console.error(`Failed to load icon: ${name}`, error);
			iconResult = '/images/marmot.svg';
			isComponent = false;
		}
	}

	onMount(() => {
		mounted = true;
		updateIcon();

		const observer = new MutationObserver((mutations) => {
			mutations.forEach((mutation) => {
				if (mutation.target.nodeName === 'HTML' && mutation.attributeName === 'class') {
					updateIcon();
				}
			});
		});

		observer.observe(document.documentElement, {
			attributes: true,
			attributeFilter: ['class']
		});

		return () => observer.disconnect();
	});

	$: if (mounted && name) {
		updateIcon();
	}
</script>

<div class="flex flex-col items-center gap-2">
	{#if isComponent && iconResult && typeof iconResult !== 'string'}
		<div class={componentClass}>
			<svelte:component this={iconResult.component} class="{sizeClasses[size]} object-contain" />
		</div>
	{:else if iconResult && typeof iconResult === 'string'}
		<img src={iconResult} alt={`${name} icon`} class="{sizeClasses[size]} object-contain" />
	{/if}
	{#if showLabel}
		<span class="font-medium text-gray-900 dark:text-gray-100 text-center">{name}</span>
	{/if}
</div>
