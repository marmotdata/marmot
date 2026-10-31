<script lang="ts">
	import { untrack } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { TeamRow } from '$lib/teams/types';

	// Renames a team or changes its description; tags and metadata are edited in place on the team page
	let {
		team,
		onClose,
		onSaved
	}: { team: TeamRow; onClose: () => void; onSaved: (team: TeamRow) => void } = $props();

	let name = $state(untrack(() => team.name));
	let description = $state(untrack(() => team.description ?? ''));
	let saving = $state(false);
	let error = $state('');
	let nameError = $state('');

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		const n = name.trim();
		nameError = !n
			? m.teams_error_name_required()
			: n.length < 2
				? m.teams_error_name_too_short()
				: '';
		if (nameError) return;
		saving = true;
		error = '';
		try {
			const response = await fetchApi(`/teams/${team.id}`, {
				method: 'PUT',
				body: JSON.stringify({ name: n, description: description.trim() })
			});
			if (!response.ok) {
				error = await handleApiError(response);
				return;
			}
			toasts.success(m.teams_saved_toast());
			onSaved({ ...team, name: n, description: description.trim() });
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_generic();
		} finally {
			saving = false;
		}
	}
</script>

<mm-drawer open heading={m.teams_edit_team()} onmm-close={onClose}>
	<form
		id="edit-team"
		class="mm-stack"
		style="--mm-gap: var(--space-4)"
		novalidate
		onsubmit={handleSubmit}
	>
		{#if error}
			<mm-callout tone="danger" heading={m.teams_save_failed()}>{error}</mm-callout>
		{/if}
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
			help={m.teams_description_help()}
			onmm-input={(e: CustomEvent<MmValueDetail>) => (description = e.detail.value)}
		></mm-textarea>
	</form>
	<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
	<mm-button slot="footer" variant="primary" type="submit" form="edit-team" loading={saving}>
		{m.serviceaccounts_save_changes()}
	</mm-button>
</mm-drawer>
