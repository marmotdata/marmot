<script lang="ts">
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import type { DataTableColumn } from '@marmotdata/elements/data-table.js';
	import { subscriptions, loadSubscriptions, type SubscriptionWithAsset } from './accountData';
	import { html } from 'lit';

	const typeLabels: Record<string, () => string> = {
		asset_change: m.subscriptions_type_asset_change,
		schema_change: m.subscriptions_type_schema_change,
		upstream_schema_change: m.subscriptions_type_upstream_schema,
		downstream_schema_change: m.subscriptions_type_downstream_schema,
		lineage_change: m.subscriptions_type_lineage_change
	};

	// MRNs read mrn://<type>/<provider>/<name>, so the provider logo comes from the MRN without another request
	function providerOf(mrn: string): string {
		return mrn.replace('mrn://', '').split('/')[1]?.toLowerCase() ?? '';
	}

	function assetHref(sub: SubscriptionWithAsset): string {
		return resolve(
			`/discover/${sub.asset_mrn.replace('mrn://', '')}` as `/discover/${string}/${string}/${string}`
		);
	}

	async function unsubscribe(sub: SubscriptionWithAsset) {
		try {
			const response = await fetchApi(`/subscriptions/${sub.id}`, { method: 'DELETE' });
			if (!response.ok) {
				toasts.error(await handleApiError(response));
				return;
			}
			subscriptions.update((s) => ({
				...s,
				items: (s.items ?? []).filter((x) => x.id !== sub.id)
			}));
			toasts.success(m.subscriptions_unsubscribed({ name: sub.asset_name }));
		} catch {
			toasts.error(m.subscriptions_unsubscribe_error());
		}
	}

	const columns: DataTableColumn<SubscriptionWithAsset>[] = [
		{
			key: 'asset_name',
			header: m.subscriptions_column_asset(),
			width: 300,
			render: (s) => {
				const name = s.asset_name || m.subscriptions_unknown_asset();
				const provider = providerOf(s.asset_mrn ?? '');
				const logo = provider
					? html`<mm-provider-logo provider=${provider} size="16" decorative></mm-provider-logo>`
					: null;
				const style =
					'display: inline-flex; align-items: center; gap: var(--space-2); white-space: nowrap';
				return s.asset_mrn
					? html`<a class="mm-link" style=${style} href=${assetHref(s)}>${logo}${name}</a>`
					: html`<span style=${style}>${logo}${name}</span>`;
			}
		},
		{ key: 'asset_type', header: m.common_type(), width: 120 },
		{
			key: 'notification_types',
			header: m.subscriptions_column_notifications(),
			width: 320,
			value: (s) => s.notification_types.map((t) => typeLabels[t]?.() ?? t).join(' · ')
		},
		{ key: 'created_at', header: m.subscriptions_column_since(), type: 'date', width: 140 },
		{
			key: 'actions',
			header: m.common_actions(),
			width: 160,
			minWidth: 160,
			align: 'end',
			render: (s) =>
				html`<mm-button
					size="sm"
					variant="ghost"
					icon="notifications-off"
					label=${m.subscriptions_unsubscribe_from({
						name: s.asset_name || m.subscriptions_unknown_asset()
					})}
					@click=${() => unsubscribe(s)}
					>${m.subscriptions_unsubscribe_title()}</mm-button
				>`
		}
	];
</script>

{#if $subscriptions.items && $subscriptions.items.length === 0}
	<mm-empty-state
		heading={m.subscriptions_empty_heading()}
		description={m.subscriptions_empty()}
		seed={6}
	>
		<mm-button slot="action" icon="travel-explore" href={resolve('/discover')}
			>{m.subscriptions_discover_assets()}</mm-button
		>
	</mm-empty-state>
{:else}
	<mm-data-table
		label={m.account_subscriptions()}
		{columns}
		rows={$subscriptions.items ?? []}
		row-key="id"
		loading={!$subscriptions.items && !$subscriptions.error}
		error={$subscriptions.error}
		onmm-retry={() => loadSubscriptions(true)}
	></mm-data-table>
{/if}
