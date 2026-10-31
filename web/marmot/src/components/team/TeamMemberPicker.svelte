<script lang="ts">
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import type { OwnerResult } from '$lib/teams/types';

	// Finds people to add to a team. Each result has its own Add button that acts at once, and people already chosen are left out of the results, so chosen and unchosen never mix in one list.
	let {
		exclude = [],
		busyId = null,
		onAdd
	}: {
		exclude?: string[];
		busyId?: string | null;
		onAdd: (person: OwnerResult) => void;
	} = $props();

	let query = $state('');
	let results = $state<OwnerResult[]>([]);
	let searching = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let available = $derived(results.filter((r) => !exclude.includes(r.id)));
	let searched = $derived(query.trim().length >= 2);

	function onInput(value: string) {
		query = value;
		clearTimeout(timer);
		const q = value.trim();
		if (q.length < 2) {
			results = [];
			searching = false;
			return;
		}
		searching = true;
		timer = setTimeout(async () => {
			try {
				const res = await fetchApi(`/owners/search?q=${encodeURIComponent(q)}&limit=50`);
				const data = res.ok ? await res.json() : { owners: [] };
				results = ((data.owners ?? []) as OwnerResult[]).filter((o) => o.type === 'user');
			} catch {
				results = [];
			} finally {
				searching = false;
			}
		}, 250);
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-3)">
	<mm-search-field
		label={m.teams_find_people()}
		placeholder={m.teams_find_people_placeholder()}
		value={query}
		onmm-input={(e: CustomEvent<MmValueDetail>) => onInput(e.detail.value)}
		onmm-search={(e: Event) => e.preventDefault()}
	></mm-search-field>
	{#if searched}
		{#if searching && available.length === 0}
			<mm-text tone="secondary" role="status">{m.teams_searching_people()}</mm-text>
		{:else if available.length === 0}
			<mm-text tone="secondary">
				{results.length === 0 ? m.teams_no_people_found() : m.teams_all_people_added()}
			</mm-text>
		{:else}
			<mm-list aria-label={m.teams_people_results()} aria-live="polite">
				{#each available as person (person.id)}
					<mm-list-item secondary={person.email ?? person.username ?? ''}>
						<mm-avatar slot="start" name={person.name} src={person.profile_picture ?? ''} decorative
						></mm-avatar>
						{person.name}
						<mm-button
							slot="end"
							size="sm"
							icon="person-add"
							label={m.teams_add_person_aria({ name: person.name })}
							loading={busyId === person.id}
							onclick={() => onAdd(person)}
						>
							{m.common_add()}
						</mm-button>
					</mm-list-item>
				{/each}
			</mm-list>
		{/if}
	{/if}
</div>
