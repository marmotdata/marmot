<script lang="ts">
	import Icon from '@iconify/svelte';

	interface MentionItem {
		type: string;
		id: string;
		name: string;
		username?: string;
		profile_picture?: string;
	}

	interface Props {
		items: Array<MentionItem>;
		command: (item: { id: string; label: string; type: string }) => void;
	}

	let { items = [], command }: Props = $props();

	let selectedIndex = $state(0);

	$effect(() => {
		if (items.length) {
			selectedIndex = Math.min(selectedIndex, items.length - 1);
		}
	});

	$effect(() => {
		// Reset selection when items change
		selectedIndex = 0;
	});

	export function onKeyDown(event: KeyboardEvent): boolean {
		if (event.key === 'ArrowUp') {
			selectedIndex = (selectedIndex + items.length - 1) % items.length;
			return true;
		}
		if (event.key === 'ArrowDown') {
			selectedIndex = (selectedIndex + 1) % items.length;
			return true;
		}
		if (event.key === 'Enter') {
			selectItem(selectedIndex);
			return true;
		}
		return false;
	}

	function selectItem(index: number) {
		const item = items[index];
		if (item) {
			// Always use the full name as the display label
			command({ id: item.id, label: item.name, type: item.type });
		}
	}

	function getDisplayName(item: MentionItem): string {
		if (item.type === 'user' && item.username) {
			return `@${item.username}`;
		}
		return item.name;
	}
</script>

<div
	class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg overflow-hidden max-h-64 overflow-y-auto"
>
	{#if items.length === 0}
		<div class="px-3 py-2 text-sm text-gray-500 dark:text-gray-400">No results found</div>
	{:else}
		{#each items as item, index (item.id)}
			<button
				type="button"
				class="w-full px-3 py-2 flex items-center gap-2 text-left hover:bg-gray-100 dark:hover:bg-gray-700 {index ===
				selectedIndex
					? 'bg-gray-100 dark:bg-gray-700'
					: ''}"
				onclick={() => selectItem(index)}
			>
				{#if item.profile_picture}
					<img
						src={item.profile_picture}
						alt={item.name}
						class="w-6 h-6 rounded-full object-cover"
					/>
				{:else}
					<div
						class="w-6 h-6 rounded-full {item.type === 'team'
							? 'bg-blue-100 dark:bg-blue-900/30'
							: 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30'} flex items-center justify-center"
					>
						<Icon
							icon={item.type === 'team' ? 'mdi:account-group' : 'mdi:account'}
							class="w-4 h-4 {item.type === 'team'
								? 'text-blue-600'
								: 'text-earthy-terracotta-600'}"
						/>
					</div>
				{/if}
				<div class="flex-1 min-w-0">
					<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
						{item.name}
					</div>
					{#if item.type === 'user' && item.username}
						<div class="text-xs text-gray-500 dark:text-gray-400 truncate">@{item.username}</div>
					{:else if item.type === 'team'}
						<div class="text-xs text-gray-500 dark:text-gray-400 truncate">Team</div>
					{/if}
				</div>
			</button>
		{/each}
	{/if}
</div>
