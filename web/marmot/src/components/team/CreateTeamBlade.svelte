<script lang="ts">
	import Blade from '$components/ui/Blade.svelte';
	import TagsInput from '$components/shared/TagsInput.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import TeamMemberPicker, { type Owner } from './TeamMemberPicker.svelte';
	import TeamWebhookEditor, { type PendingWebhook } from './TeamWebhookEditor.svelte';
	import { fetchApi } from '$lib/api';
	import { toasts, handleApiError } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import { formatList } from '$lib/utils';
	import { type CreateWebhookInput } from '$lib/teams/webhooks';

	interface Props {
		onClose: () => void;
		onCreated: (team: { id: string; name: string }) => void;
	}

	let { onClose, onCreated }: Props = $props();

	const formId = 'create-team-form';
	const sectionClass =
		'text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400';
	const labelClass = 'block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1';
	const inputClass =
		'w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-earthy-terracotta-500 focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100';

	let name = $state('');
	let description = $state('');
	let members = $state<Owner[]>([]);
	let memberRoles = $state<Record<string, 'owner' | 'member'>>({});
	let tags = $state<string[]>([]);
	let metadata = $state<Record<string, unknown>>({});
	let webhooks = $state<PendingWebhook[]>([]);
	let saving = $state(false);
	let error = $state<string | null>(null);

	let nameValid = $derived(name.trim().length >= 2);
	let webhooksValid = $derived(
		webhooks.every(
			(w) =>
				w.name.trim().length > 0 &&
				w.webhook_url.trim().length > 0 &&
				w.notification_types.length > 0
		)
	);
	let dirty = $derived(
		!!(
			name ||
			description ||
			members.length ||
			tags.length ||
			Object.keys(metadata ?? {}).length ||
			webhooks.length
		)
	);

	// The tag, metadata and integration editors use Enter for their own rows, so it must not also submit the team; the listener is attached directly because a form is not an interactive element for a11y purposes
	let formEl = $state<HTMLFormElement>();
	$effect(() => {
		if (!formEl) return;
		const blockImplicitSubmit = (e: KeyboardEvent) => {
			if (
				e.key === 'Enter' &&
				e.target instanceof HTMLInputElement &&
				e.target.closest('[data-no-implicit-submit]')
			) {
				e.preventDefault();
			}
		};
		formEl.addEventListener('keydown', blockImplicitSubmit);
		return () => formEl?.removeEventListener('keydown', blockImplicitSubmit);
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = null;
		if (!name.trim()) {
			error = m.teams_error_name_required();
			return;
		}
		if (!nameValid) {
			error = m.teams_error_name_too_short();
			return;
		}
		if (!webhooksValid) {
			error = m.teams_error_integration_incomplete();
			return;
		}

		try {
			saving = true;

			const createResp = await fetchApi('/teams', {
				method: 'POST',
				body: JSON.stringify({ name: name.trim(), description: description.trim() })
			});
			if (!createResp.ok) {
				error = await handleApiError(createResp);
				return;
			}
			const team = await createResp.json();

			const warnings: string[] = [];

			const hasMeta = Object.keys(metadata ?? {}).length > 0;
			if (tags.length > 0 || hasMeta) {
				const patch: Record<string, unknown> = {};
				if (tags.length > 0) patch.tags = tags;
				if (hasMeta) patch.metadata = metadata;
				const patchResp = await fetchApi(`/teams/${team.id}`, {
					method: 'PUT',
					body: JSON.stringify(patch)
				});
				if (!patchResp.ok) warnings.push(m.teams_failed_item_tags_metadata());
			}

			for (const member of members) {
				const role = memberRoles[member.id] ?? 'member';
				const memResp = await fetchApi(`/teams/${team.id}/members`, {
					method: 'POST',
					body: JSON.stringify({ user_id: member.id, role })
				});
				if (!memResp.ok) warnings.push(m.teams_failed_item_member({ name: member.name }));
			}

			for (const w of webhooks) {
				const payload: CreateWebhookInput = {
					name: w.name.trim(),
					provider: w.provider,
					webhook_url: w.webhook_url.trim(),
					notification_types: w.notification_types,
					enabled: w.enabled
				};
				const hookResp = await fetchApi(`/teams/${team.id}/webhooks`, {
					method: 'POST',
					body: JSON.stringify(payload)
				});
				if (!hookResp.ok) warnings.push(m.teams_failed_item_webhook({ name: w.name }));
			}

			if (warnings.length > 0) {
				toasts.warning(m.teams_created_with_warnings({ items: formatList(warnings) }));
			} else {
				toasts.success(m.teams_created_toast({ name: team.name }));
			}

			onCreated(team);
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_error_create();
		} finally {
			saving = false;
		}
	}
</script>

<Blade title={m.teams_create_team()} {onClose} width="2xl" {dirty}>
	<form id={formId} onsubmit={handleSubmit} bind:this={formEl} class="space-y-8">
		{#if error}
			<div
				class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
			>
				{error}
			</div>
		{/if}

		<section class="space-y-4">
			<h3 class={sectionClass}>{m.teams_basic_info_heading()}</h3>
			<div>
				<label for="team-name" class={labelClass}>
					{m.teams_name_label()} <span class="text-red-500">*</span>
				</label>
				<input
					id="team-name"
					type="text"
					bind:value={name}
					placeholder={m.teams_name_placeholder()}
					class={inputClass}
					required
				/>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{m.teams_name_hint()}</p>
			</div>
			<div>
				<label for="team-description" class={labelClass}>{m.common_description()}</label>
				<textarea
					id="team-description"
					bind:value={description}
					rows="3"
					placeholder={m.teams_description_placeholder()}
					class="{inputClass} resize-none"
				></textarea>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
					{m.teams_description_hint()}
				</p>
			</div>
		</section>

		<section class="space-y-3" data-no-implicit-submit>
			<div class="flex items-center justify-between">
				<h3 class={sectionClass}>
					{m.teams_members_heading()}
					<span class="ml-1 font-normal normal-case tracking-normal"
						>{m.teams_optional_suffix()}</span
					>
				</h3>
				<span class="text-xs text-gray-500 dark:text-gray-400">
					{m.teams_members_to_add_count({ count: members.length })}
				</span>
			</div>
			<p class="text-sm text-gray-600 dark:text-gray-400">{m.teams_members_hint()}</p>

			<TeamMemberPicker bind:members bind:roles={memberRoles} />
		</section>

		<section class="space-y-3" data-no-implicit-submit>
			<div>
				<h3 class={sectionClass}>
					{m.common_tags()}
					<span class="ml-1 font-normal normal-case tracking-normal"
						>{m.teams_optional_suffix()}</span
					>
				</h3>
				<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{m.teams_tags_hint()}</p>
			</div>
			<TagsInput bind:tags placeholder={m.teams_tags_placeholder()} />
		</section>

		<section class="space-y-3" data-no-implicit-submit>
			<div>
				<h3 class={sectionClass}>
					{m.teams_metadata_heading()}
					<span class="ml-1 font-normal normal-case tracking-normal"
						>{m.teams_optional_suffix()}</span
					>
				</h3>
				<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{m.teams_metadata_hint()}</p>
			</div>
			<MetadataView bind:metadata />
		</section>

		<section class="space-y-3" data-no-implicit-submit>
			<div class="flex items-center justify-between">
				<h3 class={sectionClass}>
					{m.teams_integrations_heading()}
					<span class="ml-1 font-normal normal-case tracking-normal"
						>{m.teams_optional_suffix()}</span
					>
				</h3>
				<span class="text-xs text-gray-500 dark:text-gray-400">
					{m.teams_webhooks_configured_count({ count: webhooks.length })}
				</span>
			</div>
			<p class="text-sm text-gray-600 dark:text-gray-400">{m.teams_integrations_hint()}</p>

			<TeamWebhookEditor bind:webhooks />
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
				disabled={saving || !nameValid || !webhooksValid}
				class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:opacity-50"
			>
				{saving ? m.teams_creating() : m.teams_create_team()}
			</button>
		</div>
	{/snippet}
</Blade>
