<script lang="ts">
	import { onMount } from 'svelte';
	import Blade from '$components/ui/Blade.svelte';
	import RoleSelector from '$lib/components/RoleSelector.svelte';
	import { createServiceAccount } from '$lib/serviceaccounts/api';
	import { listRoles } from '$lib/roles/api';
	import type { Role } from '$lib/roles/types';
	import type { ServiceAccount } from '$lib/serviceaccounts/types';
	import { toasts, ApiError, isLimitExceeded } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';

	interface Props {
		onClose: () => void;
		onCreated: (sa: ServiceAccount) => void;
	}

	let { onClose, onCreated }: Props = $props();

	const formId = 'create-service-account-form';
	const sectionClass =
		'text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400';
	const labelClass = 'block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1';
	const inputClass =
		'w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-earthy-terracotta-500 focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100';

	let name = $state('');
	let description = $state('');
	let selectedRoleIds = $state<string[]>([]);
	let availableRoles = $state<Role[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);

	let nameValid = $derived(name.trim().length >= 2);
	let dirty = $derived(!!(name || description || selectedRoleIds.length));

	onMount(async () => {
		try {
			availableRoles = await listRoles();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.serviceaccounts_error_load_roles());
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = null;
		if (!name.trim()) {
			error = m.serviceaccounts_error_name_required();
			return;
		}
		if (!nameValid) {
			error = m.serviceaccounts_error_name_min();
			return;
		}
		try {
			saving = true;
			const sa = await createServiceAccount({
				name: name.trim(),
				description: description.trim() || undefined,
				role_ids: selectedRoleIds.length ? selectedRoleIds : undefined
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

<Blade title={m.serviceaccounts_create_title()} {onClose} width="xl" {dirty}>
	<form id={formId} onsubmit={handleSubmit} class="space-y-6">
		{#if error}
			<div
				class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
			>
				{error}
			</div>
		{/if}

		<section class="space-y-4">
			<h3 class={sectionClass}>{m.serviceaccounts_basic_information_heading()}</h3>
			<div>
				<label for="sa-name" class={labelClass}>
					{m.common_name()} <span class="text-red-500">*</span>
				</label>
				<input
					id="sa-name"
					type="text"
					bind:value={name}
					placeholder={m.serviceaccounts_name_placeholder()}
					class={inputClass}
					required
				/>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
					{m.serviceaccounts_name_hint()}
				</p>
			</div>
			<div>
				<label for="sa-description" class={labelClass}>{m.common_description()}</label>
				<textarea
					id="sa-description"
					bind:value={description}
					rows="3"
					placeholder={m.serviceaccounts_description_placeholder()}
					class="{inputClass} resize-none"
				></textarea>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
					{m.serviceaccounts_description_hint()}
				</p>
			</div>
		</section>

		<section class="space-y-3">
			<div>
				<h3 class={sectionClass}>
					{m.serviceaccounts_assign_roles_heading()}
					<span class="ml-1 font-normal normal-case tracking-normal"
						>{m.serviceaccounts_optional_badge()}</span
					>
				</h3>
				<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
					{m.serviceaccounts_roles_intro()}
				</p>
			</div>
			<RoleSelector
				roles={availableRoles}
				selectedIds={selectedRoleIds}
				onChange={(ids) => (selectedRoleIds = ids)}
				emptyMessage={m.serviceaccounts_no_roles_available()}
			/>
		</section>
	</form>

	{#snippet footer(close)}
		<div class="flex items-center justify-end gap-2">
			<button
				type="button"
				onclick={close}
				class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
			>
				{m.common_cancel()}
			</button>
			<button
				type="submit"
				form={formId}
				disabled={saving || !nameValid}
				class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:opacity-50"
			>
				{saving ? m.serviceaccounts_creating_label() : m.serviceaccounts_create_title()}
			</button>
		</div>
	{/snippet}
</Blade>
