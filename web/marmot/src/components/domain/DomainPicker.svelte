<script lang="ts">
	import Icon from '@iconify/svelte';
	import type { DomainOption } from '$lib/domains/options';

	let {
		value = $bindable(''),
		options,
		label,
		noneLabel,
		disabled = false,
		variant = 'field',
		id,
		onSelect
	}: {
		/** Selected domain id; '' selects the none option. */
		value?: string;
		options: DomainOption[];
		/** Accessible name of the picker. */
		label: string;
		/** Adds a first option with value '' (no parent, all domains, …). */
		noneLabel?: string;
		disabled?: boolean;
		/** field: form control width; chip: compact trigger for entity pages. */
		variant?: 'field' | 'chip';
		/** Id of the trigger button, for an external <label for>. */
		id?: string;
		/** Makes the picker controlled: the parent applies the change and updates value. */
		onSelect?: (id: string) => void;
	} = $props();

	let open = $state(false);
	let root = $state<HTMLDivElement>();
	let list = $state<HTMLDivElement>();

	type Item = { id: string; name: string; path: string; depth: number; italic: boolean };
	const items = $derived<Item[]>([
		...(noneLabel !== undefined
			? [{ id: '', name: noneLabel, path: noneLabel, depth: 0, italic: false }]
			: []),
		...options.map((o) => ({
			id: o.id,
			name: o.name,
			path: o.path,
			depth: o.depth,
			italic: o.unassigned
		}))
	]);
	const selected = $derived(items.find((i) => i.id === value));

	// With onSelect the parent owns the value, so a rejected change does not show as applied.
	function choose(id: string) {
		open = false;
		if (id === value) return;
		if (onSelect) onSelect(id);
		else value = id;
	}

	function toggle(event: MouseEvent) {
		event.stopPropagation();
		open = !open;
		if (open) {
			// Put focus on the current option so arrow keys start from it.
			queueMicrotask(() =>
				list?.querySelector<HTMLElement>('[aria-selected="true"], [role="option"]')?.focus()
			);
		}
	}

	function handleListKeydown(event: KeyboardEvent) {
		if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
		event.preventDefault();
		const buttons = [...(list?.querySelectorAll<HTMLElement>('[role="option"]') ?? [])];
		const index = buttons.indexOf(document.activeElement as HTMLElement);
		const next = event.key === 'ArrowDown' ? index + 1 : index - 1;
		buttons[Math.max(0, Math.min(buttons.length - 1, next))]?.focus();
	}

	function handleWindowClick(event: MouseEvent) {
		if (!root?.contains(event.target as Node)) open = false;
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') open = false;
	}
</script>

<svelte:window onclick={handleWindowClick} onkeydown={handleWindowKeydown} />

<div class="relative {variant === 'field' ? 'block w-full' : 'inline-flex'}" bind:this={root}>
	<button
		{id}
		type="button"
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-label={label}
		{disabled}
		onclick={toggle}
		class="inline-flex items-center gap-2 text-left text-sm focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 disabled:opacity-60 {variant ===
		'field'
			? 'w-full justify-between rounded-md border border-gray-300 bg-white px-3 py-2 text-gray-900 focus:border-transparent dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100'
			: 'rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600'}"
	>
		<span class="min-w-0 truncate {selected?.italic ? 'italic' : ''}">{selected?.path ?? ''}</span>
		<Icon
			icon="material-symbols:keyboard-arrow-down"
			class="shrink-0 text-gray-500 transition-transform dark:text-gray-400 {variant === 'field'
				? 'h-4 w-4'
				: 'h-3.5 w-3.5'} {open ? 'rotate-180' : ''}"
		/>
	</button>
	{#if open}
		<div
			bind:this={list}
			role="listbox"
			tabindex="-1"
			aria-label={label}
			onkeydown={handleListKeydown}
			class="absolute left-0 top-full z-50 mt-1 max-h-72 overflow-y-auto overscroll-contain rounded-md border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800 {variant ===
			'field'
				? 'w-full'
				: 'w-max min-w-full max-w-sm'}"
		>
			{#each items as item (item.id)}
				<button
					type="button"
					role="option"
					aria-selected={item.id === value}
					onclick={(event) => {
						event.stopPropagation();
						choose(item.id);
					}}
					class="flex w-full items-center justify-between gap-3 py-2 pr-3 text-left text-sm transition-colors focus:bg-gray-100 focus:outline-none dark:focus:bg-gray-700 {item.id ===
					value
						? 'font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
						: 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'}"
					style="padding-left: {0.75 + item.depth * 1}rem"
				>
					<span class="flex min-w-0 items-center gap-2">
						{#if item.id}
							<Icon
								icon={item.italic
									? 'material-symbols:inbox-outline-rounded'
									: 'material-symbols:account-tree-outline-rounded'}
								class="h-4 w-4 shrink-0 text-gray-400"
							/>
						{/if}
						<span class="truncate {item.italic ? 'italic' : ''}">{item.name}</span>
					</span>
					{#if item.id === value}
						<span
							class="h-1.5 w-1.5 shrink-0 rounded-full bg-earthy-terracotta-700"
							aria-hidden="true"
						></span>
					{/if}
				</button>
			{/each}
		</div>
	{/if}
</div>
