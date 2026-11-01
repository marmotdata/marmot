<script lang="ts">
	import '@marmotdata/elements/code-sample.js';
	import { m } from '$lib/paraglide/messages';

	// How to add Marmot's MCP server to the common agent clients; clients sign in through OAuth in the browser, so no key is pasted anywhere
	type Client = 'claude-code' | 'cursor' | 'claude-ai';

	let client = $state<Client>('claude-code');

	const origin =
		typeof window === 'undefined' ? 'https://marmot.example.com' : window.location.origin;
	const mcpUrl = `${origin}/api/v1/mcp`;

	const clients: { id: Client; label: string }[] = [
		{ id: 'claude-code', label: 'Claude Code' },
		{ id: 'cursor', label: 'Cursor' },
		{ id: 'claude-ai', label: 'Claude.ai' }
	];

	const claudeCode = `claude mcp add --transport http marmot ${mcpUrl}`;
	const cursor = JSON.stringify({ mcpServers: { marmot: { url: mcpUrl } } }, null, 2);
</script>

<div class="mm-stack" style="--mm-gap: var(--space-3)">
	<mm-segmented-control
		label={m.home_connect_client()}
		hide-label
		value={client}
		onmm-change={(e: CustomEvent<{ value: string }>) => (client = e.detail.value as Client)}
	>
		{#each clients as c (c.id)}
			<mm-segment value={c.id}>{c.label}</mm-segment>
		{/each}
	</mm-segmented-control>
	{#if client === 'claude-code'}
		<mm-code-sample language="shell" wrap label={m.home_connect_terminal()} code={claudeCode}
		></mm-code-sample>
		<mm-text variant="caption" tone="secondary" block>{m.home_connect_claude_code_signin()}</mm-text
		>
	{:else if client === 'cursor'}
		<mm-code-sample language="json" filename=".cursor/mcp.json" code={cursor}></mm-code-sample>
		<mm-text variant="caption" tone="secondary" block>{m.home_connect_cursor_signin()}</mm-text>
	{:else}
		<ol class="mm-stack list-decimal ps-5 text-dense text-mm-text" style="--mm-gap: var(--space-1)">
			<li>{m.home_connect_claude_ai_step1()}</li>
			<li>{m.home_connect_claude_ai_step2()}</li>
		</ol>
		<mm-code-sample language="text" label={m.home_connect_mcp_url()} code={mcpUrl}></mm-code-sample>
		<mm-text variant="caption" tone="secondary" block>{m.home_connect_claude_ai_admin()}</mm-text>
	{/if}
</div>
