<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import type { PluginVersionStatus } from '$lib/plugins/types';
	import { pluginVersionsColumns, type PluginRowAction } from './pluginVersionsColumns';

	// One row per core plugin, in the order the page has already sorted and filtered them. Row
	// actions call straight back to the page, which owns the API calls and the toasts.
	let {
		plugins,
		canManage,
		disabled,
		busyId,
		onAction
	}: {
		plugins: PluginVersionStatus[];
		canManage: boolean;
		/** Disables every row's actions while any one plugin has a change in flight. */
		disabled: boolean;
		busyId: string | null;
		onAction: (plugin: PluginVersionStatus, action: PluginRowAction) => void;
	} = $props();

	let columns = $derived(pluginVersionsColumns(canManage, disabled, busyId, onAction));
</script>

<mm-data-table
	label={m.plugin_versions_table_label()}
	row-key="id"
	{columns}
	rows={plugins}
	empty-heading={m.plugin_versions_empty_heading()}
	empty-text={m.plugin_versions_empty_body()}
></mm-data-table>
