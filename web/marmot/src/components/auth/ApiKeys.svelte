<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/utils';
	import DatePicker from '$components/ui/DatePicker.svelte';
	import { myAccess, loadMyAccess } from '$lib/iam/myAccess';
	import { canSee, settingsGroups } from '$lib/settings/sections';
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/code-sample.js';
	import { MmConfirmDialog } from '@marmotdata/elements/confirm-dialog.js';
	import type { DataTableColumn } from '@marmotdata/elements/data-table.js';
	import type { MmDialog } from '@marmotdata/elements/dialog.js';
	import { apiKeys, loadApiKeys, type ApiKey } from './accountData';
	import { html } from 'lit';

	// Keys expiring within this many days get a warning pill so people rotate them before anything breaks
	const EXPIRING_DAYS = 14;
	const DAY = 86_400_000;

	const expirationOptions = [
		{ value: '1', label: m.apikeys_expiration_days_option({ count: 1 }) },
		{ value: '7', label: m.apikeys_expiration_days_option({ count: 7 }) },
		{ value: '30', label: m.apikeys_expiration_days_option({ count: 30 }) },
		{ value: '90', label: m.apikeys_expiration_days_option({ count: 90 }) },
		{ value: '180', label: m.apikeys_expiration_days_option({ count: 180 }) },
		{ value: '365', label: m.apikeys_expiration_days_option({ count: 365 }) },
		{ value: 'custom', label: m.apikeys_expiration_custom() },
		{ value: 'never', label: m.apikeys_expiration_never() }
	];

	let createDialog = $state<MmDialog>();
	let newKeyName = $state('');
	let nameError = $state('');
	let expiration = $state('90');
	let customDate = $state('');
	let creating = $state(false);
	let created = $state<ApiKey | null>(null);
	// Set when the drawer is showing a rotated key rather than a new one, so its heading says so
	let rotated = $state(false);

	const origin = typeof window === 'undefined' ? '' : window.location.origin;
	const mcpConfig = $derived(
		created?.key
			? JSON.stringify(
					{
						mcpServers: {
							marmot: { url: `${origin}/api/v1/mcp`, headers: { 'X-API-Key': created.key } }
						}
					},
					null,
					2
				)
			: ''
	);

	// Personal keys die with the person, so point anyone who can manage service accounts at the kind of key that outlives them
	const serviceAccountsItem = settingsGroups
		.flatMap((group) => group.items)
		.find((item) => item.id === 'service-accounts');

	onMount(() => {
		loadMyAccess();
		loadApiKeys();
	});

	function startOfToday(): Date {
		const now = new Date();
		return new Date(now.getFullYear(), now.getMonth(), now.getDate());
	}

	function toDateInputValue(date: Date): string {
		const month = String(date.getMonth() + 1).padStart(2, '0');
		const day = String(date.getDate()).padStart(2, '0');
		return `${date.getFullYear()}-${month}-${day}`;
	}

	function daysUntil(dateValue: string): number | null {
		const [year, month, day] = dateValue.split('-').map(Number);
		if (!year || !month || !day) return null;
		const picked = new Date(year, month - 1, day);
		const days = Math.round((picked.getTime() - startOfToday().getTime()) / DAY);
		return days > 0 ? days : null;
	}

	function resolveExpiresInDays(): number | null {
		if (expiration === 'never') return 0;
		if (expiration === 'custom') return customDate ? daysUntil(customDate) : null;
		return Number(expiration);
	}

	function openCreate() {
		newKeyName = '';
		nameError = '';
		expiration = '90';
		customDate = '';
		createDialog?.show();
	}

	function validateName(): boolean {
		const name = newKeyName.trim();
		if (!name) nameError = m.apikeys_error_name_empty();
		else if ($apiKeys.items?.some((key) => key.name === name))
			nameError = m.apikeys_error_name_exists();
		else nameError = '';
		return !nameError;
	}

	async function createApiKey(e?: Event) {
		e?.preventDefault();
		if (creating || !validateName()) return;
		const expiresInDays = resolveExpiresInDays();
		if (expiresInDays === null) {
			toasts.warning(m.apikeys_error_custom_date_future());
			return;
		}
		creating = true;
		try {
			const response = await fetchApi('/users/apikeys', {
				method: 'POST',
				body: JSON.stringify({ name: newKeyName.trim(), expires_in_days: expiresInDays })
			});
			if (!response.ok) {
				toasts.error(await handleApiError(response));
				return;
			}
			const result: ApiKey = await response.json();
			apiKeys.update((s) => ({ ...s, items: [...(s.items ?? []), { ...result, key: undefined }] }));
			createDialog?.close();
			rotated = false;
			created = result;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.apikeys_create_error());
		} finally {
			creating = false;
		}
	}

	async function revoke(key: ApiKey) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.apikeys_delete_title(),
			body: m.apikeys_delete_message(),
			confirmLabel: m.apikeys_revoke_button(),
			confirmIcon: 'delete',
			confirmText: key.name
		});
		if (!ok) return;
		try {
			const response = await fetchApi(`/users/apikeys/${key.id}`, { method: 'DELETE' });
			if (!response.ok) {
				toasts.error(await handleApiError(response));
				return;
			}
			apiKeys.update((s) => ({ ...s, items: (s.items ?? []).filter((k) => k.id !== key.id) }));
			toasts.success(m.apikeys_deleted_success({ name: key.name }));
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.apikeys_delete_error());
		}
	}

	async function rotate(key: ApiKey) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.apikeys_rotate_title(),
			body: m.apikeys_rotate_message(),
			confirmLabel: m.apikeys_rotate_button(),
			confirmIcon: 'sync'
		});
		if (!ok) return;
		try {
			const response = await fetchApi(`/users/apikeys/${key.id}/rotate`, { method: 'POST' });
			if (!response.ok) {
				toasts.error(await handleApiError(response));
				return;
			}
			const result: ApiKey = await response.json();
			apiKeys.update((s) => ({
				...s,
				items: (s.items ?? []).map((k) => (k.id === key.id ? { ...result, key: undefined } : k))
			}));
			rotated = true;
			created = result;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.apikeys_rotate_error());
		}
	}

	function onRowMenu(key: ApiKey, value: string) {
		if (value === 'revoke') revoke(key);
	}

	function stateOf(key: ApiKey): 'expired' | 'expiring' | 'active' {
		if (!key.expires_at) return 'active';
		const left = new Date(key.expires_at).getTime() - Date.now();
		if (left <= 0) return 'expired';
		return left < EXPIRING_DAYS * DAY ? 'expiring' : 'active';
	}

	const columns: DataTableColumn<ApiKey>[] = [
		{ key: 'name', header: m.common_name(), width: 240 },
		{
			key: 'prefix',
			header: m.apikeys_column_key(),
			type: 'mono',
			width: 160,
			empty: m.apikeys_prefix_unknown()
		},
		{ key: 'created_at', header: m.apikeys_column_created(), type: 'date', width: 130 },
		{
			key: 'last_used_at',
			header: m.apikeys_column_last_used(),
			type: 'date',
			width: 130,
			empty: m.common_never()
		},
		{
			key: 'expires_at',
			header: m.apikeys_column_status(),
			width: 190,
			minWidth: 160,
			render: (k) => {
				const state = stateOf(k);
				if (state === 'expired')
					return html`<mm-status-pill status="danger" icon="error"
						>${m.apikeys_expired_badge()}</mm-status-pill
					>`;
				const date = k.expires_at ? formatDate(k.expires_at) : '';
				if (state === 'expiring')
					return html`<mm-status-pill status="warning" icon="schedule"
						>${m.apikeys_expires_on({ date })}</mm-status-pill
					>`;
				return html`<span class="mm-muted"
					>${date ? m.apikeys_expires_on({ date }) : m.apikeys_never_expires()}</span
				>`;
			}
		},
		{
			key: 'actions',
			header: m.common_actions(),
			width: 180,
			minWidth: 180,
			align: 'end',
			// One visible button per row: the action the row needs most, with the rest in More
			render: (k) =>
				stateOf(k) === 'active'
					? html`<mm-button
							size="sm"
							variant="ghost"
							icon="delete"
							label=${m.apikeys_revoke_named({ name: k.name })}
							@click=${() => revoke(k)}
							>${m.apikeys_revoke_button()}</mm-button
						>`
					: html`<span
							class="mm-cluster"
							style="--mm-gap:var(--space-1);justify-content:flex-end;flex-wrap:nowrap"
						>
							<mm-button size="sm" icon="sync" @click=${() => rotate(k)}
								>${m.apikeys_rotate_button()}</mm-button
							>
							<mm-menu-button
								size="sm"
								variant="ghost"
								icon="more-horiz"
								icon-only
								label=${m.apikeys_more_actions({ name: k.name })}
							>
								<mm-menu
									label=${m.apikeys_actions_for({ name: k.name })}
									@mm-select=${(e: CustomEvent<{ value: string }>) => onRowMenu(k, e.detail.value)}
								>
									<mm-menu-item icon="delete" value="revoke" danger
										>${m.apikeys_revoke_key()}</mm-menu-item
									>
								</mm-menu>
							</mm-menu-button>
						</span>`
		}
	];
