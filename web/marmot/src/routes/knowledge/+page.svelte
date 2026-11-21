<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/env';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { auth } from '#lib/stores/auth.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { formatRelativeTime } from '#lib/utils/format.ts';
	import { kindById, knowledgeKinds, loadKinds, statusOf } from '#lib/knowledge/kinds.ts';
	import {
		getCounts,
		listEntries,
		type KnowledgeCounts,
		type KnowledgeSummary
	} from '#lib/knowledge/api.ts';
	import StatusPill from '#components/knowledge/StatusPill.svelte';

	// The front door to what the company knows. It answers before it lists: a search that shows the best match with its status and owner as you type, then what kinds of knowledge exist, what changed lately and what needs someone's attention. Filtering everything lives on Browse.
	setCrumbs([{ label: m.knowledge_heading() }]);

	const canManage = auth.hasPermission('assets', 'manage');
	const RESULTS = 8;

	let counts = $state<KnowledgeCounts | null>(null);
	let recent = $state<KnowledgeSummary[]>([]);
	let attention = $state<KnowledgeSummary[]>([]);
	let attentionIsMine = $state(true);
	let loadFailed = $state(false);

	let query = $state('');
	let results = $state<KnowledgeSummary[]>([]);
	let resultTotal = $state(0);
	let searching = $state(false);
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let seq = 0;

	onMount(() => {
		if (!browser) return;
		loadKinds()
			.then(loadAttention)
			.catch(() => {});
		getCounts()
			.then((c) => (counts = c))
			.catch(() => (loadFailed = true));
		listEntries({ sort: 'updated', limit: 6 })
			.then((r) => (recent = r.entries))
			.catch(() => {});
	});

	// What needs looking at: overdue reviews first, then drafts and proposals, from the person's own entries when they own any and from everyone's otherwise.
	async function loadAttention() {
		const pending = [
			...new SvelteSet(
				$knowledgeKinds.flatMap((k) =>
					k.statuses.filter((s) => s.state === 'pending').map((s) => s.value)
				)
			)
		];
		const pick = async (mine: boolean) => {
			const [due, drafts] = await Promise.all([
				listEntries({ reviewDue: true, mine, sort: 'review', limit: 5 }),
				listEntries({ statuses: pending, mine, sort: 'updated', limit: 5 })
			]);
			const seen = new SvelteSet<string>();
			return [...due.entries, ...drafts.entries]
				.filter((e) => !seen.has(e.asset_id) && seen.add(e.asset_id))
				.slice(0, 6);
		};
		try {
			let list = await pick(true);
			attentionIsMine = list.length > 0;
			if (!list.length) list = await pick(false);
			attention = list;
		} catch {
			attention = [];
		}
	}

	function onInput(value: string) {
		query = value;
		clearTimeout(searchTimer);
		const q = value.trim();
		if (!q) {
			results = [];
			searching = false;
			return;
		}
		searching = true;
		searchTimer = setTimeout(async () => {
			const mine = ++seq;
			try {
				const r = await listEntries({ q, limit: RESULTS });
				if (mine !== seq) return;
				results = r.entries;
				resultTotal = r.total;
			} catch {
				if (mine === seq) results = [];
			} finally {
				if (mine === seq) searching = false;
			}
		}, 180);
	}

	// A Browse path with its filters; callers resolve it.
	function browsePath(params: Record<string, string>): string {
		const qs = Object.entries(params)
			.map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
			.join('&');
		return `/knowledge/browse${qs ? `?${qs}` : ''}`;
	}

	function kindLabel(e: KnowledgeSummary): string {
		return kindById(e.kind, $knowledgeKinds)?.label ?? '';
	}

	function statusWord(e: KnowledgeSummary): string {
		return statusOf(kindById(e.kind, $knowledgeKinds), e.status)?.label ?? '';
	}

	function owners(e: KnowledgeSummary): string {
		return e.owners.map((o) => o.name).join(', ');
	}

	let best = $derived(results[0]);
	let rest = $derived(results.slice(1));
	let inUse = $derived((id: string) => counts?.by_kind[id] ?? 0);
</script>

