<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import Icon from '@iconify/svelte';

	export let enabled: boolean = false;
	export let dismissible: boolean = true;
	export let variant: 'info' | 'warning' | 'error' | 'success' = 'info';
	export let message: string = '';
	export let id: string = 'banner-1';

	let visible = false;

	$: variantClasses = {
		info: 'bg-gradient-to-r from-blue-50 to-blue-100 dark:from-blue-950/40 dark:to-blue-900/40 border-blue-300 dark:border-blue-700 text-blue-900 dark:text-blue-50',
		warning: 'bg-gradient-to-r from-yellow-50 to-yellow-100 dark:from-yellow-950/40 dark:to-yellow-900/40 border-yellow-300 dark:border-yellow-700 text-yellow-900 dark:text-yellow-50',
		error: 'bg-gradient-to-r from-red-50 to-red-100 dark:from-red-950/40 dark:to-red-900/40 border-red-300 dark:border-red-700 text-red-900 dark:text-red-50',
		success: 'bg-gradient-to-r from-green-50 to-green-100 dark:from-green-950/40 dark:to-green-900/40 border-green-300 dark:border-green-700 text-green-900 dark:text-green-50'
	};

	$: iconMap = {
		info: 'material-symbols:info-outline',
		warning: 'material-symbols:warning-outline',
		error: 'material-symbols:error-outline',
		success: 'material-symbols:check-circle-outline'
	};

	$: linkColorClasses = {
		info: 'text-blue-800 dark:text-blue-200 hover:text-blue-950 dark:hover:text-blue-100 font-semibold',
		warning: 'text-yellow-800 dark:text-yellow-200 hover:text-yellow-950 dark:hover:text-yellow-100 font-semibold',
		error: 'text-red-800 dark:text-red-200 hover:text-red-950 dark:hover:text-red-100 font-semibold',
		success: 'text-green-800 dark:text-green-200 hover:text-green-950 dark:hover:text-green-100 font-semibold'
	};

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

	function parseMarkdownLinks(text: string): { type: 'text' | 'link'; content: string; url?: string }[] {
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
	<div class="border-b shadow-sm {variantClasses[variant]}">
		<div class="max-w-14xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
			<div class="flex items-center justify-center gap-4">
				<div class="flex items-center justify-center gap-3 flex-1">
					<div class="flex-shrink-0">
						<Icon icon={iconMap[variant]} class="w-5 h-5" />
					</div>
					<div class="text-sm font-medium text-center">
						{#each messageParts as part}
							{#if part.type === 'link'}
								<a
									href={part.url}
									class="underline underline-offset-2 transition-colors {linkColorClasses[variant]}"
									target={part.url?.startsWith('http') ? '_blank' : undefined}
									rel={part.url?.startsWith('http') ? 'noopener noreferrer' : undefined}
								>
									{part.content}
								</a>
							{:else}
								<span>{part.content}</span>
							{/if}
						{/each}
					</div>
				</div>
				{#if dismissible}
					<button
						onclick={dismiss}
						class="flex-shrink-0 p-1.5 rounded-md hover:bg-black/10 dark:hover:bg-white/10 transition-all hover:scale-110"
						aria-label="Dismiss banner"
					>
						<Icon icon="material-symbols:close" class="w-4 h-4" />
					</button>
				{/if}
			</div>
		</div>
	</div>
{/if}