</script>

<mm-section heading={m.apikeys_heading()} description={m.account_api_keys_description()}>
	<mm-button slot="actions" variant="primary" icon="key" onclick={openCreate}
		>{m.apikeys_new_key_button()}</mm-button
	>
	<div class="mm-stack" style="--mm-gap: var(--space-4)">
		{#if $apiKeys.items && $apiKeys.items.length === 0}
			<mm-empty-state
				heading={m.apikeys_none_found()}
				description={m.apikeys_empty_description()}
				seed={4}
			></mm-empty-state>
		{:else}
			<mm-data-table
				label={m.apikeys_heading()}
				{columns}
				rows={$apiKeys.items ?? []}
				row-key="id"
				loading={!$apiKeys.items && !$apiKeys.error}
				error={$apiKeys.error}
				onmm-retry={() => loadApiKeys(true)}
			></mm-data-table>
		{/if}
		{#if serviceAccountsItem && canSee($myAccess, serviceAccountsItem)}
			<mm-text variant="caption" tone="secondary" block>
				{m.apikeys_service_account_hint()}
				<mm-link href={resolve(serviceAccountsItem.href)}
					>{m.apikeys_service_account_link()}</mm-link
				>
			</mm-text>
		{/if}
	</div>
</mm-section>

<mm-dialog bind:this={createDialog} heading={m.apikeys_create_heading()} size="sm">
	<form class="mm-stack" style="--mm-gap: var(--space-5)" onsubmit={createApiKey}>
		<mm-text-field
			label={m.apikeys_name_label()}
			value={newKeyName}
			error={nameError}
			required
			oninput={(e: Event) => (newKeyName = (e.target as HTMLInputElement).value)}
			onmm-change={validateName}
			onkeydown={(e: KeyboardEvent) => e.key === 'Enter' && createApiKey(e)}
		></mm-text-field>
		<mm-select
			label={m.apikeys_expiration_label()}
			value={expiration}
			help={expiration === 'never' ? m.apikeys_never_expires_hint() : ''}
			onmm-change={(e: CustomEvent<{ value: string }>) => (expiration = e.detail.value)}
		>
			{#each expirationOptions as option (option.value)}
				<mm-option value={option.value} label={option.label}></mm-option>
			{/each}
		</mm-select>
		{#if expiration === 'custom'}
			<div class="mm-stack" style="--mm-gap: var(--space-2)">
				<label class="date-label" for="key-custom-date">{m.apikeys_expiry_date_label()}</label>
				<DatePicker
					id="key-custom-date"
					bind:value={customDate}
					min={toDateInputValue(new Date(startOfToday().getTime() + DAY))}
				/>
			</div>
		{/if}
	</form>
	<mm-button slot="footer" data-close="cancel" disabled={creating}>{m.common_cancel()}</mm-button>
	<mm-button
		slot="footer"
		variant="primary"
		disabled={creating || (expiration === 'custom' && !customDate)}
		onclick={createApiKey}>{m.apikeys_new_key_button()}</mm-button
	>
</mm-dialog>

{#if created}
	<mm-drawer
		open
		heading={rotated ? m.apikeys_rotated_heading() : m.apikeys_created_heading()}
		description={created.expires_at
			? m.apikeys_created_expires({ date: formatDate(created.expires_at) })
			: m.apikeys_never_expires()}
		onmm-close={() => (created = null)}
	>
		<div class="mm-stack" style="--mm-gap: var(--space-5)">
			<mm-callout tone="warning" heading={m.apikeys_copy_warning()}
				>{m.apikeys_copy_warning_hint()}</mm-callout
			>
			<mm-code-sample language="shell" label={m.apikeys_new_key_label()} code={created.key ?? ''}
			></mm-code-sample>
			<mm-section heading={m.apikeys_connect_agent()}>
				<mm-code-sample wrap language="json" label={m.apikeys_mcp_config_label()} code={mcpConfig}
				></mm-code-sample>
			</mm-section>
		</div>
		<mm-button slot="footer" variant="primary" data-close="done"
			>{m.apikeys_copied_button()}</mm-button
		>
	</mm-drawer>
{/if}

<style>
	.date-label {
		font: var(--text-label);
		color: var(--mm-text-2);
	}
</style>
