<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { resolve } from '$app/paths';
	import '@marmotdata/elements/banner.js';

	export let enabled: boolean = false;
	export let dismissible: boolean = true;
	export let variant: 'info' | 'warning' | 'error' | 'success' = 'info';
	export let message: string = '';
	export let id: string = 'banner-1';

	let visible = false;




	onMount(() => {
		if (!enabled || !browser) {
			return;
		}

		if (dismissible) {
			const dismissed = localStorage.getItem(`banner-dismissed-${id}`);
			visible = !dismissed;
		} else {
			visible = true;
		}
	});

	function dismiss() {
		if (browser && dismissible) {
			localStorage.setItem(`banner-dismissed-${id}`, 'true');
			visible = false;
		}
	}

	function parseMarkdownLinks(
		text: string
	): { type: 'text' | 'link'; content: string; url?: string }[] {
		const parts: { type: 'text' | 'link'; content: string; url?: string }[] = [];
		const linkRegex = /\[([^\]]+)\]\(([^)]+)\)/g;
		let lastIndex = 0;
		let match;

		while ((match = linkRegex.exec(text)) !== null) {
			if (match.index > lastIndex) {
				parts.push({
					type: 'text',
					content: text.slice(lastIndex, match.index)
				});
			}

			parts.push({
				type: 'link',
				content: match[1],
				url: match[2]
			});

			lastIndex = match.index + match[0].length;
		}

		if (lastIndex < text.length) {
			parts.push({
				type: 'text',
				content: text.slice(lastIndex)
			});
		}

		return parts.length > 0 ? parts : [{ type: 'text', content: text }];
	}

	$: messageParts = parseMarkdownLinks(message);
</script>

{#if visible && enabled}
	<mm-banner
		tone={variant === 'error' ? 'danger' : variant === 'success' ? 'info' : variant}
		{dismissible}
		onmm-dismiss={dismiss}
	>
		{#each messageParts as part, i (i)}
			{#if part.type === 'link'}
				{#if part.url?.startsWith('http')}
					<a
						href={part.url}
						class="mm-link"
						target="_blank"
						rel="external noopener noreferrer">{part.content}</a
					>
				{:else}
					<a href={resolve((part.url ?? '/') as '/')} class="mm-link">{part.content}</a>
				{/if}
			{:else}
				<span>{part.content}</span>
			{/if}
		{/each}
	</mm-banner>
{/if}
