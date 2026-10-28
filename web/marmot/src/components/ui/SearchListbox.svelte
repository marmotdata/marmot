<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';

	// A reusable accessible dropdown list for search-driven pickers (role=listbox/option plus
	// keyboard navigation and auto-scroll of the highlighted row). Fetching, debouncing and the row
	// markup stay with the caller; this only standardises the parts that make the widget accessible.
	let {
		listId,
		results,
		loading = false,
		active = $bindable(0),
		onSelect,
		onEscape,
		getKey,
		row,
		empty,
		class: listClass = ''
	}: {
		listId: string;
		results: T[];
		loading?: boolean;
		active?: number;
		onSelect: (item: T) => void;
		onEscape?: () => void;
		getKey: (item: T) => string;
		row: Snippet<[T, number, boolean]>;
		empty: Snippet;
		class?: string;
	} = $props();

	let listEl: HTMLUListElement | null = null;

	// Keeps the highlighted row in view when arrow keys move it off-screen, the way a native select does
	$effect(() => {
		listEl?.querySelector(`[data-option-index="${active}"]`)?.scrollIntoView({ block: 'nearest' });
	});

	// Exposed so the input that owns focus can forward its keydown event here via bind:this
	export function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			active = Math.min(active + 1, results.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			active = Math.max(active - 1, 0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (results[active]) onSelect(results[active]);
		} else if (e.key === 'Escape') {
			onEscape?.();
		}
	}
</script>

<ul id={listId} role="listbox" bind:this={listEl} class={listClass}>
	{#each results as item, i (getKey(item))}
		<li id="{listId}-opt-{i}" role="option" aria-selected={i === active} data-option-index={i}>
			{@render row(item, i, i === active)}
		</li>
	{:else}
		{#if !loading}
			<li class="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
				{@render empty()}
			</li>
		{/if}
	{/each}
</ul>