<svelte:head>
	<title>{m.knowledge_page_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-6)">
	<mm-page-header heading={m.knowledge_heading()} description={m.knowledge_home_description()}>
		<div slot="actions" class="mm-cluster" style="--mm-gap: var(--space-2)">
			{#if counts?.total}
				<mm-button variant="secondary" icon="filter-list" href={resolve('/knowledge/browse')}
					>{m.knowledge_browse_all()}</mm-button
				>
			{/if}
			{#if canManage}
				<mm-menu-button variant="primary" icon="add" label={m.knowledge_new()}>
					<mm-menu
						label={m.knowledge_new()}
						onmm-select={(e: CustomEvent<{ value: string }>) =>
							goto(resolve(`/knowledge/new?kind=${e.detail.value}` as '/knowledge/new'))}
					>
						{#each $knowledgeKinds as k (k.id)}
							<mm-menu-item value={k.id} icon={k.icon} description={k.description}
								>{k.label}</mm-menu-item
							>
						{/each}
					</mm-menu>
				</mm-menu-button>
			{/if}
		</div>
	</mm-page-header>

	{#if counts && counts.total === 0}
		<mm-empty-state
			heading={m.knowledge_empty_title()}
			description={m.knowledge_empty_description()}
			seed={7}
		></mm-empty-state>
		{#if canManage}
			<mm-section heading={m.knowledge_start_heading()}>
				<mm-link-list columns="2">
					{#each $knowledgeKinds as k (k.id)}
						<mm-link-list-item
							href={`${resolve('/knowledge/new')}?kind=${k.id}`}
							icon={k.icon}
							description={k.description}>{k.label}</mm-link-list-item
						>
					{/each}
				</mm-link-list>
			</mm-section>
		{/if}
	{:else}
		<div class="ask">
			<mm-search-field
				label={m.knowledge_ask_label()}
				hide-label
				placeholder={m.knowledge_ask_placeholder()}
				shortcut="/"
				value={query}
				onmm-input={(e: CustomEvent<{ value: string }>) => onInput(e.detail.value)}
				onmm-search={() =>
					query.trim() && goto(resolve(browsePath({ q: query.trim() }) as '/knowledge/browse'))}
			></mm-search-field>
			{#if !query}
				<mm-text variant="caption" tone="secondary" block>{m.knowledge_ask_hint()}</mm-text>
			{/if}
		</div>

		{#if query.trim()}
			<section
				class="mm-stack"
				style="--mm-gap: var(--space-4)"
				aria-label={m.knowledge_results_label()}
				aria-busy={searching}
			>
				{#if searching && results.length === 0}
					<mm-skeleton loading count={4}></mm-skeleton>
				{:else if !best}
					<mm-empty-state
						heading={m.knowledge_no_match_title()}
						description={m.knowledge_ask_no_match()}
						seed={3}
					>
						{#if canManage}
							<mm-button slot="action" href={resolve('/knowledge/new')}
								>{m.knowledge_ask_write_it()}</mm-button
							>
						{/if}
					</mm-empty-state>
				{:else}
					{@const kind = kindById(best.kind, $knowledgeKinds)}
					<article class="best" aria-label={m.knowledge_best_match()}>
						<mm-text variant="label" tone="secondary" block>{m.knowledge_best_match()}</mm-text>
						<div class="mm-cluster" style="--mm-gap: var(--space-2)">
							<mm-icon name={kind?.icon ?? 'description'} size={20} aria-hidden="true"></mm-icon>
							<a
								class="mm-link mm-link--quiet best-name"
								href={resolve('/knowledge/[id]', { id: best.asset_id })}>{best.name}</a
							>
							<StatusPill
								{kind}
								status={best.status}
								state={best.state}
								reviewDue={best.review_due}
							/>
						</div>
						{#if best.summary}
							<p class="best-summary">{best.summary}</p>
						{/if}
						<mm-text variant="caption" tone="secondary" block>
							{[
								kindLabel(best),
								owners(best) && m.knowledge_ask_who({ names: owners(best) }),
								m.knowledge_updated({ date: formatRelativeTime(best.updated_at) })
							]
								.filter(Boolean)
								.join(' · ')}
						</mm-text>
					</article>
					{#if rest.length}
						<mm-list aria-label={m.knowledge_more_results()}>
							{#each rest as e (e.asset_id)}
								{@const k = kindById(e.kind, $knowledgeKinds)}
								<mm-list-item
									href={resolve('/knowledge/[id]', { id: e.asset_id })}
									secondary={e.summary || kindLabel(e)}
								>
									<mm-icon slot="start" name={k?.icon ?? 'description'} aria-hidden="true"
									></mm-icon>
									{e.name}
									<span slot="end"
										><StatusPill
											kind={k}
											status={e.status}
											state={e.state}
											reviewDue={e.review_due}
										/></span
									>
								</mm-list-item>
							{/each}
						</mm-list>
					{/if}
					{#if resultTotal > results.length}
						<div>
							<mm-link href={resolve(browsePath({ q: query.trim() }) as '/knowledge/browse')}
								>{m.knowledge_see_all_results({ count: resultTotal })}</mm-link
							>
						</div>
					{/if}
				{/if}
			</section>
		{:else}
			{#if loadFailed}
				<mm-callout tone="danger" heading={m.knowledge_load_failed()}></mm-callout>
			{/if}

			<mm-section
				heading={m.knowledge_shelf_heading()}
				description={m.knowledge_shelf_description()}
			>
				<ul class="shelf">
					{#each $knowledgeKinds as k (k.id)}
						<li>
							<a href={resolve(browsePath({ kind: k.id }) as '/knowledge/browse')}>
								<mm-icon name={k.icon} size={20} aria-hidden="true"></mm-icon>
								<span class="shelf-text">
									<span class="shelf-name">{k.label}</span>
									<mm-text variant="caption" tone="secondary" block>{k.description}</mm-text>
								</span>
								<mm-count value={inUse(k.id)} label={m.knowledge_shelf_count({ kind: k.label })}
								></mm-count>
							</a>
						</li>
					{/each}
				</ul>
			</mm-section>

			<div class="mm-split mm-split--even">
				<mm-panel
					heading={m.knowledge_recent_heading()}
					icon="history"
					link-href={resolve(browsePath({}) as '/knowledge/browse')}
					link-label={m.knowledge_browse_all()}
				>
					<mm-list aria-label={m.knowledge_recent_heading()}>
						{#each recent as e (e.asset_id)}
							{@const k = kindById(e.kind, $knowledgeKinds)}
							<mm-list-item
								href={resolve('/knowledge/[id]', { id: e.asset_id })}
								secondary={[kindLabel(e), statusWord(e), formatRelativeTime(e.updated_at)]
									.filter(Boolean)
									.join(' · ')}
							>
								<mm-icon slot="start" name={k?.icon ?? 'description'} aria-hidden="true"></mm-icon>
								{e.name}
							</mm-list-item>
						{/each}
					</mm-list>
				</mm-panel>

				<div class="mm-stack" style="--mm-gap: var(--space-5)">
					{#if attention.length}
						<mm-panel
							heading={attentionIsMine ? m.knowledge_attention_mine() : m.knowledge_attention_all()}
							icon="flag"
							count={attention.length}
							link-href={resolve(browsePath({ show: 'review' }) as '/knowledge/browse')}
							link-label={m.knowledge_attention_link()}
						>
							<mm-list aria-label={m.knowledge_attention_all()}>
								{#each attention as e (e.asset_id)}
									{@const k = kindById(e.kind, $knowledgeKinds)}
									<mm-list-item
										href={resolve('/knowledge/[id]', { id: e.asset_id })}
										secondary={e.review_due
											? m.knowledge_attention_review({
													date: formatRelativeTime(e.review_by ?? e.updated_at)
												})
											: m.knowledge_attention_pending({ status: statusWord(e).toLowerCase() })}
									>
										<mm-icon slot="start" name={k?.icon ?? 'description'} aria-hidden="true"
										></mm-icon>
										{e.name}
									</mm-list-item>
								{/each}
							</mm-list>
						</mm-panel>
					{:else if counts}
						<mm-panel heading={m.knowledge_attention_all()} icon="flag" padding="default">
							<mm-text variant="dense" tone="secondary" block
								>{m.knowledge_attention_clear()}</mm-text
							>
						</mm-panel>
					{/if}

					{#if counts?.by_team.length}
						<mm-panel heading={m.knowledge_teams_heading()} icon="groups">
							<mm-list aria-label={m.knowledge_teams_heading()}>
								{#each counts.by_team.slice(0, 6) as t (t.id)}
									<mm-list-item href={resolve(browsePath({ team: t.id }) as '/knowledge/browse')}>
										<mm-icon slot="start" name="groups" aria-hidden="true"></mm-icon>
										{t.name}
										<mm-count slot="end" value={t.count}></mm-count>
									</mm-list-item>
								{/each}
							</mm-list>
						</mm-panel>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	.ask {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		max-width: 48rem;
	}
	.ask mm-search-field {
		--mm-field-height: 3rem;
	}
	.best {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		padding: var(--space-5);
		border: 1px solid var(--mm-rule);
		border-radius: var(--radius-md);
		background: var(--mm-surface);
		max-width: 48rem;
	}
	.best-name {
		font: var(--text-heading);
	}
	.best-summary {
		margin: 0;
		font: var(--text-body);
		color: var(--mm-text);
	}
	.shelf {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr));
		border-block-start: 1px solid var(--mm-rule);
	}
	.shelf li {
		border-block-end: 1px solid var(--mm-rule);
	}
	.shelf a {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-2);
		color: inherit;
		text-decoration: none;
	}
	.shelf a:hover,
	.shelf a:focus-visible {
		background: var(--mm-sunken);
	}
	.shelf a:hover .shelf-name {
		text-decoration: underline;
	}
	.shelf-text {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		flex: 1;
		min-width: 0;
	}
	.shelf-name {
		font: var(--text-body-strong);
	}
</style>
