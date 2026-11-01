<script lang="ts">
	import { onMount } from 'svelte';
	import { html } from 'lit';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { MmConfirmDialog } from '@marmotdata/elements/confirm-dialog.js';
	import type { DataTableColumn } from '@marmotdata/elements/data-table.js';
	import { connectedApps, loadConnectedApps, type ConnectedApp } from './accountData';

	onMount(() => loadConnectedApps());

	// The name is whatever the app registered with, so the id is always shown beside it
	function appName(app: ConnectedApp): string {
		return app.client_name || m.connected_apps_unnamed();
	}

	async function revoke(app: ConnectedApp) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.connected_apps_revoke_title(),
			body: m.connected_apps_revoke_message({ name: appName(app) }),
			confirmLabel: m.connected_apps_revoke_button(),
			confirmIcon: 'delete'
		});
		if (!ok) return;
		try {
			const response = await fetchApi(
				`/users/me/connected-apps/${encodeURIComponent(app.client_id)}`,
				{ method: 'DELETE' }
			);
			if (!response.ok && response.status !== 404) {
				toasts.error(await handleApiError(response));
				return;
			}
			connectedApps.update((s) => ({
				...s,
				items: (s.items ?? []).filter((a) => a.client_id !== app.client_id)
			}));
			toasts.success(m.connected_apps_revoked({ name: appName(app) }));
		} catch {
			toasts.error(m.connected_apps_revoke_error());
		}
	}

	const columns: DataTableColumn<ConnectedApp>[] = [
		{ key: 'client_name', header: m.connected_apps_column_app(), width: 200, value: appName },
		{ key: 'client_id', header: m.login_consent_client(), type: 'mono', width: 200 },
		{
			key: 'scopes',
			header: m.connected_apps_column_can(),
			width: 360,
			value: (a) => a.scope_descriptions.map((d) => d.summary).join(' ')
		},
		{
			key: 'last_authorised_at',
			header: m.connected_apps_column_authorised(),
			type: 'date',
			width: 140
		},
		{ key: 'expires_at', header: m.connected_apps_column_until(), type: 'date', width: 140 },
		{
			key: 'actions',
			header: m.common_actions(),
			width: 140,
			minWidth: 140,
			align: 'end',
			render: (a) =>
				html`<mm-button
					size="sm"
					variant="ghost"
					icon="delete"
					label=${m.connected_apps_revoke_named({ name: appName(a) })}
					@click=${() => revoke(a)}
					>${m.connected_apps_revoke_button()}</mm-button
				>`
		}
	];
</script>

<mm-section heading={m.account_connected_apps()} description={m.connected_apps_description()}>
	{#if $connectedApps.items && $connectedApps.items.length === 0}
		<mm-empty-state
			heading={m.connected_apps_empty_heading()}
			description={m.connected_apps_empty_description()}
			seed={7}
		></mm-empty-state>
	{:else}
		<mm-data-table
			label={m.account_connected_apps()}
			{columns}
			rows={$connectedApps.items ?? []}
			row-key="client_id"
			loading={!$connectedApps.items && !$connectedApps.error}
			error={$connectedApps.error}
			onmm-retry={() => loadConnectedApps(true)}
		></mm-data-table>
	{/if}
</mm-section>
