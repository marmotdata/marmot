<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Avatar from '$components/user/Avatar.svelte';
	import SearchListbox from '$components/ui/SearchListbox.svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { INPUT } from '$lib/ui/classes';

	export interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string;
		email?: string;
		profile_picture?: string;
	}

	const inputClass = INPUT;

	let {
		members = $bindable([]),
		roles = $bindable({})
	}: {
		members?: Owner[];
		roles?: Record<string, 'owner' | 'member'>;
	} = $props();

	const listId = `team-member-picker-${Math.random().toString(36).slice(2, 8)}`;

	let query = $state('');
	let results = $state<Owner[]>([]);
	let loading = $state(false);
	let active = $state(0);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let listbox = $state<{ handleKeydown: (e: KeyboardEvent) => void } | null>(null);
	let showResults = $derived(query.trim().length >= 2);
	let available = $derived(results.filter((r) => !members.some((member) => member.id === r.id)));

	// Reset keyboard focus whenever the candidate list changes, so a stale highlight does not point at a row that moved
	$effect(() => {
		void available;
		active = 0;
	});

	function scheduleSearch() {
		clearTimeout(timer);
		const q = query.trim();
		if (q.length < 2) {
			results = [];
			loading = false;
			return;
		}
		loading = true;
		timer = setTimeout(async () => {
			try {
				const res = await fetchApi(`/owners/search?q=${encodeURIComponent(q)}&limit=50`);
				if (res.ok) {
					const data = await res.json();
					const all: Owner[] = data.owners ?? [];
					results = all.filter((o) => o.type === 'user');
				} else {
					results = [];
				}
			} catch {
				results = [];
			} finally {
				loading = false;
			}
		}, 250);
	}

	function clearSearch() {
		query = '';
		results = [];
	}

	function addMember(owner: Owner) {
		if (members.some((member) => member.id === owner.id)) return;
		members = [...members, owner];
		if (!roles[owner.id]) {
			roles = { ...roles, [owner.id]: 'member' };
		}
		clearSearch();
	}

	function toggleRole(id: string) {
		roles = { ...roles, [id]: roles[id] === 'owner' ? 'member' : 'owner' };
	}

	function removeMember(id: string) {
		members = members.filter((member) => member.id !== id);
		const next = { ...roles };
		delete next[id];
		roles = next;
	}

	// Enter in the search box picks a result rather than submitting the whole form, and Escape clears the search without also closing the blade it sits in
	function onInputKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') e.preventDefault();
		if (e.key === 'Escape' && query) e.stopPropagation();
		listbox?.handleKeydown(e);
	}
</script>

<div class="relative">
	<IconifyIcon
		icon="material-symbols:search"
		class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
	/>
	<input
		type="text"
		bind:value={query}
		oninput={scheduleSearch}
		onkeydown={onInputKeydown}
		role="combobox"
		aria-expanded={showResults}
		aria-controls={listId}
		placeholder={m.teams_member_search_placeholder()}
		aria-label={m.teams_member_search_placeholder()}
		class="{inputClass} pl-9 pr-9"
	/>
	{#if query}
		<button
			type="button"
			onclick={clearSearch}
			class="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700"
			aria-label={m.teams_clear_search_aria()}
		>
			<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
		</button>
	{/if}

	{#if showResults}
		{#if loading}
			<div
				class="absolute z-10 mt-1 max-h-64 w-full overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
			>
				<div
					class="flex items-center justify-center gap-2 px-4 py-6 text-sm text-gray-500 dark:text-gray-400"
				>
					<div
						class="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-earthy-terracotta-600"
					></div>
					{m.teams_searching()}
				</div>
			</div>
		{:else}
			<SearchListbox
				{listId}
				results={available}
				bind:active
				bind:this={listbox}
				getKey={(owner) => owner.id}
				onSelect={addMember}
				onEscape={clearSearch}
				class="absolute z-10 mt-1 max-h-64 w-full overflow-hidden overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
			>
				{#snippet row(owner, i, isActive)}
					<button
						type="button"
						onclick={() => addMember(owner)}
						onmouseenter={() => (active = i)}
						class="flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors
							{isActive
							? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
							: 'hover:bg-gray-50 dark:hover:bg-gray-700'}"
					>
						<Avatar name={owner.name} profilePicture={owner.profile_picture} size="sm" />
						<div class="min-w-0 flex-1">
							<div class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
								{owner.name}
							</div>
							{#if owner.email}
								<div class="truncate text-xs text-gray-500 dark:text-gray-400">
									{owner.email}
								</div>
							{/if}
						</div>
						<IconifyIcon
							icon="material-symbols:add"
							class="h-4 w-4 shrink-0 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
						/>
					</button>
				{/snippet}
				{#snippet empty()}
					{results.length === 0 ? m.teams_no_users_found() : m.teams_all_users_added()}
				{/snippet}
			</SearchListbox>
		{/if}
	{/if}
</div>

{#if members.length === 0}
	<div
		class="flex items-center gap-3 rounded-lg border border-dashed border-gray-200 bg-gray-50 px-4 py-4 text-sm text-gray-500 dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-400"
	>
		<IconifyIcon icon="material-symbols:person-add-outline" class="h-5 w-5 text-gray-400" />
		<span>{m.teams_no_members_yet()}</span>
	</div>
{:else}
	<ul
		class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800/50"
	>
		{#each members as owner (owner.id)}
			{@const role = roles[owner.id] ?? 'member'}
			<li
				class="flex items-center gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-gray-700/60"
			>
				<Avatar name={owner.name} profilePicture={owner.profile_picture} size="md" />
				<div class="min-w-0 flex-1">
					<div class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
						{owner.name}
					</div>
					{#if owner.email}
						<div class="truncate text-xs text-gray-500 dark:text-gray-400">
							{owner.email}
						</div>
					{/if}
				</div>
				<button
					type="button"
					onclick={() => toggleRole(owner.id)}
					class="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors
						{role === 'owner'
						? 'bg-earthy-terracotta-100 text-earthy-terracotta-700 hover:bg-earthy-terracotta-200 dark:bg-earthy-terracotta-900/40 dark:text-earthy-terracotta-300'
						: 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'}"
					title={m.teams_toggle_role_title()}
				>
					<IconifyIcon
						icon={role === 'owner' ? 'material-symbols:star' : 'material-symbols:person-outline'}
						class="h-3.5 w-3.5"
					/>
					{role === 'owner' ? m.common_owner() : m.teams_role_member()}
				</button>
				<button
					type="button"
					onclick={() => removeMember(owner.id)}
					class="-m-1 p-1 text-gray-400 hover:text-red-500 dark:hover:text-red-400"
					aria-label={m.common_remove()}
				>
					<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
				</button>
			</li>
		{/each}
	</ul>
{/if}
