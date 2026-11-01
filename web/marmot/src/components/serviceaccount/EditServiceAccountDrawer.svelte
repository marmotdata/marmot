<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/multiselect.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import type { MmSwitchDetail } from '@marmotdata/elements/classes/switch.js';
	import { updateServiceAccount } from '$lib/serviceaccounts/api';
	import { listRoles } from '$lib/roles/api';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/users/format';
	import type { Role } from '$lib/roles/types';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';

	let {
		sa,
		onClose,
		onSaved
	}: { sa: ServiceAccount; onClose: () => void; onSaved: (sa: ServiceAccount) => void } = $props();

	// The drawer edits a copy taken when it opens
	let name = $state(untrack(() => sa.name));
	let description = $state(untrack(() => sa.description ?? ''));
	let active = $state(untrack(() => sa.active));
	let roleIds = $state(untrack(() => sa.roles.map((r) => r.id)));
	let roles = $state<Role[]>([]);
	let saving = $state(false);
	let error = $state('');
	let nameError = $state('');

	onMount(async () => {
		try {
			roles = await listRoles();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_load_roles());
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		const n = name.trim();
		nameError = !n
			? m.serviceaccounts_error_name_required()
			: n.length < 2
				? m.serviceaccounts_error_name_min()
				: '';
		if (nameError) return;
		saving = true;
		error = '';
		try {
			const updated = await updateServiceAccount(sa.id, {
				name: n,
				description: description.trim() || undefined,
				active,
				role_ids: roleIds
			});
			toasts.success(m.serviceaccounts_updated_success());
			onSaved(updated);
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_update();
		} finally {
			saving = false;
		}
	}
</script>

<mm-drawer open heading={m.serviceaccounts_edit_button()} onmm-close={onClose}>
	<form
		id="edit-service-account"
		class="mm-stack"
		style="--mm-gap: var(--space-6)"
		novalidate
		onsubmit={handleSubmit}
	>
		{#if error}
			<mm-callout tone="danger" heading={m.serviceaccounts_save_failed()}>{error}</mm-callout>
		{/if}
		<div class="mm-stack" style="--mm-gap: var(--space-4)">
			<mm-text-field
				label={m.common_name()}
				name="name"
				required
				value={name}
				error={nameError}
				oninput={(e: Event) => (name = (e.currentTarget as unknown as { value: string }).value)}
			></mm-text-field>
			<mm-textarea
				label={m.common_description()}
				name="description"
				optional
				rows={3}
				value={description}
				onmm-input={(e: CustomEvent<MmValueDetail>) => (description = e.detail.value)}
			></mm-textarea>
		</div>
		<mm-multiselect
			label={m.users_header_roles()}
			name="roles"
			optional
			value={roleIds}
			help={m.serviceaccounts_roles_help()}
			onmm-change={(e: CustomEvent<{ value: string[] }>) => (roleIds = e.detail.value)}
		>
			{#each roles as role (role.id)}
				<mm-option value={role.id} label={roleLabel(role.name)} description={role.description}
				></mm-option>
			{/each}
		</mm-multiselect>
		<mm-switch
			label={m.common_active()}
			description={m.serviceaccounts_active_hint()}
			checked={active}
			onmm-change={(e: CustomEvent<MmSwitchDetail>) => (active = e.detail.checked)}
		></mm-switch>
	</form>
	<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
	<mm-button
		slot="footer"
		variant="primary"
		type="submit"
		form="edit-service-account"
		loading={saving}
	>
		{m.serviceaccounts_save_changes()}
	</mm-button>
</mm-drawer>
