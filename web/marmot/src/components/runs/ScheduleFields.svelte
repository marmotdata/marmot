<script lang="ts">
	import '@marmotdata/elements/fieldset.js';
	import '@marmotdata/elements/cron-field.js';
	import '@marmotdata/elements/key-value-editor.js';
	import type { KeyValuePair } from '@marmotdata/elements/classes/key-value-editor.js';
	import { m } from '$lib/paraglide/messages';
	import ListInput from './ListInput.svelte';

	interface BaseConfig {
		tags?: string[];
		external_links?: { name: string; url: string }[];
		filter?: { include?: string[]; exclude?: string[] };
		[key: string]: unknown;
	}

	interface Props {
		cronExpression: string;
		scheduled: boolean;
		config: BaseConfig;
		// Editing a pipeline can pause its schedule without losing it
		disableSchedule?: boolean;
		showPause?: boolean;
		cronError?: string;
	}

	let {
		cronExpression = $bindable(),
		scheduled = $bindable(),
		config = $bindable(),
		disableSchedule = $bindable(false),
		showPause = false,
		cronError
	}: Props = $props();

	// A schedule adds work, so it starts off and its fields appear only once it is switched on
	const DEFAULT_CRON = '0 2 * * *';

	// Example filter patterns are regexes, so they are technical tokens rather than copy
	const includeExample = '^public_.*';
	const excludeExample = '^_tmp_.*';

	let links = $derived<KeyValuePair[]>(
		(config.external_links ?? []).map((l) => ({ key: l.name, value: l.url }))
	);

	function setFilter(key: 'include' | 'exclude', values: string[]) {
		config.filter = { ...config.filter, [key]: values };
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-6)">
	<mm-fieldset>
		<mm-heading slot="label" level={3} variant="heading-sm"
			>{m.pipelines_schedule_label()}</mm-heading
		>
		<div class="mm-stack" style="--mm-gap: var(--space-4)">
			<mm-switch
				label={m.pipelines_schedule_switch_label()}
				description={m.pipelines_schedule_switch_hint()}
				checked={scheduled}
				onmm-change={(e: CustomEvent<{ checked: boolean }>) => {
					scheduled = e.detail.checked;
					if (scheduled && !cronExpression.trim()) cronExpression = DEFAULT_CRON;
				}}
			></mm-switch>
			{#if scheduled}
				<mm-cron-field
					label={m.pipelines_cron_runs_label()}
					name="cron"
					value={cronExpression}
					error={cronError}
					onmm-input={(e: CustomEvent<{ value: string }>) => (cronExpression = e.detail.value)}
					onmm-change={(e: CustomEvent<{ value: string }>) => (cronExpression = e.detail.value)}
				></mm-cron-field>
				{#if showPause}
					<mm-switch
						label={m.pipelines_disable_schedule_label()}
						description={m.pipelines_disable_schedule_hint()}
						checked={disableSchedule}
						onmm-change={(e: CustomEvent<{ checked: boolean }>) =>
							(disableSchedule = e.detail.checked)}
					></mm-switch>
				{/if}
			{/if}
		</div>
	</mm-fieldset>

	<mm-fieldset>
		<mm-heading slot="label" level={3} variant="heading-sm"
			>{m.pipelines_filters_label()}</mm-heading
		>
		<div class="mm-stack" style="--mm-gap: var(--space-4)">
			<ListInput
				label={m.pipelines_filter_include_label()}
				help={m.pipelines_filter_include_hint()}
				placeholder={includeExample}
				mono
				values={config.filter?.include ?? []}
				onchange={(values) => setFilter('include', values)}
			/>
			<ListInput
				label={m.pipelines_filter_exclude_label()}
				help={m.pipelines_filter_exclude_hint()}
				placeholder={excludeExample}
				mono
				values={config.filter?.exclude ?? []}
				onchange={(values) => setFilter('exclude', values)}
			/>
			<ListInput
				label={m.common_tags()}
				help={m.pipelines_tags_hint()}
				placeholder={m.pipelines_tags_typeahead_placeholder()}
				values={config.tags ?? []}
				onchange={(values) => (config.tags = values)}
			/>
			<div class="mm-stack" style="--mm-gap: var(--space-2)">
				<mm-text variant="label" tone="secondary">{m.pipelines_external_links_label()}</mm-text>
				<mm-text variant="caption" tone="secondary">{m.pipelines_external_links_hint()}</mm-text>
				<mm-key-value-editor
					label={m.pipelines_external_links_label()}
					key-label={m.common_name()}
					value-label={m.pipelines_url_label()}
					key-placeholder={m.pipelines_link_name_placeholder()}
					value-placeholder="https://"
					add-label={m.pipelines_add_external_link()}
					value={links}
					onmm-change={(e: CustomEvent<{ value: { key: string; value: string }[] }>) =>
						(config.external_links = e.detail.value.map((p) => ({ name: p.key, url: p.value })))}
				></mm-key-value-editor>
			</div>
		</div>
	</mm-fieldset>
</div>
