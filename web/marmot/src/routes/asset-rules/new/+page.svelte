<script lang="ts">
	import { resolve } from '$app/paths';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import RuleEditor from '$components/assetrule/RuleEditor.svelte';

	setCrumbs([
		{ label: m.assetrules_heading(), href: resolve('/asset-rules') },
		{ label: m.assetrules_new_rule_heading() }
	]);
</script>

<svelte:head>
	<title>{m.assetrules_new_page_title()}</title>
</svelte:head>

<div class="mm-stack">
	<mm-page-header
		heading={m.assetrules_new_rule_heading()}
		back-href={resolve('/asset-rules')}
		back-label={m.assetrules_heading()}
	></mm-page-header>
	<RuleEditor cancelHref={resolve('/asset-rules')} />
</div>
