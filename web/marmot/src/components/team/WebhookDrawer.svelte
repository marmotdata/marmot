<script lang="ts">
	import { untrack } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/fieldset.js';
	import type { MmValueDetail } from '@marmotdata/elements/classes/form-field.js';
	import type { MmCheckedDetail } from '@marmotdata/elements/classes/checkbox.js';
	import type { MmSwitchDetail } from '@marmotdata/elements/classes/switch.js';
	import { fetchApi } from '$lib/api';
	import { handleApiError, toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import {
		notificationTypeOptions,
		providerOptions,
		type CreateWebhookInput,
		type TeamWebhook
	} from '$lib/teams/webhooks';

	// Adds or edits one team webhook. The API returns only a masked URL, so when editing the URL field starts empty and is sent only if replaced.
	let {
		teamId,
		webhook = null,
		onClose,
		onSaved
	}: {
		teamId: string;
		webhook?: TeamWebhook | null;
		onClose: () => void;
		onSaved: () => void;
	} = $props();

	let form = $state<CreateWebhookInput>(
		untrack(() => ({
			name: webhook?.name ?? '',
			provider: webhook?.provider ?? 'slack',
			webhook_url: '',
			notification_types: [...(webhook?.notification_types ?? [])],
			enabled: webhook?.enabled ?? true
		}))
	);
	let saving = $state(false);
	let error = $state('');
	let nameError = $state('');
	let urlError = $state('');
	let typesError = $state('');

	function toggleType(type: string, on: boolean) {
		form.notification_types = on
			? [...form.notification_types, type]
			: form.notification_types.filter((t) => t !== type);
		typesError = '';
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		nameError = form.name.trim() ? '' : m.teams_error_name_required();
		urlError = !webhook && !form.webhook_url.trim() ? m.teams_webhook_url_required() : '';
		typesError = form.notification_types.length ? '' : m.teams_webhook_types_required();
		if (nameError || urlError || typesError) return;

		saving = true;
		error = '';
		try {
			const response = webhook
				? await fetchApi(`/teams/${teamId}/webhooks/${webhook.id}`, {
						method: 'PUT',
						body: JSON.stringify({
							name: form.name.trim(),
							webhook_url: form.webhook_url.trim() || undefined,
							notification_types: form.notification_types,
							enabled: form.enabled
						})
					})
				: await fetchApi(`/teams/${teamId}/webhooks`, {
						method: 'POST',
						body: JSON.stringify({
							...form,
							name: form.name.trim(),
							webhook_url: form.webhook_url.trim()
						})
					});
			if (!response.ok) {
				error = await handleApiError(response);
				return;
			}
			toasts.success(webhook ? m.teams_webhook_saved_toast() : m.teams_webhook_added_toast());
			onSaved();
		} catch (err) {
			error = err instanceof Error ? err.message : m.teams_webhook_error_save();
		} finally {
			saving = false;
		}
	}
</script>

<mm-drawer
	open
	heading={webhook ? m.teams_webhook_edit_title() : m.teams_add_webhook()}
	onmm-close={onClose}
>
	<form
		id="team-webhook"
		class="mm-stack"
		style="--mm-gap: var(--space-6)"
		novalidate
		onsubmit={handleSubmit}
	>
		{#if error}
			<mm-callout tone="danger" heading={m.teams_webhook_error_save()}>{error}</mm-callout>
		{/if}
		<mm-text-field
			label={m.common_name()}
			name="name"
			required
			value={form.name}
			error={nameError}
			placeholder={m.teams_webhook_name_placeholder()}
			oninput={(e: Event) => (form.name = (e.currentTarget as unknown as { value: string }).value)}
		></mm-text-field>
		{#if !webhook}
			<mm-segmented-control
				label={m.teams_webhook_send_to()}
				value={form.provider}
				onmm-change={(e: CustomEvent<MmValueDetail>) =>
					(form.provider = e.detail.value as CreateWebhookInput['provider'])}
			>
				{#each providerOptions() as provider (provider.value)}
					<mm-segment value={provider.value}>{provider.label}</mm-segment>
				{/each}
			</mm-segmented-control>
		{/if}
		<mm-text-field
			label={m.teams_webhook_url_label()}
			name="webhook_url"
			type="url"
			mono
			required={!webhook}
			value={form.webhook_url}
			placeholder={webhook?.webhook_url ?? 'https://hooks.slack.com/services/…'}
			help={webhook ? m.teams_webhook_url_keep_help() : ''}
			error={urlError}
			oninput={(e: Event) =>
				(form.webhook_url = (e.currentTarget as unknown as { value: string }).value)}
		></mm-text-field>
		<mm-fieldset label={m.teams_webhook_events_label()} required error={typesError}>
			{#each notificationTypeOptions() as option (option.type)}
				<mm-checkbox
					label={option.label}
					checked={form.notification_types.includes(option.type)}
					onmm-change={(e: CustomEvent<MmCheckedDetail>) =>
						toggleType(option.type, e.detail.checked)}
				></mm-checkbox>
			{/each}
		</mm-fieldset>
		<mm-switch
			label={m.teams_webhook_send_notifications()}
			checked={form.enabled}
			onmm-change={(e: CustomEvent<MmSwitchDetail>) => (form.enabled = e.detail.checked)}
		></mm-switch>
	</form>
	<mm-button slot="footer" data-close="cancel">{m.common_cancel()}</mm-button>
	<mm-button
		slot="footer"
		variant="primary"
		type="submit"
		form="team-webhook"
		icon={webhook ? '' : 'add'}
		loading={saving}
	>
		{webhook ? m.serviceaccounts_save_changes() : m.teams_add_webhook()}
	</mm-button>
</mm-drawer>
