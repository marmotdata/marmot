<script lang="ts">
	import { onMount } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/multiselect.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import { createServiceAccount } from '$lib/serviceaccounts/api';
	import { listRoles } from '$lib/roles/api';
	import { toasts, ApiError, isLimitExceeded } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/users/format';
	import type { Role } from '$lib/roles/types';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';

	let { onClose, onCreated }: { onClose: () => void; onCreated: (sa: ServiceAccount) => void } =
		$props();

	let name = $state('');
	let description = $state('');
	let roleIds = $state<string[]>([]);
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

	function checkName(): boolean {
		const n = name.trim();
		nameError = !n
			? m.serviceaccounts_error_name_required()
			: n.length < 2
				? m.serviceaccounts_error_name_min()
				: '';
		return !nameError;
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (!checkName()) return;
		saving = true;
		try {
			const sa = await createServiceAccount({
				name: name.trim(),
				description: description.trim() || undefined,
				role_ids: roleIds.length ? roleIds : undefined
			});
			toasts.success(m.serviceaccounts_create_success({ name: sa.name }));
			onCreated(sa);
		} catch (err) {
			error = err instanceof Error ? err.message : m.serviceaccounts_error_create();
			if (err instanceof ApiError && isLimitExceeded(err)) toasts.warning(err.message);
		} finally {
			saving = false;
		}
	}
</script>

<mm-drawer open heading={m.serviceaccounts_create_button()} onmm-close={onClose}>
	<form
		id="create-service-account"
		class="mm-stack"
		style="--mm-gap: var(--space-6)"
		novalidate
		onsubmit={handleSubmit}
	>
		{#if error}
			<mm-callout tone="danger" heading={m.serviceaccounts_create_failed()}>{error}</mm-callout>
		{/if}
		<div class="mm-stack" style="--mm-gap: var(--space-4)">
			<mm-text-field
				label={m.common_name()}
				name="name"
				required
				value={name}
				error={nameError}
				help={m.serviceaccounts_name_help()}
				oninput={(e: Event) => (name = (e.currentTarget as unknown as { value: string }).value)}
				onmm-change={checkName}
			></mm-text-field>
			<mm-textarea
				label={m.common_description()}
				name="description"
				optional
				rows={3}
				value={description}
				help={m.serviceaccounts_description_help()}
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
	</form>
	<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
	<mm-button
		slot="footer"
		variant="primary"
		type="submit"
		form="create-service-account"
		icon="add"
		loading={saving}
	>
		{m.serviceaccounts_create_button()}
	</mm-button>
</mm-drawer>
