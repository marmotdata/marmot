<script lang="ts">
	import { onMount } from 'svelte';
	import { html } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
	import type { MmDialog } from '@marmotdata/elements/dialog.js';
	import { createGroupMapping, deleteGroupMapping, listGroupMappings } from '$lib/sso/api';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { providerName, roleLabel } from '$lib/users/format';
	import type { GroupMapping, SSOProviderStatus } from '$lib/sso/types';
	import type { Role } from '$lib/roles/types';
	import type { Team } from '$lib/teams/types';

	// Groups from the identity provider, each giving a role, a team or both. They are applied at every sign-in, and a mapped role replaces the default one.
	let {
		providers,
		roles,
		teams,
		onChange
	}: {
		providers: SSOProviderStatus[];
		roles: Role[];
		teams: Team[];
		onChange?: (mappings: GroupMapping[]) => void;
	} = $props();

	let mappings = $state<GroupMapping[]>([]);
	let loading = $state(true);
	let error = $state('');
	let dialog = $state<MmDialog>();
	let provider = $state('');
	let group = $state('');
	let roleId = $state('');
	let teamId = $state('');
	let formError = $state('');
	let groupError = $state('');
	let saving = $state(false);

	let multipleProviders = $derived(providers.length > 1);

	async function load() {
		loading = true;
		error = '';
		try {
			mappings = await listGroupMappings();
			onChange?.(mappings);
		} catch (err) {
			error = err instanceof Error ? err.message : m.sso_mappings_load_error();
		} finally {
			loading = false;
		}
	}

	function openAdd() {
		provider = providers[0]?.type ?? '';
		group = '';
		roleId = '';
		teamId = '';
		formError = '';
		groupError = '';
		dialog?.show();
	}

	async function save(e?: Event) {
		e?.preventDefault();
		groupError = group.trim() ? '' : m.sso_mapping_error_group();
		formError = roleId || teamId ? '' : m.sso_mapping_error_target();
		if (groupError || formError) return;
		saving = true;
		try {
			await createGroupMapping({
				provider,
				sso_group_name: group.trim(),
				role_id: roleId || undefined,
				team_id: teamId || undefined
			});
			dialog?.close();
			toasts.success(m.sso_mapping_added_toast({ group: group.trim() }));
			await load();
		} catch (err) {
			formError = err instanceof Error ? err.message : m.sso_mapping_add_failed();
		} finally {
			saving = false;
		}
	}

	async function remove(mapping: GroupMapping) {
		const ok = await MmConfirmDialog.confirm({
			heading: m.sso_mapping_delete_heading({ group: mapping.sso_group_name }),
			body: m.sso_mapping_delete_body(),
			confirmLabel: m.sso_mapping_delete()
		});
		if (!ok) return;
		try {
			await deleteGroupMapping(mapping.id);
			toasts.success(m.sso_mapping_deleted_toast({ group: mapping.sso_group_name }));
			await load();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.sso_mapping_delete_failed());
		}
	}

	let columns = $derived<DataTableColumn<GroupMapping>[]>([
		{ key: 'sso_group_name', header: m.sso_mapping_col_group(), type: 'mono', width: 260 },
		...(multipleProviders
			? [
					{
						key: 'provider',
						header: m.sso_column_provider(),
						width: 140,
						value: (g: GroupMapping) => providerName(g.provider)
					}
				]
			: []),
		{
			key: 'role_name',
			header: m.sso_mapping_col_role(),
			width: 160,
			value: (g) => (g.role_name ? roleLabel(g.role_name) : m.sso_mapping_default_role())
		},
		{
			key: 'team_name',
			header: m.sso_mapping_col_team(),
			width: 240,
			value: (g) => g.team_name || m.sso_mapping_no_team()
		},
		{
			key: 'actions',
			header: m.common_actions(),
			width: 120,
			minWidth: 120,
			align: 'end',
			render: (g) =>
				html`<mm-button
					size="sm"
					variant="ghost"
					icon="delete"
					label=${m.sso_mapping_delete_aria({ group: g.sso_group_name })}
					@click=${() => remove(g)}
					>${m.common_delete()}</mm-button
				>`
		}
	]);

	onMount(load);

	const val = (e: Event) => (e.currentTarget as unknown as { value: string }).value;
</script>

<mm-section heading={m.sso_mappings_heading()} description={m.sso_mappings_description()}>
	{#if providers.length}
		<mm-button slot="actions" icon="add" onclick={openAdd}>{m.sso_mapping_add()}</mm-button>
	{/if}
	<mm-data-table
		label={m.sso_mappings_heading()}
		row-key="id"
		{columns}
		rows={mappings}
		{loading}
		{error}
		empty-heading={m.sso_mappings_empty_title()}
		empty-text={providers.length
			? m.sso_mappings_empty_body()
			: m.sso_mappings_empty_no_providers()}
		onmm-retry={load}
	></mm-data-table>
</mm-section>

<mm-dialog bind:this={dialog} heading={m.sso_mapping_add()} size="sm">
	<form class="mm-stack" style="--mm-gap: var(--space-5)" novalidate onsubmit={save}>
		{#if formError}
			<mm-callout tone="danger" compact>{formError}</mm-callout>
		{/if}
		{#if multipleProviders}
			<mm-select
				label={m.sso_column_provider()}
				value={provider}
				onmm-change={(e: CustomEvent<MmValueDetail>) => (provider = e.detail.value)}
			>
				{#each providers as p (p.type)}
					<mm-option value={p.type} label={p.name}></mm-option>
				{/each}
			</mm-select>
		{/if}
		<mm-text-field
			label={m.sso_mapping_col_group()}
			required
			value={group}
			error={groupError}
			help={m.sso_mapping_group_help()}
			oninput={(e: Event) => (group = val(e))}
		></mm-text-field>
		<mm-select
			label={m.sso_mapping_col_role()}
			value={roleId}
			help={m.sso_mapping_role_help()}
			onmm-change={(e: CustomEvent<MmValueDetail>) => (roleId = e.detail.value)}
		>
			<mm-option value="" label={m.sso_mapping_default_role()}></mm-option>
			{#each roles as role (role.id)}
				<mm-option value={role.id} label={roleLabel(role.name)}></mm-option>
			{/each}
		</mm-select>
		<mm-select
			label={m.sso_mapping_col_team()}
			value={teamId}
			onmm-change={(e: CustomEvent<MmValueDetail>) => (teamId = e.detail.value)}
		>
			<mm-option value="" label={m.sso_mapping_no_team()}></mm-option>
			{#each teams as team (team.id)}
				<mm-option value={team.id} label={team.name}></mm-option>
			{/each}
		</mm-select>
	</form>
	<mm-button slot="footer" data-close="cancel" disabled={saving}>{m.common_cancel()}</mm-button>
	<mm-button slot="footer" variant="primary" icon="add" loading={saving} onclick={save}
		>{m.sso_mapping_add()}</mm-button
	>
</mm-dialog>
