<script lang="ts">
	import Blade from '$components/ui/Blade.svelte';
	import PermissionEditor from '$lib/components/PermissionEditor.svelte';
	import { createRole } from '$lib/roles/api';
	import { m } from '$lib/paraglide/messages';
	import { toasts } from '$lib/stores/toast';

	interface Props {
		onClose: () => void;
		onCreated: () => void;
	}

	let { onClose, onCreated }: Props = $props();

	const formId = 'create-role-form';
	const sectionClass =
		'text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400';
	const labelClass = 'block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1';
	const inputClass =
		'w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-earthy-terracotta-500 focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100';

	let name = $state('');
	let description = $state('');
	let selectedPermIds = $state<string[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);

	let nameValid = $derived(name.trim().length >= 2);
	let dirty = $derived(!!(name || description || selectedPermIds.length));

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = null;
		if (!name.trim()) {
			error = m.roles_error_name_required();
			return;
		}
		if (!nameValid) {
			error = m.roles_error_name_min();
			return;
		}
		try {
			saving = true;
			await createRole({
				name: name.trim(),
				description: description.trim() || undefined,
				permission_ids: selectedPermIds
			});
			toasts.success(m.roles_create_success({ name: name.trim() }));
			onCreated();
		} catch (err) {
			error = err instanceof Error ? err.message : m.roles_error_create();
		} finally {
			saving = false;
		}
	}
</script>

<Blade title={m.roles_create_title()} {onClose} width="2xl" {dirty}>
	<form id={formId} onsubmit={handleSubmit} class="space-y-6">
		{#if error}
			<div
				class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
			>
				{error}
			</div>
		{/if}

		<section class="space-y-4">
			<h3 class={sectionClass}>{m.roles_basic_information_heading()}</h3>
			<div>
				<label for="role-name" class={labelClass}>
					{m.common_name()} <span class="text-red-500">*</span>
				</label>
				<input
					id="role-name"
					type="text"
					bind:value={name}
					placeholder={m.roles_name_placeholder()}
					class={inputClass}
					required
				/>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{m.roles_name_hint()}</p>
			</div>
			<div>
				<label for="role-description" class={labelClass}>{m.common_description()}</label>
				<textarea
					id="role-description"
					bind:value={description}
					rows="3"
					placeholder={m.roles_description_placeholder()}
					class="{inputClass} resize-none"
				></textarea>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
					{m.roles_description_hint()}
				</p>
			</div>
		</section>

		<section class="space-y-3">
			<div class="flex items-center justify-between">
				<h3 class={sectionClass}>
					{m.roles_step_permissions()}
					<span class="ml-1 font-normal normal-case tracking-normal"
						>{m.roles_optional_badge()}</span
					>
				</h3>
				<span class="text-xs text-gray-500 dark:text-gray-400">
					{m.roles_selected_count({ count: selectedPermIds.length })}
				</span>
			</div>
			<p class="text-sm text-gray-600 dark:text-gray-400">{m.roles_permissions_intro()}</p>
			<PermissionEditor selectedIds={selectedPermIds} onChange={(ids) => (selectedPermIds = ids)} />
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
				{saving ? m.roles_creating_label() : m.roles_create_title()}
			</button>
		</div>
	{/snippet}
</Blade>
