<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { kindById, knowledgeKinds, loadKinds, statusOf } from '#lib/knowledge/kinds.ts';
	import {
		knowledgeAbout,
		type KnowledgeAboutItem,
		type KnowledgeAboutPage
	} from '#lib/knowledge/api.ts';
	import { ruleLabel } from '#lib/knowledge/rules.ts';

	// What the company knows about one asset, on that asset's page: policies, decisions and runbooks linked to it, matched to it by a rule, or inherited from what contains it. Only a few are shown, grouped by kind with totals, and the rest are one click away on the Knowledge page, so an asset with thousands of entries loads as fast as one with none.
	let { assetId }: { assetId: string } = $props();

	const PREVIEW = 8;

	let page = $state<KnowledgeAboutPage | null>(null);

	$effect(() => {
		const id = assetId;
		page = null;
		const ctrl = new AbortController();
		loadKinds().catch(() => {});
		knowledgeAbout(id, { limit: PREVIEW }, ctrl.signal)
			.then((p) => {
				if (assetId === id) page = p;
			})
			.catch(() => {});
		return () => ctrl.abort();
	});

	let groups = $derived.by(() => {
		const out: {
			kind: string;
			label: string;
			icon: string;
			total: number;
			items: KnowledgeAboutItem[];
		}[] = [];
		for (const item of page?.entries ?? []) {
			let g = out.find((x) => x.kind === item.kind);
			if (!g) {
				const k = kindById(item.kind, $knowledgeKinds);
				g = {
					kind: item.kind,
					label: k?.label ?? item.kind,
					icon: k?.icon ?? 'description',
					total: page?.by_kind[item.kind] ?? 0,
					items: []
				};
				out.push(g);
			}
			g.items.push(item);
		}
		return out;
	});

	function note(item: KnowledgeAboutItem): string {
		const parts: string[] = [];
		if (item.state === 'pending' || item.state === 'historical') {
			parts.push(statusOf(kindById(item.kind, $knowledgeKinds), item.status)?.label ?? '');
		}
		if (item.source === 'inherited' && item.via)
			parts.push(m.knowledge_about_via({ name: item.via.name }));
		if (item.source === 'rule' && item.rule) parts.push(ruleLabel(item.rule));
		return parts.filter(Boolean).join(' · ');
	}
</script>

{#if page && page.total > 0}
	<mm-blade-section heading={m.knowledge_blade_heading()}>
		<div class="mm-stack" style="--mm-gap: var(--space-4)">
			{#each groups as g (g.kind)}
				<div class="mm-stack" style="--mm-gap: var(--space-2)">
					<div class="mm-cluster" style="--mm-gap: var(--space-2)">
						<mm-icon name={g.icon} size={16} aria-hidden="true"></mm-icon>
						<mm-text variant="label" tone="secondary">{g.label}</mm-text>
						<mm-count value={g.total}></mm-count>
					</div>
					<ul class="items">
						{#each g.items as item (item.asset_id)}
							<li class:historical={item.state === 'historical'}>
								<mm-link href={resolve('/knowledge/[id]', { id: item.asset_id })}
									>{item.name}</mm-link
								>
								{#if note(item)}
									<mm-text variant="caption" tone="secondary" block>{note(item)}</mm-text>
								{/if}
							</li>
						{/each}
					</ul>
				</div>
			{/each}
			{#if page.total > page.entries.length}
				<mm-link href={resolve('/knowledge/about/[id]', { id: assetId })}
					>{m.knowledge_about_see_all({ count: page.total })}</mm-link
				>
			{/if}
		</div>
	</mm-blade-section>
{/if}

<style>
	.items {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}
	.historical mm-link {
		opacity: 0.75;
	}
</style>
