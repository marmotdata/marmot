<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { listRoles } from '$lib/roles/api';
	import type { Role } from '$lib/roles/types';

	interface Props {
		selectedIds: string[];
		onChange: (ids: string[]) => void;
		roles?: Role[];
		placeholder?: string;
		emptyMessage?: string;
		pageSize?: number;
	}

	let {
		selectedIds,
		onChange,
		roles: providedRoles,
		placeholder = 'Search roles by name or description...',
		emptyMessage = 'No roles found.',
		pageSize = 6
	}: Props = $props();

	let internalRoles = $state<Role[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let query = $state('');
	let page = $state(1);

	let allRoles = $derived(providedRoles ?? internalRoles);

	let filtered = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return allRoles;
		return allRoles.filter(
			(r) => r.name.toLowerCase().includes(q) || (r.description ?? '').toLowerCase().includes(q)
		);
	});

	let totalPages = $derived(Math.max(1, Math.ceil(filtered.length / pageSize)));

	// Clamp page whenever filtered length or pageSize changes
	$effect(() => {
		if (page > totalPages) page = totalPages;
	});

	// Reset to page 1 when the search query changes
	$effect(() => {
		const _q = query;
		page = 1;
	});

	let visible = $derived(filtered.slice((page - 1) * pageSize, (page - 1) * pageSize + pageSize));

	let showingFrom = $derived(filtered.length === 0 ? 0 : (page - 1) * pageSize + 1);
	let showingTo = $derived(Math.min(page * pageSize, filtered.length));

	onMount(async () => {
		if (providedRoles) return;
		try {
			loading = true;
			internalRoles = await listRoles();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load roles';
		} finally {
			loading = false;
		}
	});

	function toggle(id: string) {
		const next = selectedIds.includes(id)
			? selectedIds.filter((r) => r !== id)
			: [...selectedIds, id];
		onChange(next);
	}

	function clearSelection() {
		onChange([]);
	}

	function prev() {
		if (page > 1) page -= 1;
	}
	function next() {
		if (page < totalPages) page += 1;
	}
</script>

<div class="space-y-3">
	<!-- Search bar -->
	<div class="relative">
		<IconifyIcon
			icon="material-symbols:search"
			class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400 pointer-events-none"
		/>
		<input
			type="text"
			bind:value={query}
			{placeholder}
			class="w-full pl-9 pr-9 py-2.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
		/>
		{#if query}
			<button
				type="button"
				onclick={() => (query = '')}
				class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
				aria-label="Clear search"
			>
				<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
			</button>
		{/if}
	</div>

	<!-- Selection summary -->
	<div class="flex items-center justify-between text-xs">
		<span class="text-gray-500 dark:text-gray-400">
			{#if filtered.length === 0}
				No results
			{:else}
				<span class="font-medium text-gray-700 dark:text-gray-300">{showingFrom}–{showingTo}</span>
				of <span class="font-medium text-gray-700 dark:text-gray-300">{filtered.length}</span>
				{filtered.length === 1 ? 'role' : 'roles'}{query ? ' matched' : ''}
			{/if}
		</span>
		<div class="flex items-center gap-3">
			<span class="text-gray-500 dark:text-gray-400">
				<span class="font-medium text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
					>{selectedIds.length}</span
				>
				selected
			</span>
			{#if selectedIds.length > 0}
				<button
					type="button"
					class="text-xs text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
					onclick={clearSelection}
				>
					Clear
				</button>
			{/if}
		</div>
	</div>

	<!-- List -->
	<div
		class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-white dark:bg-gray-800/50"
	>
		{#if loading}
			<div class="flex justify-center p-8">
				<div
					class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"
				></div>
			</div>
		{:else if error}
			<div class="p-4 text-sm text-red-600 dark:text-red-400">{error}</div>
		{:else if filtered.length === 0}
			<div class="p-8 text-sm text-gray-500 dark:text-gray-400 text-center">
				{query ? `No roles match "${query}".` : emptyMessage}
			</div>
		{:else}
			<ul class="divide-y divide-gray-100 dark:divide-gray-700/60">
				{#each visible as role (role.id)}
					{@const isSelected = selectedIds.includes(role.id)}
					<li>
						<label
							class="flex items-center gap-3 px-4 py-3 cursor-pointer transition-colors
								{isSelected
								? 'bg-earthy-terracotta-50/60 dark:bg-earthy-terracotta-900/15 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/25'
								: 'hover:bg-earthy-brown-50 dark:hover:bg-gray-800'}"
						>
							<input
								type="checkbox"
								checked={isSelected}
								onchange={() => toggle(role.id)}
								class="h-4 w-4 rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 flex-shrink-0"
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2 flex-wrap">
									<span class="text-sm font-medium text-gray-900 dark:text-gray-100">
										{role.name}
									</span>
									{#if role.is_system}
										<span
											class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200"
										>
											<IconifyIcon icon="material-symbols:lock" class="h-2.5 w-2.5" />
											system
										</span>
									{/if}
								</div>
								{#if role.description}
									<p
										class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate"
										title={role.description}
									>
										{role.description}
									</p>
								{/if}
							</div>
							{#if isSelected}
								<IconifyIcon
									icon="material-symbols:check-circle"
									class="h-4 w-4 text-earthy-terracotta-600 dark:text-earthy-terracotta-400 flex-shrink-0"
								/>
							{/if}
						</label>
					</li>
				{/each}
			</ul>

			<!-- Pagination footer -->
			{#if totalPages > 1}
				<div
					class="flex items-center justify-between px-4 py-2.5 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/40"
				>
					<span class="text-xs text-gray-500 dark:text-gray-400">
						Page <span class="font-medium text-gray-700 dark:text-gray-300">{page}</span>
						of <span class="font-medium text-gray-700 dark:text-gray-300">{totalPages}</span>
					</span>
					<div class="flex items-center gap-1">
						<button
							type="button"
							onclick={prev}
							disabled={page <= 1}
							class="p-1.5 rounded-md text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed"
							aria-label="Previous page"
						>
							<IconifyIcon icon="material-symbols:chevron-left" class="h-4 w-4" />
						</button>
						<button
							type="button"
							onclick={next}
							disabled={page >= totalPages}
							class="p-1.5 rounded-md text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed"
							aria-label="Next page"
						>
							<IconifyIcon icon="material-symbols:chevron-right" class="h-4 w-4" />
						</button>
					</div>
				</div>
			{/if}
		{/if}
	</div>
</div>
