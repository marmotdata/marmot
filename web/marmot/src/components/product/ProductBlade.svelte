<script lang="ts">
	import { resolve } from '$app/paths';
	import '@marmotdata/elements/popularity.js';
	import { BladeSlot } from '$lib/mm';
	import type { DataProduct, Member } from '$lib/dataproducts/types';
	import { getProductMembers } from '$lib/dataproducts/api';
	import { assetRouteParams, popularityLevel } from '$lib/dataproducts/assets';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime, formatRelativeTime } from '$lib/utils';
	import ProductIcon from './ProductIcon.svelte';

	const PREVIEW = 5;

	interface Props {
		product?: DataProduct | null;
		onClose: () => void;
		/** Renders the blade where it is placed instead of in the shell's blade column. */
		staticPlacement?: boolean;
		collapsed?: boolean;
		onToggleCollapse?: () => void;
	}

	let {
		product = $bindable(null),
		onClose,
		staticPlacement = false,
		collapsed = false,
		onToggleCollapse
	}: Props = $props();

	let inside = $state<Member[]>([]);
	let loadingAssets = $state(false);
	let token = 0;

	$effect(() => {
		const id = product?.id;
		const mine = ++token;
		inside = [];
		if (!id) return;
		loadingAssets = true;
		getProductMembers(id, { sort: 'popular', limit: PREVIEW })
			.then((res) => {
				if (mine === token) inside = res.members;
			})
			.catch(() => {})
			.finally(() => {
				if (mine === token) loadingAssets = false;
			});
	});

	function handleClose(event: Event) {
		// The blade lives in the shell's column, so closing is the caller's job.
		event.preventDefault();
		onClose();
	}
</script>

{#snippet blade(p: DataProduct)}
	<mm-details-blade
		closable={!staticPlacement}
		collapsible={!!onToggleCollapse}
		{collapsed}
		onmm-close={handleClose}
		onmm-collapse={() => onToggleCollapse?.()}
	>
		<span slot="heading" class="mm-cluster" style="--mm-gap: var(--space-2); flex-wrap: nowrap">
			<ProductIcon iconUrl={p.icon_url} size="sm" />
			{p.name}
		</span>

		<mm-blade-section heading={m.products_blade_glance()}>
			<mm-description-list>
				<mm-description term={m.products_assets()}
					><span class="mm-num">{p.asset_count ?? 0}</span></mm-description
				>
				{#if popularityLevel(p.popularity)}
					<mm-description term={m.products_popularity()}
						><mm-popularity level={popularityLevel(p.popularity)}></mm-popularity></mm-description
					>
				{/if}
				<mm-description term={m.products_updated()}
					>{formatRelativeTime(p.updated_at)}</mm-description
				>
				{#if (p.tags ?? []).length > 0}
					<mm-description term={m.common_tags()}>
						<span class="mm-cluster" style="--mm-gap: var(--space-1)">
							{#each p.tags ?? [] as tag (tag)}<mm-tag>{tag}</mm-tag>{/each}
						</span>
					</mm-description>
				{/if}
			</mm-description-list>
		</mm-blade-section>

		{#if p.description}
			<mm-blade-section heading={m.products_blade_about()}>
				<p class="mm-dense about">{p.description}</p>
			</mm-blade-section>
		{/if}

		{#if (p.owners ?? []).length > 0}
			<mm-blade-section heading={m.products_blade_who_to_ask()}>
				<div class="mm-stack" style="--mm-gap: var(--space-3)">
					{#each p.owners ?? [] as owner (owner.type + owner.id)}
						<mm-owner-chip
							name={owner.name || owner.username || owner.id}
							kind={owner.type === 'team' ? 'team' : 'person'}
							src={owner.profile_picture ?? ''}
						></mm-owner-chip>
					{/each}
				</div>
			</mm-blade-section>
		{/if}

		{#if loadingAssets || inside.length > 0}
			<mm-blade-section heading={m.products_blade_inside()} flush>
				{#if loadingAssets && inside.length === 0}
					<mm-skeleton loading count={3}></mm-skeleton>
				{:else}
					<mm-list dense aria-label={m.products_blade_inside_label({ name: p.name })}>
						{#each inside as member (member.asset_id)}
							{@const params = assetRouteParams(member)}
							<mm-list-item
								href={params ? resolve('/discover/[type]/[service]/[name]', params) : undefined}
							>
								<mm-provider-logo
									slot="start"
									provider={member.providers.length === 1
										? member.providers[0].toLowerCase()
										: member.type}
									size={16}
									decorative
								></mm-provider-logo>
								{member.name}
								<span slot="end" class="mm-muted">{member.type}</span>
							</mm-list-item>
						{/each}
					</mm-list>
				{/if}
			</mm-blade-section>
		{/if}

		<mm-blade-section heading={m.products_blade_more_details()} collapsible>
			<mm-description-list>
				<mm-description term={m.products_added_manually()}
					><span class="mm-num">{p.manual_asset_count ?? 0}</span></mm-description
				>
				<mm-description term={m.products_from_rules()}
					><span class="mm-num">{p.rule_asset_count ?? 0}</span></mm-description
				>
				<mm-description term={m.products_tab_rules()}
					><span class="mm-num">{(p.rules ?? []).length}</span></mm-description
				>
				{#if p.created_by}
					<mm-description term={m.products_created_by()}>{p.created_by}</mm-description>
				{/if}
				<mm-description term={m.products_created_at()}
					>{formatDateTime(p.created_at)}</mm-description
				>
				<mm-description term={m.products_last_updated()}
					>{formatDateTime(p.updated_at)}</mm-description
				>
			</mm-description-list>
		</mm-blade-section>

		{#if !staticPlacement}
			<div slot="footer" class="mm-cluster" style="--mm-gap: var(--space-2)">
				<mm-button
					variant="primary"
					icon="open-in-full"
					href={resolve('/products/[id]', { id: p.id })}>{m.products_open_product()}</mm-button
				>
			</div>
		{/if}
	</mm-details-blade>
{/snippet}

{#if product}
	{#if staticPlacement}
		{@render blade(product)}
	{:else}
		<BladeSlot label={product.name}>
			{@render blade(product)}
		</BladeSlot>
	{/if}
{/if}

<style>
	.about {
		margin: 0;
		white-space: pre-wrap;
	}
</style>
