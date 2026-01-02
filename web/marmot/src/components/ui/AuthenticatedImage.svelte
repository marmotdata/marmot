<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { onMount } from 'svelte';

	interface Props {
		src: string;
		alt: string;
		class?: string;
		fallback?: import('svelte').Snippet;
	}

	let { src, alt, class: className = '', fallback }: Props = $props();

	let blobUrl = $state<string | null>(null);
	let loading = $state(true);
	let error = $state(false);

	async function fetchImage(url: string) {
		loading = true;
		error = false;

		try {
			const token = auth.getToken();
			const headers: Record<string, string> = {};
			if (token) {
				headers['Authorization'] = `Bearer ${token}`;
			}

			const response = await fetch(url, { headers });

			if (!response.ok) {
				throw new Error('Failed to fetch image');
			}

			const blob = await response.blob();

			// Revoke old blob URL if exists
			if (blobUrl) {
				URL.revokeObjectURL(blobUrl);
			}

			blobUrl = URL.createObjectURL(blob);
		} catch (e) {
			console.error('Failed to load authenticated image:', e);
			error = true;
		} finally {
			loading = false;
		}
	}

	// Fetch image when src changes
	$effect(() => {
		if (src) {
			fetchImage(src);
		} else {
			loading = false;
			error = true;
		}
	});

	// Cleanup blob URL on unmount
	onMount(() => {
		return () => {
			if (blobUrl) {
				URL.revokeObjectURL(blobUrl);
			}
		};
	});
</script>

{#if loading}
	<div class="{className} animate-pulse bg-gray-200 dark:bg-gray-700"></div>
{:else if error || !blobUrl}
	{#if fallback}
		{@render fallback()}
	{:else}
		<div class="{className} bg-gray-200 dark:bg-gray-700"></div>
	{/if}
{:else}
	<img src={blobUrl} {alt} class={className} />
{/if}
