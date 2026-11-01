<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import { formatRelativeTime } from '$lib/utils';
	import type { PluginUpdatesStatus } from '$lib/plugins/types';

	// The server's own update policy and the state of the plugin index: read-only here, since both
	// are set in Marmot's configuration, not from this page.
	let {
		status,
		canManage,
		checking,
		onCheck
	}: {
		status: PluginUpdatesStatus;
		canManage: boolean;
		checking: boolean;
		onCheck: () => void;
	} = $props();

	const policyLabel = $derived(
		{
			off: m.plugin_versions_policy_off(),
			security: m.plugin_versions_policy_security(),
			patch: m.plugin_versions_policy_patch()
		}[status.auto_update]
	);

	const policyDetail = $derived(
		{
			off: m.plugin_versions_policy_off_detail(),
			security: m.plugin_versions_policy_security_detail(),
			patch: m.plugin_versions_policy_patch_detail()
		}[status.auto_update]
	);
</script>

<mm-section
	heading={m.plugin_versions_index_heading()}
	description={m.plugin_versions_index_description()}
>
	{#if canManage && status.enabled}
		<mm-button
			slot="actions"
			icon="refresh"
			loading={checking}
			disabled={checking}
			onclick={onCheck}
		>
			{m.plugin_versions_check_now()}
		</mm-button>
	{/if}

	{#if !status.enabled}
		<mm-callout tone="info" compact heading={m.plugin_versions_disabled_notice()}></mm-callout>
	{:else}
		<div class="mm-stack" style="--mm-gap: var(--space-3)">
			<mm-description-list layout="inline-grid" columns={4}>
				<mm-description term={m.plugin_versions_policy_label()}>{policyLabel}</mm-description>
				<mm-description term={m.plugin_versions_marmot_version()}
					>{status.marmot_version}</mm-description
				>
				<mm-description term={m.plugin_versions_index_checked_label()}>
					{status.index.checked_at
						? formatRelativeTime(status.index.checked_at)
						: m.plugin_versions_index_never_checked()}
				</mm-description>
				<mm-description term={m.plugin_versions_index_generated_label()}>
					{status.index.generated ? formatRelativeTime(status.index.generated) : m.common_never()}
				</mm-description>
			</mm-description-list>
			<mm-text variant="caption" tone="secondary">{policyDetail}</mm-text>
		</div>

		{#if status.index.expired}
			<mm-callout tone="warning" compact heading={m.plugin_versions_index_expired()}></mm-callout>
		{/if}
		{#if status.index.error}
			<mm-callout
				tone="danger"
				compact
				heading={m.plugin_versions_index_error({ error: status.index.error })}
			></mm-callout>
		{/if}
	{/if}
</mm-section>
