<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { tick } from 'svelte';
	import { FILTER_PILL, FILTER_PILL_ON, FILTER_PILL_OFF } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';

	// A filter over a long list of values, such as roles: one pill that names the choice and opens a searchable list, rather than a pill per value.
	let {
		label,
		options,
		value,
		onChange,
		anyLabel,
		searchPlaceholder
	}: {
		label: string;
		options: { id: string; label: string; hint?: string }[];
		value: string;
		onChange: (id: string) => void;
		anyLabel: string;
		searchPlaceholder: string;
	} = $props();

	let open = $state(false);
	let query = $state('');
	let highlighted = $state(0);
	let root: HTMLDivElement | undefined = $state();
	let input: HTMLInputElement | undefined = $state();

	let selected = $derived(options.find((o) => o.id === value));
	let choices = $derived.by(() => {
		const q = query.trim().toLowerCase();
		const matching = q
			? options.filter((o) => `${o.label} ${o.hint ?? ''}`.toLowerCase().includes(q))
			: options;
		return [{ id: '', label: anyLabel }, ...matching];
	});

	async function toggle() {
		open = !open;
		if (open) {
			query = '';
			highlighted = Math.max(
				0,
				choices.findIndex((c) => c.id === value)
			);
			await tick();
			input?.focus();
		}
	}

	function choose(id: string) {
		onChange(id);
		open = false;
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			highlighted = Math.min(highlighted + 1, choices.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			highlighted = Math.max(highlighted - 1, 0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (choices[highlighted]) choose(choices[highlighted].id);
		} else if (e.key === 'Escape') {
			open = false;
		}
	}

	function onWindowClick(e: MouseEvent) {
		if (open && root && !root.contains(e.target as Node)) open = false;
	}
</script>

<svelte:window onclick={onWindowClick} />

<div class="relative" bind:this={root}>
	<button
		type="button"
		class="{FILTER_PILL} {value ? FILTER_PILL_ON : FILTER_PILL_OFF}"
		aria-haspopup="listbox"
		aria-expanded={open}
		onclick={toggle}
	>
		<span class={value ? '' : 'text-gray-500 dark:text-gray-400'}>{label}:</span>
		{selected?.label ?? anyLabel}
		<IconifyIcon icon="material-symbols:keyboard-arrow-down" class="h-3.5 w-3.5" />
	</button>
	{#if open}
		<div
			class="absolute left-0 z-30 mt-1.5 w-64 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
		>
			<div class="border-b border-gray-100 p-2 dark:border-gray-700/60">
				<input
					bind:this={input}
					bind:value={query}
					oninput={() => (highlighted = 0)}
					onkeydown={onKeydown}
					type="search"
					placeholder={searchPlaceholder}
					aria-label={searchPlaceholder}
					class="w-full rounded-md border-0 bg-gray-50 px-2.5 py-1.5 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:bg-gray-900 dark:text-gray-100"
				/>
			</div>
			<ul class="max-h-72 overflow-y-auto py-1" role="listbox" aria-label={label}>
				{#each choices as choice, i (choice.id)}
					<li role="option" aria-selected={choice.id === value}>
						<button
							type="button"
							onclick={() => choose(choice.id)}
							onmouseenter={() => (highlighted = i)}
							class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm {i === highlighted
								? 'bg-gray-100 dark:bg-gray-700/60'
								: ''} {choice.id === ''
								? 'text-gray-500 dark:text-gray-400'
								: 'text-gray-800 dark:text-gray-200'}"
						>
							<span class="min-w-0 flex-1 truncate">{choice.label}</span>
							{#if choice.id === value}
								<IconifyIcon
									icon="material-symbols:check"
									class="h-4 w-4 text-earthy-terracotta-600"
								/>
							{/if}
						</button>
					</li>
				{/each}
				{#if choices.length === 1 && query.trim()}
					<li class="px-3 py-2 text-sm text-gray-500 dark:text-gray-400">
						{m.common_no_results()}
					</li>
				{/if}
			</ul>
		</div>
	{/if}
</div>
