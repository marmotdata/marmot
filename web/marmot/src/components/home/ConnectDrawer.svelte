<script lang="ts" module>
	export type ConnectTab = 'agent' | 'source' | 'cli' | 'api';
</script>

<script lang="ts">
	import '@marmotdata/elements/drawer.js';
	import '@marmotdata/elements/code-sample.js';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import AgentClientSnippet from './AgentClientSnippet.svelte';

	let { tab = 'agent', onClose }: { tab?: ConnectTab; onClose: () => void } = $props();

	interface Plugin {
		id: string;
		name: string;
		supports_query?: boolean;
	}

	const DOCS = 'https://marmotdata.io/docs';
	const canManageIngestion = auth.hasPermission('ingestion', 'manage');
	const isAdmin = auth.hasRole('admin');

	const origin =
		typeof window === 'undefined' ? 'https://marmot.example.com' : window.location.origin;
	const mcpUrl = `${origin}/api/v1/mcp`;
	const install = `curl -fsSL get.marmotdata.io | sh\nmarmot login ${origin}`;
	const cliTour = `marmot search orders\nmarmot query sources\nmarmot query run <source> --table <name>\nmarmot skills install`;
	const curl = `curl -H "X-API-Key: $MARMOT_API_KEY" \\\n  "${origin}/api/v1/search?q=orders"`;

	// The drawer is created each time it opens, so the tab it opens on is read once
	let active = $state<ConnectTab>((() => tab)());
	let plugins = $state<Plugin[]>([]);

	// Plugins that answer queries lead, since querying is what people connect sources for
	let featured = $derived(
		[...plugins].sort((a, b) => Number(!!b.supports_query) - Number(!!a.supports_query)).slice(0, 5)
	);

	onMount(async () => {
		if (!canManageIngestion) return;
		try {
			const res = await fetchApi('/plugins');
			if (res.ok) {
				const data = await res.json();
				plugins = Array.isArray(data?.plugins) ? data.plugins : [];
			}
		} catch {
			// The list is a shortcut; Browse all plugins still works without it
		}
	});
</script>

<mm-drawer open dismissible heading={m.home_connect_title()} onmm-close={onClose}>
	<mm-tabs
		label={m.home_connect_title()}
		value={active}
		onmm-tab-change={(e: CustomEvent<{ value: string }>) => (active = e.detail.value as ConnectTab)}
	>
		<mm-tab value="agent" icon="smart-toy">{m.home_connect_tab_agent()}</mm-tab>
		<mm-tab value="source" icon="database">{m.home_connect_tab_source()}</mm-tab>
		<mm-tab value="cli" icon="terminal">{m.home_connect_tab_cli()}</mm-tab>
		<mm-tab value="api" icon="api">{m.home_connect_tab_api()}</mm-tab>

		<mm-tab-panel value="agent">
			<div class="mm-stack py-space-4" style="--mm-gap: var(--space-4)">
				<mm-text variant="dense" block>{m.home_connect_agent_intro()}</mm-text>
				<AgentClientSnippet />
				<mm-text variant="dense" tone="secondary" block>
					{m.home_connect_agent_headless()}
					{#if isAdmin}
						<mm-link href={resolve('/settings/service-accounts')}
							>{m.home_connect_agent_service_account()}</mm-link
						>
					{/if}
				</mm-text>
				<mm-description-list>
					<mm-description term={m.home_connect_mcp_endpoint()} mono>{mcpUrl}</mm-description>
					<mm-description term={m.home_connect_transport()}
						>{m.home_connect_transport_http()}</mm-description
					>
				</mm-description-list>
			</div>
		</mm-tab-panel>

		<mm-tab-panel value="source">
			<div class="mm-stack py-space-4" style="--mm-gap: var(--space-4)">
				{#if canManageIngestion}
					{#if featured.length > 0}
						<mm-list aria-label={m.home_connect_tab_source()}>
							{#each featured as p (p.id)}
								<mm-list-item
									href="{resolve('/pipelines/new')}?plugin={encodeURIComponent(p.id)}"
									secondary={p.supports_query
										? m.home_connect_supports_query()
										: m.home_connect_metadata_only()}
								>
									<mm-provider-logo slot="start" provider={p.id} size="20" decorative
									></mm-provider-logo>
									{p.name}
								</mm-list-item>
							{/each}
						</mm-list>
					{/if}
					<div class="mm-cluster" style="--mm-gap: var(--space-4)">
						<mm-link href={resolve('/pipelines/new')}>{m.home_connect_browse_plugins()}</mm-link>
						<mm-link href="{DOCS}/Populating/CLI" external>{m.home_connect_source_code()}</mm-link>
					</div>
				{:else}
					<mm-text variant="dense" tone="secondary" block>{m.home_needs_admin()}</mm-text>
				{/if}
			</div>
		</mm-tab-panel>

		<mm-tab-panel value="cli">
			<div class="mm-stack py-space-4" style="--mm-gap: var(--space-4)">
				<mm-text variant="dense" block>{m.home_connect_cli_intro()}</mm-text>
				<mm-code-sample language="shell" wrap label={m.home_connect_cli_install()} code={install}
				></mm-code-sample>
				<mm-code-sample language="shell" wrap label={m.home_connect_cli_try()} code={cliTour}
				></mm-code-sample>
			</div>
		</mm-tab-panel>

		<mm-tab-panel value="api">
			<div class="mm-stack py-space-4" style="--mm-gap: var(--space-4)">
				<mm-text variant="dense" block>{m.home_connect_api_intro()}</mm-text>
				<mm-code-sample language="shell" wrap label={m.home_connect_api_sample()} code={curl}
				></mm-code-sample>
				<mm-link href="{DOCS}/Populating/API" external>{m.home_connect_api_docs()}</mm-link>
			</div>
		</mm-tab-panel>
	</mm-tabs>
	<mm-button slot="footer" href="{DOCS}/MCP" external icon="menu-book"
		>{m.home_connect_mcp_guide()}</mm-button
	>
</mm-drawer>
