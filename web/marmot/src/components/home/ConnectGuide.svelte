<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import Snippet from './Snippet.svelte';
	import AgentClientSnippet from './AgentClientSnippet.svelte';
	import HeadlessAgentHint from './HeadlessAgentHint.svelte';
	import type { ConnectTab } from './ConnectBlade.svelte';

	// The how-to for each way into Marmot, shared by the Connect blade and the homepage setup panel so the instructions are written once
	let { tab }: { tab: ConnectTab } = $props();

	const canManageIngestion = auth.hasPermission('ingestion', 'manage');

	const origin =
		typeof window === 'undefined' ? 'https://marmot.example.com' : window.location.origin;

	const install = `curl -fsSL get.marmotdata.io | sh
marmot login ${origin}`;
	const cliTour = `marmot search orders              # find anything in the catalog
marmot query sources               # sources you can query
marmot query run <source> --table <name>
marmot skills install              # teach your coding agent the CLI`;
	const OPTION = 'flex flex-col rounded-xl border border-gray-200 p-4 dark:border-gray-700';
	const codeGuides = [
		{ label: m.home_connect_guide_cli(), href: 'https://marmotdata.io/docs/Populating/CLI' },
		{ label: 'Terraform', href: 'https://marmotdata.io/docs/Populating/Terraform' },
		{ label: 'Pulumi', href: 'https://marmotdata.io/docs/Populating/Pulumi' }
	];
	const curl = `curl -H "X-API-Key: $MARMOT_API_KEY" \\
  "${origin}/api/v1/search?q=orders"`;
</script>

{#snippet step(n: number, title: string)}
	<h3 class="flex items-center gap-2.5 text-sm font-semibold text-gray-900 dark:text-gray-100">
		<span
			class="flex h-6 w-6 items-center justify-center rounded-full bg-gray-900 text-xs text-white dark:bg-gray-100 dark:text-gray-900"
			>{n}</span
		>
		{title}
	</h3>
{/snippet}

{#if tab === 'agent'}
	<p class="text-sm text-gray-600 dark:text-gray-300">{m.home_connect_agent_intro()}</p>
	<div class="mt-6 space-y-6">
		<section class="space-y-3">
			{@render step(1, m.home_connect_agent_step_client())}
			<div class="pl-8"><AgentClientSnippet /></div>
		</section>

		<section class="space-y-2">
			{@render step(2, m.home_connect_agent_step_try())}
			<div class="pl-8">
				<Snippet code={m.home_connect_agent_prompt()} label={m.home_connect_prompt_label()} />
			</div>
		</section>

		<HeadlessAgentHint />
	</div>
{:else if tab === 'source'}
	<!-- Two alternative routes side by side rather than numbered steps, since people pick one or the other -->
	<div class="grid gap-3 md:grid-cols-2">
		<section class={OPTION}>
			<div class="flex items-center gap-2">
				<IconifyIcon icon="material-symbols:web" class="h-5 w-5 text-earthy-terracotta-600" />
				<h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
					{m.home_connect_source_ui()}
				</h3>
				<span
					class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-300"
					>{m.home_connect_recommended()}</span
				>
			</div>
			<p class="mt-1 flex-1 text-sm text-gray-600 dark:text-gray-400">
				{m.home_connect_source_ui_body()}
			</p>
			<div class="mt-4">
				{#if canManageIngestion}
					<a
						href={resolve('/pipelines/new')}
						class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white hover:bg-earthy-terracotta-500"
					>
						<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
						{m.pipelines_create_pipeline()}
					</a>
				{:else}
					<p class="text-sm text-amber-700 dark:text-amber-300">{m.home_needs_admin()}</p>
				{/if}
			</div>
		</section>
		<section class={OPTION}>
			<div class="flex items-center gap-2">
				<IconifyIcon icon="material-symbols:code" class="h-5 w-5 text-gray-500" />
				<h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
					{m.home_connect_source_code()}
				</h3>
			</div>
			<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
				{m.home_connect_source_code_body()}
			</p>
			<div class="mt-4 space-y-2">
				<Snippet code="marmot ingest -c pipeline.yaml" />
				<div class="flex flex-wrap gap-x-4 gap-y-1">
					{#each codeGuides as guide (guide.href)}
						<a
							href={guide.href}
							target="_blank"
							rel="external noopener noreferrer"
							class="inline-flex items-center gap-1 text-sm font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
						>
							{guide.label}
							<IconifyIcon icon="material-symbols:arrow-outward" class="h-3.5 w-3.5" />
						</a>
					{/each}
				</div>
			</div>
		</section>
	</div>
{:else if tab === 'cli'}
	<p class="text-sm text-gray-600 dark:text-gray-300">{m.home_connect_cli_intro()}</p>
	<div class="mt-6 space-y-6">
		<section class="space-y-2">
			{@render step(1, m.home_connect_cli_install())}
			<div class="pl-8"><Snippet code={install} label={m.home_connect_terminal()} /></div>
		</section>
		<section class="space-y-2">
			{@render step(2, m.home_connect_cli_try())}
			<div class="pl-8"><Snippet code={cliTour} label={m.home_connect_terminal()} /></div>
		</section>
	</div>
{:else}
	<p class="text-sm text-gray-600 dark:text-gray-300">{m.home_connect_api_intro()}</p>
	<div class="mt-6 space-y-3">
		<Snippet code={curl} label={m.home_connect_terminal()} />
		<a
			href="https://marmotdata.io/docs/Populating/API"
			target="_blank"
			rel="external noopener noreferrer"
			class="inline-flex items-center gap-1 text-sm font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
		>
			{m.home_connect_api_docs()}
			<IconifyIcon icon="material-symbols:arrow-outward" class="h-3.5 w-3.5" />
		</a>
	</div>
{/if}
