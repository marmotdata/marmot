<script lang="ts">
	import { onMount, tick } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { listRoles } from '$lib/roles/api';
	import type { Role } from '$lib/roles/types';
	import { m } from '$lib/paraglide/messages';
	import { BTN_SECONDARY } from '$lib/ui/classes';
	import Alert from '$components/ui/Alert.svelte';
	import Skeleton from '$components/ui/Skeleton.svelte';

	// Picks roles for a member or service account. An instance has dozens of roles and a principal holds a few, so the few are shown as chips and the rest are reached through a search, rather than a checkbox per role.
	interface Props {
		selectedIds: string[];
		onChange: (ids: string[]) => void;
		roles?: Role[];
		placeholder?: string;
		emptyMessage?: string;
		/** Kept for callers that still pass it; the picker no longer pages. */
		pageSize?: number;
	}

	let {
		selectedIds,
		onChange,
		roles: providedRoles,
		placeholder = m.ui_roleselector_search_placeholder(),
		emptyMessage = m.ui_roleselector_empty()
	}: Props = $props();

	let internalRoles = $state<Role[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let open = $state(false);
	let query = $state('');
	let highlighted = $state(0);
	let root: HTMLDivElement | undefined = $state();
	let input: HTMLInputElement | undefined = $state();
	const listId = $props.id();

	let allRoles = $derived(providedRoles ?? internalRoles);
	let selected = $derived(
		selectedIds.map((id) => allRoles.find((r) => r.id === id)).filter((r): r is Role => !!r)
	);
	let choices = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return allRoles
			.filter((r) => !selectedIds.includes(r.id))
			.filter(
				(r) =>
					!q || r.name.toLowerCase().includes(q) || (r.description ?? '').toLowerCase().includes(q)
			);
	});

	onMount(async () => {
		if (providedRoles) return;
		try {
			loading = true;
			internalRoles = await listRoles();
		} catch (err) {
			error = err instanceof Error ? err.message : m.ui_roles_load_error();
		} finally {
			loading = false;
		}
	});

	async function openPicker() {
		open = true;
		query = '';
		highlighted = 0;
		await tick();
		input?.focus();
	}

	function add(role: Role) {
		onChange([...selectedIds, role.id]);
		query = '';
		highlighted = 0;
		input?.focus();
	}

	function remove(id: string) {
		onChange(selectedIds.filter((r) => r !== id));
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
			if (choices[highlighted]) add(choices[highlighted]);
		} else if (e.key === 'Escape') {
			// The picker closes first; the blade around it only on a second Escape
			e.stopPropagation();
			open = false;
		}
	}

	function onWindowClick(e: MouseEvent) {
		if (open && root && !root.contains(e.target as Node)) open = false;
	}
</script>

<svelte:window onclick={onWindowClick} />

{#if loading}
	<Skeleton class="h-9" />
{:else if error}
	<Alert tone="error">{error}</Alert>
{:else}
	<div class="relative" bind:this={root}>
		<div class="flex flex-wrap items-center gap-1.5">
			{#each selected as role (role.id)}
				<span
					class="inline-flex items-center gap-1 rounded-md border border-gray-200 bg-white py-0.5 pl-2 pr-1 text-sm text-gray-800 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
					title={role.description}
				>
					{#if role.is_system}
						<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3 text-gray-400" />
					{/if}
					{role.name}
					<button
						type="button"
						onclick={() => remove(role.id)}
						class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-700 dark:hover:text-gray-200"
						aria-label={m.ui_roleselector_remove({ role: role.name })}
					>
						<IconifyIcon icon="material-symbols:close" class="h-3.5 w-3.5" />
					</button>
				</span>
			{:else}
				<span class="text-sm text-gray-500 dark:text-gray-400"
					>{m.ui_roleselector_none_selected()}</span
				>
			{/each}
			<button
				type="button"
				class="{BTN_SECONDARY} px-2 py-1 text-xs"
				aria-haspopup="listbox"
				aria-expanded={open}
				onclick={openPicker}
			>
				<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
				{m.ui_roleselector_add()}
			</button>
		</div>

		{#if open}
			<div
				class="absolute left-0 z-30 mt-2 w-full max-w-md overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
			>
				<div class="border-b border-gray-100 p-2 dark:border-gray-700/60">
					<input
						bind:this={input}
						bind:value={query}
						oninput={() => (highlighted = 0)}
						onkeydown={onKeydown}
						type="search"
						{placeholder}
						aria-label={placeholder}
						aria-controls={listId}
						class="w-full rounded-md border-0 bg-gray-50 px-2.5 py-1.5 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:bg-gray-900 dark:text-gray-100"
					/>
				</div>
				<ul id={listId} class="max-h-72 overflow-y-auto py-1" role="listbox">
					{#each choices as role, i (role.id)}
						<li role="option" aria-selected={i === highlighted}>
							<button
								type="button"
								onclick={() => add(role)}
								onmouseenter={() => (highlighted = i)}
								class="block w-full px-3 py-2 text-left {i === highlighted
									? 'bg-gray-100 dark:bg-gray-700/60'
									: ''}"
							>
								<span
									class="flex items-center gap-1.5 text-sm font-medium text-gray-900 dark:text-gray-100"
								>
									{role.name}
									{#if role.is_system}
										<IconifyIcon
											icon="material-symbols:lock-outline"
											class="h-3 w-3 text-gray-400"
										/>
									{/if}
								</span>
								{#if role.description}
									<span class="block truncate text-xs text-gray-500 dark:text-gray-400"
										>{role.description}</span
									>
								{/if}
							</button>
						</li>
					{:else}
						<li class="px-3 py-3 text-sm text-gray-500 dark:text-gray-400">
							{query.trim() ? m.ui_roleselector_no_match({ query }) : emptyMessage}
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	</div>
{/if}
