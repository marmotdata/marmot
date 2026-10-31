<script lang="ts">
	import { onMount } from 'svelte';
	import { m } from '$lib/paraglide/messages';
	import {
		notificationPreferences,
		type NotificationType
	} from '$lib/stores/notificationPreferences';

	let ready = $state(false);

	const types: { type: NotificationType; label: string; description: string; icon: string }[] = [
		{
			type: 'system',
			label: m.notifpref_system_label(),
			description: m.notifpref_system_description(),
			icon: 'info'
		},
		{
			type: 'schema_change',
			label: m.notifpref_schema_change_label(),
			description: m.notifpref_schema_change_description(),
			icon: 'schema'
		},
		{
			type: 'upstream_schema_change',
			label: m.notifpref_upstream_schema_change_label(),
			description: m.notifpref_upstream_schema_change_description(),
			icon: 'arrow-upward-alt'
		},
		{
			type: 'downstream_schema_change',
			label: m.notifpref_downstream_schema_change_label(),
			description: m.notifpref_downstream_schema_change_description(),
			icon: 'arrow-downward-alt'
		},
		{
			type: 'asset_change',
			label: m.notifpref_asset_change_label(),
			description: m.notifpref_asset_change_description(),
			icon: 'database'
		},
		{
			type: 'lineage_change',
			label: m.notifpref_lineage_change_label(),
			description: m.notifpref_lineage_change_description(),
			icon: 'timeline'
		},
		{
			type: 'mention',
			label: m.notifpref_mention_label(),
			description: m.notifpref_mention_description(),
			icon: 'alternate-email'
		},
		{
			type: 'asset_deleted',
			label: m.notifpref_asset_deleted_label(),
			description: m.notifpref_asset_deleted_description(),
			icon: 'delete'
		},
		{
			type: 'job_complete',
			label: m.notifpref_job_complete_label(),
			description: m.notifpref_job_complete_description(),
			icon: 'check-circle'
		},
		{
			type: 'access_expiry',
			label: m.notifpref_access_expiry_label(),
			description: m.notifpref_access_expiry_description(),
			icon: 'timer'
		}
	];

	onMount(async () => {
		await notificationPreferences.initialize();
		ready = true;
	});
</script>

<!-- Each switch saves as soon as it changes, so this tab has no save button -->
<mm-section heading={m.account_notify_me_about()}>
	<mm-list class="prefs">
		{#each types as t (t.type)}
			<mm-list-item secondary={t.description}>
				<mm-icon slot="start" name={t.icon} aria-hidden="true"></mm-icon>
				{t.label}
				<mm-switch
					slot="end"
					label={t.label}
					hide-label
					checked={$notificationPreferences[t.type]}
					disabled={!ready}
					onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
						notificationPreferences.setPreference(t.type, e.detail.checked)}
				></mm-switch>
			</mm-list-item>
		{/each}
	</mm-list>
</mm-section>

<style>
	.prefs {
		--mm-list-inset: 0;
		max-width: 40rem;
	}
</style>
