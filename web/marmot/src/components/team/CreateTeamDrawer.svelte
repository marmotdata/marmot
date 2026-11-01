<script lang="ts">
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/key-value-editor.js';
	import {
		pairsToObject,
		type KeyValuePair
	} from '@marmotdata/elements/classes/key-value-editor.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import TagsInput from '$components/shared/TagsInput.svelte';
	import TeamMemberPicker from './TeamMemberPicker.svelte';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { formatList } from '$lib/utils';
	import type { OwnerResult } from '$lib/teams/types';

	// Creating a team asks for its name first; members, tags and metadata are optional groups that start as one Add button each. Webhooks are added from the team page once it exists.
	let {
		onClose,
		onCreated
	}: { onClose: () => void; onCreated: (team: { id: string; name: string }) => void } = $props();

	let name = $state('');
	let description = $state('');
	let members = $state<(OwnerResult & { role: 'owner' | 'member' })[]>([]);
	let tags = $state<string[]>([]);
	let pairs = $state<KeyValuePair[]>([]);
	let showMembers = $state(false);
	let showTags = $state(false);
	let showMetadata = $state(false);
	let saving = $state(false);
	let error = $state('');
	let nameError = $state('');

	function checkName(): boolean {
		const n = name.trim();
		nameError = !n
			? m.teams_error_name_required()
			: n.length < 2
				? m.teams_error_name_too_short()
				: '';
		return !nameError;
	}

	function removeMember(id: string) {
		members = members.filter((p) => p.id !== id);
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (!checkName()) return;
		saving = true;
		try {
			const createResp = await fetchApi('/teams', {
				method: 'POST',
				body: JSON.stringify({ name: name.trim(), description: description.trim() })
			});
			if (!createResp.ok) {
				error = await handleApiError(createResp);
				return;
			}
			const team = await createResp.json();
			const failed: string[] = [];

			const metadata = pairsToObject(pairs.filter((p) => p.key.trim()));
			const hasMeta = Object.keys(metadata).length > 0;
			if (tags.length || hasMeta) {
				const patch: Record<string, unknown> = {};
				if (tags.length) patch.tags = tags;
				if (hasMeta) patch.metadata = metadata;
				const patchResp = await fetchApi(`/teams/${team.id}`, {
					method: 'PUT',
					body: JSON.stringify(patch)
				});
				if (!patchResp.ok) failed.push(m.teams_failed_item_tags_metadata());
			}

			for (const member of members) {
				const memResp = await fetchApi(`/teams/${team.id}/members`, {
					method: 'POST',
					body: JSON.stringify({ user_id: member.id, role: member.role })
				});
				if (!memResp.ok) failed.push(m.teams_failed_item_member({ name: member.name }));
			}

			if (failed.length)
				toasts.warning(m.teams_created_with_warnings({ items: formatList(failed) }));
			else toasts.success(m.teams_created_toast({ name: team.name }));
			onCreated(team);
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_create();
		} finally {
			saving = false;
		}
	}
</script>

<mm-drawer open heading={m.teams_create_team_button()} onmm-close={onClose}>
	<form
		id="create-team"
		class="mm-stack"
		style="--mm-gap: var(--space-6)"
		novalidate
		onsubmit={handleSubmit}
	>
		{#if error}
			<mm-callout tone="danger" heading={m.teams_create_failed()}>{error}</mm-callout>
		{/if}
		<div class="mm-stack" style="--mm-gap: var(--space-4)">
			<mm-text-field
				label={m.common_name()}
				name="name"
				required
				value={name}
				error={nameError}
				oninput={(e: Event) => (name = (e.currentTarget as unknown as { value: string }).value)}
				onmm-change={checkName}
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
		</div>

		{#if showMembers}
			<mm-section heading={m.teams_members_heading()}>
				<div class="mm-stack" style="--mm-gap: var(--space-4)">
					{#if members.length}
						<mm-list aria-label={m.teams_chosen_members()}>
							{#each members as person (person.id)}
								<mm-list-item secondary={person.email ?? ''}>
									<mm-avatar
										slot="start"
										name={person.name}
										src={person.profile_picture ?? ''}
										decorative
									></mm-avatar>
									{person.name}
									<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-1)">
										<mm-select
											style="width: 8rem"
											label={m.teams_team_role_for({ name: person.name })}
											hide-label
											value={person.role}
											onmm-change={(e: CustomEvent<MmValueDetail>) =>
												(person.role = e.detail.value as 'owner' | 'member')}
										>
											<mm-option value="member" label={m.teams_role_member()}></mm-option>
											<mm-option value="owner" label={m.common_owner()}></mm-option>
										</mm-select>
										<mm-button
											size="sm"
											variant="ghost"
											icon="close"
											icon-only
											label={m.teams_remove_person_aria({ name: person.name })}
											onclick={() => removeMember(person.id)}
										></mm-button>
									</span>
								</mm-list-item>
							{/each}
						</mm-list>
					{/if}
					<TeamMemberPicker
						exclude={members.map((p) => p.id)}
						onAdd={(person) => (members = [...members, { ...person, role: 'member' }])}
					/>
				</div>
			</mm-section>
		{/if}

		{#if showTags}
			<mm-section heading={m.common_tags()}>
				<TagsInput bind:tags placeholder={m.teams_tags_placeholder()} />
			</mm-section>
		{/if}

		{#if showMetadata}
			<mm-key-value-editor
				label={m.teams_metadata_heading()}
				value={pairs}
				json
				onmm-change={(e: CustomEvent<{ value: KeyValuePair[] }>) => (pairs = e.detail.value)}
			></mm-key-value-editor>
		{/if}

		{#if !showMembers || !showTags || !showMetadata}
			<div class="mm-cluster" style="--mm-gap: var(--space-2)">
				{#if !showMembers}
					<mm-button size="sm" icon="add" onclick={() => (showMembers = true)}>
						{m.teams_add_members()}
					</mm-button>
				{/if}
				{#if !showTags}
					<mm-button size="sm" icon="add" onclick={() => (showTags = true)}
						>{m.teams_add_tags()}</mm-button
					>
				{/if}
				{#if !showMetadata}
					<mm-button size="sm" icon="add" onclick={() => (showMetadata = true)}>
						{m.teams_add_metadata()}
					</mm-button>
				{/if}
			</div>
		{/if}
	</form>
	<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
	<mm-button
		slot="footer"
		variant="primary"
		type="submit"
		form="create-team"
		icon="group-add"
		loading={saving}
	>
		{m.teams_create_team_button()}
	</mm-button>
</mm-drawer>
