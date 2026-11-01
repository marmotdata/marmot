<script lang="ts">
	import { onMount } from 'svelte';
	import { html } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table';
	import { listProviderStatuses, setProviderEnabled } from '$lib/sso/api';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { SSOProviderStatus } from '$lib/sso/types';

	// Providers are defined in the server configuration, so this page cannot add one; it can switch a configured provider off and on again without touching the file, and shows who uses each.
	let { onChange }: { onChange?: (providers: SSOProviderStatus[]) => void } = $props();

	let providers = $state<SSOProviderStatus[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busy = $state<string | null>(null);

	async function toggle(p: SSOProviderStatus) {
		busy = p.type;
		try {
			const updated = await setProviderEnabled(p.type, !p.enabled);
			providers = providers.map((x) => (x.type === p.type ? updated : x));
			onChange?.(providers);
			toasts.success(
				updated.enabled
					? m.sso_turned_on_toast({ name: p.name })
					: m.sso_turned_off_toast({ name: p.name })
			);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.sso_toggle_failed());
		} finally {
			busy = null;
		}
	}

	let columns = $derived<DataTableColumn<SSOProviderStatus>[]>([
		{ key: 'name', header: m.sso_column_provider(), width: 200 },
		{ key: 'protocol', header: m.sso_column_protocol(), width: 110 },
		{ key: 'issuer_url', header: m.sso_column_issuer_url(), type: 'mono', width: 360, empty: '–' },
		{
			key: 'enabled',
			header: m.common_status(),
			width: 130,
			value: (p) => (p.enabled ? m.sso_status_on() : m.sso_status_off()),
			// No pill: a provider switched off is a choice, not a fault
			render: (p) =>
				p.enabled ? m.sso_status_on() : html`<span class="mm-muted">${m.sso_status_off()}</span>`
		},
		{ key: 'member_count', header: m.sso_column_members(), type: 'number', width: 110 },
		{
			key: 'actions',
			header: m.common_actions(),
			width: 140,
			minWidth: 140,
			align: 'end',
			render: (p) =>
				html`<mm-button
					size="sm"
					icon=${p.enabled ? 'block' : 'power'}
					?loading=${busy === p.type}
					@click=${() => toggle(p)}
					>${p.enabled ? m.sso_turn_off() : m.sso_turn_on()}</mm-button
				>`
		}
	]);

	async function load() {
		loading = true;
		error = '';
		try {
			providers = await listProviderStatuses();
			onChange?.(providers);
		} catch (err) {
			error = err instanceof Error ? err.message : m.sso_load_error();
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<mm-section
	heading={m.sso_providers_heading()}
	description={m.sso_config_note_body({ file: 'config.yaml', section: 'auth' })}
>
	{#if !loading && !error && providers.length === 0}
		<mm-empty-state
			compact
			heading={m.sso_none_configured()}
			description={m.sso_none_configured_description()}
		></mm-empty-state>
	{:else}
		<mm-data-table
			label={m.sso_providers_heading()}
			row-key="type"
			{columns}
			rows={providers}
			{loading}
			{error}
			onmm-retry={load}
		></mm-data-table>
	{/if}
</mm-section>
