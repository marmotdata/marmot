<script lang="ts">
	import { m } from '$lib/paraglide/messages';

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
</script>

<div
	class="bg-mm-raised border border-mm-rule rounded-md shadow-float overflow-hidden max-h-64 overflow-y-auto py-space-1"
	role="listbox"
>
	{#if items.length === 0}
		<div class="px-space-3 py-space-2 text-dense text-mm-text-2">{m.common_no_results()}</div>
	{:else}
		{#each items as item, index (item.id)}
			<button
				type="button"
				role="option"
				aria-selected={index === selectedIndex}
				class="mention-option w-full px-space-3 py-space-2 flex items-center gap-space-2 text-left text-mm-text"
				class:is-selected={index === selectedIndex}
				onclick={() => selectItem(index)}
			>
				{#if item.profile_picture}
					<img src={item.profile_picture} alt="" class="w-6 h-6 rounded-full object-cover" />
				{:else}
					<mm-icon
						name={item.type === 'team' ? 'groups' : 'person'}
						aria-hidden="true"
						class="w-6 h-6 flex items-center justify-center text-mm-text-2"
					></mm-icon>
				{/if}
				<div class="flex-1 min-w-0">
					<div class="text-dense font-semibold truncate">
						{item.name}
					</div>
					{#if item.type === 'user' && item.username}
						<div class="text-caption text-mm-text-2 truncate">@{item.username}</div>
					{:else if item.type === 'team'}
						<div class="text-caption text-mm-text-2 truncate">
							{m.editor_mention_team_label()}
						</div>
					{/if}
				</div>
			</button>
		{/each}
	{/if}
</div>

<style>
	.mention-option {
		transition: background-color var(--mm-dur-press) var(--mm-ease);
	}
	.mention-option:hover,
	.mention-option.is-selected {
		background: var(--mm-selected);
	}
	.mention-option:focus-visible {
		outline: 2px solid var(--mm-text);
		outline-offset: -2px;
	}
</style>
