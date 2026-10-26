<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import Snippet from './Snippet.svelte';

	// How to add Marmot's MCP server to the common agent clients, used wherever an agent is being connected; clients sign in through OAuth in the browser, so no key is pasted anywhere
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
	const HINT = 'mt-2 text-xs text-gray-500 dark:text-gray-400';
	const cursor = JSON.stringify({ mcpServers: { marmot: { url: mcpUrl } } }, null, 2);
</script>

<div>
	<div
		class="mb-3 inline-flex rounded-lg border border-gray-200 bg-white p-0.5 dark:border-gray-700 dark:bg-gray-800"
		role="group"
	>
		{#each clients as c (c.id)}
			<button
				type="button"
				onclick={() => (client = c.id)}
				aria-pressed={client === c.id}
				class="rounded-md px-3 py-1 text-xs font-medium transition {client === c.id
					? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
					: 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
				>{c.label}</button
			>
		{/each}
	</div>
	{#if client === 'claude-code'}
		<Snippet code={claudeCode} label={m.home_connect_terminal()} />
		<p class={HINT}>{m.home_connect_claude_code_signin()}</p>
	{:else if client === 'cursor'}
		<Snippet code={cursor} label={m.home_connect_cursor_file()} />
		<p class={HINT}>{m.home_connect_cursor_signin()}</p>
	{:else}
		<ol class="list-decimal space-y-1.5 pl-5 text-sm text-gray-700 dark:text-gray-300">
			<li>{m.home_connect_claude_ai_step1()}</li>
			<li>{m.home_connect_claude_ai_step2()}</li>
		</ol>
		<div class="mt-3"><Snippet code={mcpUrl} label={m.home_connect_mcp_url()} /></div>
		<p class={HINT}>{m.home_connect_claude_ai_admin()}</p>
	{/if}
</div>
