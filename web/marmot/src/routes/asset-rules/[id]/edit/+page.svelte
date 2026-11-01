<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { getAssetRule } from '$lib/assetrules/api';
	import type { AssetRule } from '$lib/assetrules/types';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import RuleEditor from '$components/assetrule/RuleEditor.svelte';

	let ruleId = $derived(page.params.id ?? '');
	let rule = $state<AssetRule | null>(null);
	let loadError = $state<string | null>(null);

	let detailHref = $derived(resolve('/asset-rules/[id]', { id: ruleId }));

	$effect(() => {
		const id = ruleId;
		rule = null;
		loadError = null;
		getAssetRule(id)
			.then((r) => (rule = r))
			.catch((e) => (loadError = e instanceof Error ? e.message : m.assetrules_load_one_error()));
	});

	$effect(() => {
		setCrumbs([
			{ label: m.assetrules_heading(), href: resolve('/asset-rules') },
			{ label: rule?.name ?? m.assetrules_fallback_name(), href: detailHref },
			{ label: m.common_edit() }
		]);
	});
</script>

<svelte:head>
	<title>{m.assetrules_edit_page_title({ name: rule?.name ?? m.assetrules_fallback_name() })}</title
	>
</svelte:head>

<div class="mm-stack">
	<mm-page-header
		heading={m.assetrules_edit_rule_heading()}
		back-href={detailHref}
		back-label={rule?.name ?? m.assetrules_fallback_name()}
	></mm-page-header>
	{#if loadError}
		<mm-callout tone="danger" heading={m.assetrules_load_one_error()}>
			{loadError}
			<mm-button slot="actions" size="sm" href={resolve('/asset-rules')}
				>{m.assetrules_back_to_rules()}</mm-button
			>
		</mm-callout>
	{:else if !rule}
		<mm-skeleton loading count={6}></mm-skeleton>
	{:else}
		{#key rule.id}
			<RuleEditor {rule} cancelHref={detailHref} />
		{/key}
	{/if}
</div>
