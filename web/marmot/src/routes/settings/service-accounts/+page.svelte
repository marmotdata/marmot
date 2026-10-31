<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { html, nothing } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import type { MmPageChangeDetail } from '@marmotdata/elements/classes/pagination.js';
	import { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import { setCrumbs } from '$lib/mm';
	import { toasts } from '$lib/stores/toast';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { m } from '$lib/paraglide/messages';
	import { listServiceAccounts, deleteServiceAccount } from '$lib/serviceaccounts/api';
	import { rowMenu } from '$lib/users/cells';
	import { roleLabel } from '$lib/users/format';
	import { assetHref } from '$lib/assetrules/assets';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';
	import CreateServiceAccountDrawer from '$components/serviceaccount/CreateServiceAccountDrawer.svelte';

	// The API returns every service account at once, so search, the status filter and paging run here
	const PAGE_SIZE = 50;

	let canManage = $derived(holdsAtRoot($myAccess, 'service_accounts:manage'));

	let accounts = $state<ServiceAccount[]>([]);
	let loading = $state(true);
	let error = $state('');
	let query = $state('');
	let status = $state<'all' | 'active' | 'inactive'>('all');
	let pageNumber = $state(1);
	let creating = $state(false);

	setCrumbs([
		{ label: m.settings_title(), href: resolve('/settings') },
		{ label: m.settings_service_accounts() }
	]);

	let shown = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return accounts
			.filter((a) => (status === 'all' ? true : status === 'active' ? a.active : !a.active))
			.filter(
				(a) =>
					!q || a.name.toLowerCase().includes(q) || (a.description ?? '').toLowerCase().includes(q)
			)
			.sort((a, b) => a.name.localeCompare(b.name));
	});
	let pageRows = $derived(shown.slice((pageNumber - 1) * PAGE_SIZE, pageNumber * PAGE_SIZE));
	let filtered = $derived(!!query.trim() || status !== 'all');

	async function fetchAccounts() {
		loading = true;
		error = '';
		try {
			accounts = await listServiceAccounts();
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_list();
		} finally {
			loading = false;
		}
	}

	async function remove(sa: ServiceAccount) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.serviceaccounts_delete_heading({ name: sa.name }),
			body: m.serviceaccounts_delete_body(),
			confirmLabel: m.serviceaccounts_delete_button(),
			confirmText: sa.name
		});
		if (!ok) return;
		try {
			await deleteServiceAccount(sa.id);
			toasts.success(m.serviceaccounts_deleted_success({ name: sa.name }));
			accounts = accounts.filter((a) => a.id !== sa.id);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_delete_account());
		}
	}

	let columns = $derived<DataTableColumn<ServiceAccount>[]>([
		{
			key: 'name',
			header: m.serviceaccounts_col_account(),
			width: 200,
			href: (a) => resolve('/settings/service-accounts/[id]', { id: a.id })
		},
		{ key: 'description', header: m.common_description(), width: 340 },
		{
			key: 'roles',
			header: m.users_header_roles(),
			width: 180,
			value: (a) => (a.roles ?? []).map((r) => roleLabel(r.name)).join(', '),
			empty: m.serviceaccounts_no_roles()
		},
		{
			key: 'used_by',
			header: m.serviceaccounts_col_used_by(),
			width: 220,
			value: (a) => (a.used_by ?? []).map((g) => g.name).join(', '),
			render: (a) =>
				a.used_by?.length
					? html`<span class="mm-cluster" style="--mm-gap:var(--space-2)"
							>${a.used_by.map(
								(g) =>
									html`<mm-owner-chip
										name=${g.name}
										kind="agent"
										href=${assetHref(g)}
									></mm-owner-chip>`
							)}</span
						>`
					: html`<span class="mm-muted">${m.serviceaccounts_used_by_none()}</span>`
		},
		{ key: 'api_key_count', header: m.serviceaccounts_col_keys(), type: 'number', width: 100 },
		{
			key: 'last_used_at',
			header: m.serviceaccounts_col_last_used(),
			type: 'date',
			width: 130,
			empty: m.common_never()
		},
		{
			key: 'active',
			header: m.common_status(),
			width: 120,
			value: (a) => (a.active ? m.common_active() : m.common_inactive()),
			render: (a) =>
				a.active ? m.common_active() : html`<span class="mm-muted">${m.common_inactive()}</span>`
		},
		{
			key: 'actions',
			header: m.common_actions(),
			width: 72,
			minWidth: 72,
			align: 'end',
			render: (a) =>
				canManage
					? rowMenu(
							m.serviceaccounts_actions_aria({ name: a.name }),
							[
								{
									value: 'delete',
									label: m.serviceaccounts_delete_button(),
									icon: 'delete',
									danger: true
								}
							],
							() => remove(a)
						)
					: nothing
		}
	]);

	onMount(() => {
		fetchAccounts();
		// The old /service-accounts/new route redirects here with ?create=1, so open the drawer and drop the flag so a refresh does not reopen it
		if (page.url.searchParams.get('create') === '1') {
			creating = true;
			goto(resolve('/settings/service-accounts'), {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});
</script>

<svelte:head>
	<title>{m.settings_service_accounts()} · {m.settings_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header
		heading={m.settings_service_accounts()}
		compact
		count={loading || error ? undefined : accounts.length}
		description={m.serviceaccounts_page_description()}
	>
		{#if canManage && (accounts.length > 0 || loading || error)}
			<mm-button slot="actions" variant="primary" icon="add" onclick={() => (creating = true)}>
				{m.serviceaccounts_create_button()}
			</mm-button>
		{/if}
	</mm-page-header>

	{#if !loading && !error && accounts.length === 0}
		<mm-empty-state
			heading={m.serviceaccounts_none_yet()}
			description={m.serviceaccounts_empty_hint()}
			seed={4}
		>
			{#if canManage}
				<mm-button slot="action" variant="primary" icon="add" onclick={() => (creating = true)}>
					{m.serviceaccounts_create_button()}
				</mm-button>
			{/if}
		</mm-empty-state>
	{:else}
		<div class="mm-cluster" style="align-items: end">
			<mm-search-field
				style="width: 20rem; max-width: 100%"
				label={m.serviceaccounts_search_label()}
				hide-label
				placeholder={m.serviceaccounts_search_by_name()}
				value={query}
				onmm-input={(e: CustomEvent<MmValueDetail>) => {
					query = e.detail.value;
					pageNumber = 1;
				}}
			></mm-search-field>
			<mm-select
				style="width: 12rem"
				label={m.common_status()}
				hide-label
				value={status}
				onmm-change={(e: CustomEvent<MmValueDetail>) => {
					status = e.detail.value as typeof status;
					pageNumber = 1;
				}}
			>
				<mm-option value="all" label={m.members_any_status()}></mm-option>
				<mm-option value="active" label={m.common_active()}></mm-option>
				<mm-option value="inactive" label={m.common_inactive()}></mm-option>
			</mm-select>
		</div>

		<mm-data-table
			label={m.settings_service_accounts()}
			{columns}
			rows={pageRows}
			loading={loading && accounts.length === 0}
			{error}
			empty-heading={filtered ? m.serviceaccounts_no_match() : m.serviceaccounts_none_yet()}
			empty-text={filtered ? m.serviceaccounts_no_match_body() : ''}
			onmm-retry={fetchAccounts}
		></mm-data-table>

		{#if shown.length > PAGE_SIZE}
			<mm-pagination
				label={m.serviceaccounts_pagination_label()}
				page={pageNumber}
				page-size={PAGE_SIZE}
				total={shown.length}
				onmm-page-change={(e: CustomEvent<MmPageChangeDetail>) => (pageNumber = e.detail.page)}
			></mm-pagination>
		{/if}
	{/if}
</div>

{#if creating}
	<CreateServiceAccountDrawer
		onClose={() => (creating = false)}
		onCreated={(sa) => {
			creating = false;
			goto(resolve('/settings/service-accounts/[id]', { id: sa.id }));
		}}
	/>
{/if}
