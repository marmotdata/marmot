<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { setCrumbs } from '$lib/mm';

	// Error pages keep the app shell so every destination stays one click away, and show no banner as well because it would say the same thing as the page

	let offline = $state(false);

	onMount(() => {
		offline = !navigator.onLine;
		const update = () => (offline = !navigator.onLine);
		window.addEventListener('online', update);
		window.addEventListener('offline', update);
		return () => {
			window.removeEventListener('online', update);
			window.removeEventListener('offline', update);
		};
	});

	const status = $derived(page.status);
	const reference = $derived(page.error?.requestId ?? '');
	const kind = $derived.by(() => {
		if (offline) return 'offline';
		if (status === 404) return 'not-found';
		if (status === 401 || status === 403) return 'denied';
		if (status === 502 || status === 503 || status === 504) return 'unreachable';
		return 'server';
	});

	// The last part of the address is usually the thing people were looking for, so the search starts from it
	const lastSegment = $derived(
		decodeURIComponent(page.url.pathname.split('/').filter(Boolean).pop() ?? '')
	);
	const searchHref = $derived(
		lastSegment
			? `${resolve('/discover')}?q=${encodeURIComponent(lastSegment)}`
			: resolve('/discover')
	);

	$effect(() => {
		const label = {
			offline: m.errors_offline_crumb,
			'not-found': m.errors_not_found_crumb,
			denied: m.errors_denied_crumb,
			unreachable: m.errors_server_crumb,
			server: m.errors_server_crumb
		}[kind]();
		setCrumbs([{ label }]);
	});

	function reload() {
		window.location.reload();
	}
</script>

<div class="centred">
	{#if kind === 'offline'}
		<mm-empty-state
			heading={m.errors_offline_heading()}
			description={m.errors_offline_body()}
			level={1}
			seed={14}
		>
			<mm-button slot="action" icon="refresh" onclick={reload}>{m.errors_try_again()}</mm-button>
		</mm-empty-state>
	{:else if kind === 'not-found'}
		<mm-empty-state heading={m.errors_not_found_heading()} level={1} seed={2}>
			<span><code>{page.url.pathname}</code><br />{m.errors_not_found_body()}</span>
			<mm-button slot="action" variant="primary" icon="search" href={searchHref}
				>{m.errors_search_catalog()}</mm-button
			>
		</mm-empty-state>
	{:else if kind === 'denied'}
		<mm-empty-state
			heading={m.errors_denied_heading()}
			description={m.errors_denied_body()}
			level={1}
			seed={10}
		>
			<mm-button slot="action" icon="arrow-back" onclick={() => history.back()}
				>{m.errors_go_back()}</mm-button
			>
		</mm-empty-state>
	{:else if kind === 'unreachable'}
		<mm-empty-state
			heading={m.errors_unreachable_heading()}
			description={m.errors_unreachable_body()}
			level={1}
			seed={12}
		>
			<mm-button slot="action" icon="refresh" onclick={reload}>{m.errors_try_again_now()}</mm-button
			>
		</mm-empty-state>
	{:else}
		<mm-empty-state
			heading={m.errors_server_heading()}
			description={reference ? '' : m.errors_server_body()}
			level={1}
			seed={15}
		>
			{#if reference}
				<span>{m.errors_server_body_reference()} <code>{reference}</code>.</span>
			{/if}
			<mm-button slot="action" variant="primary" icon="refresh" onclick={reload}
				>{m.errors_reload_page()}</mm-button
			>
		</mm-empty-state>
	{/if}
</div>

<style>
	.centred {
		display: grid;
		min-height: 100%;
	}
</style>
