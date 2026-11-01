<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';

	export interface AssetOwner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string;
		email?: string;
		profile_picture?: string;
	}

	// "Add owner" in the page header: a popover that searches people and teams, with a Link button per result that applies at once.
	let {
		owners,
		onadd
	}: { owners: AssetOwner[]; onadd: (owner: AssetOwner) => void | Promise<void> } = $props();

	let open = $state(false);
	let query = $state('');
	let results = $state<AssetOwner[]>([]);
	let loading = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	let ownerKeys = $derived(new Set(owners.map((o) => `${o.type}-${o.id}`)));

	function onInput(value: string) {
		query = value;
		clearTimeout(timer);
		timer = setTimeout(search, 200);
	}

	async function search() {
		loading = true;
		try {
			const response = await fetchApi(`/owners/search?q=${encodeURIComponent(query)}&limit=20`);
			if (response.ok) {
				const data = await response.json();
				results = data.owners || [];
			}
		} catch {
			results = [];
		} finally {
			loading = false;
		}
	}

	function onToggle(e: CustomEvent<{ open: boolean }>) {
		open = e.detail.open;
		if (open && results.length === 0) search();
	}
</script>

<mm-popover label={m.asset_page_add_owner()} {open} onmm-toggle={onToggle}>
	<mm-button slot="trigger" variant="ghost" size="sm" icon="person-add"
		>{m.asset_page_add_owner()}</mm-button
	>
	<div class="mm-stack" style="--mm-gap: var(--space-2); min-inline-size: 18rem">
		<mm-search-field
			label={m.asset_page_find_owner()}
			hide-label
			placeholder={m.asset_page_find_owner()}
			value={query}
			autofocus
			onmm-input={(e: CustomEvent<{ value: string }>) => onInput(e.detail.value)}
		></mm-search-field>
		{#if loading && results.length === 0}
			<mm-skeleton count={3}></mm-skeleton>
		{:else if results.length === 0}
			<mm-text variant="dense" tone="secondary" block>{m.asset_page_no_owners_found()}</mm-text>
		{:else}
			<mm-list dense aria-label={m.asset_page_owner_results()}>
				{#each results as o (`${o.type}-${o.id}`)}
					{@const added = ownerKeys.has(`${o.type}-${o.id}`)}
					<mm-list-item secondary={o.type === 'team' ? m.asset_page_team() : o.email || o.username}>
						<mm-avatar
							slot="start"
							name={o.name}
							src={o.profile_picture}
							kind={o.type === 'team' ? 'team' : 'person'}
							size="20"
						></mm-avatar>
						{o.name}
						{#if added}
							<mm-text slot="end" variant="caption" tone="secondary"
								>{m.asset_page_owner_added()}</mm-text
							>
						{:else}
							<mm-button slot="end" size="sm" onclick={() => onadd(o)}
								>{m.asset_page_add_button()}</mm-button
							>
						{/if}
					</mm-list-item>
				{/each}
			</mm-list>
		{/if}
	</div>
</mm-popover>
