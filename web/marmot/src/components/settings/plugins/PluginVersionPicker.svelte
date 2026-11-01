<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import type { PluginVersionOption, PluginVersionStatus } from '$lib/plugins/types';

	// Every published version of one plugin, newest first (the server's own order), so an admin can
	// move to something other than the latest: an older compatible release, or one it was pinned to
	// before. Versions that cannot be chosen stay visible, greyed out, with the reason underneath.
	let {
		plugin,
		busyVersion,
		onChoose,
		onClose
	}: {
		plugin: PluginVersionStatus | null;
		busyVersion: string | null;
		onChoose: (version: string) => void;
		onClose: () => void;
	} = $props();

	function hasSecondary(v: PluginVersionOption): boolean {
		return !v.compatible || Boolean(v.yanked && v.yanked_reason) || Boolean(v.changelog_url);
	}

	function unselectable(plugin: PluginVersionStatus, v: PluginVersionOption): boolean {
		return !v.compatible || v.yanked === true || v.version === plugin.version;
	}
</script>

<mm-dialog
	open={plugin !== null}
	heading={plugin ? m.plugin_versions_picker_heading({ name: plugin.name }) : ''}
	description={m.plugin_versions_picker_description()}
	size="md"
	onmm-close={onClose}
>
	{#if plugin}
		{#if plugin.versions.length === 0}
			<mm-empty-state
				compact
				heading={m.plugin_versions_picker_empty_heading()}
				description={m.plugin_versions_picker_empty_body()}
			></mm-empty-state>
		{:else}
			<mm-list>
				{#each plugin.versions as v (v.version)}
					<mm-list-item disabled={unselectable(plugin, v)}>
						<span class="mm-cluster" style="--mm-gap: var(--space-2); flex-wrap: nowrap">
							<mm-text variant="mono-dense">{v.version}</mm-text>
							{#if v.version === plugin.version}
								<mm-tag>{m.plugin_versions_picker_current()}</mm-tag>
							{/if}
							{#if v.yanked}
								<mm-status-pill status="danger">{m.plugin_versions_picker_yanked()}</mm-status-pill>
							{/if}
							{#if v.advisories.length > 0}
								<mm-status-pill status="danger"
									>{m.plugin_versions_picker_advisory({
										ids: v.advisories.join(', ')
									})}</mm-status-pill
								>
							{/if}
						</span>
						{#if hasSecondary(v)}
							<span slot="secondary" class="mm-cluster" style="--mm-gap: var(--space-2)">
								{#if !v.compatible}
									<span
										>{m.plugin_versions_picker_needs_marmot({
											version: v.marmot_version ?? ''
										})}</span
									>
								{/if}
								{#if v.yanked && v.yanked_reason}
									<span>{m.plugin_versions_picker_yanked_reason({ reason: v.yanked_reason })}</span>
								{/if}
								{#if v.changelog_url}
									<a
										class="mm-link"
										href={v.changelog_url}
										target="_blank"
										rel="external noopener noreferrer">{m.plugin_versions_picker_changelog()}</a
									>
								{/if}
							</span>
						{/if}
						<div slot="end">
							<mm-button
								size="sm"
								variant="secondary"
								loading={busyVersion === v.version}
								disabled={unselectable(plugin, v) ||
									(busyVersion !== null && busyVersion !== v.version)}
								onclick={() => onChoose(v.version)}
							>
								{m.plugin_versions_picker_use()}
							</mm-button>
						</div>
					</mm-list-item>
				{/each}
			</mm-list>
		{/if}
	{/if}
	<mm-button slot="footer" data-close="cancel">{m.common_close()}</mm-button>
</mm-dialog>
