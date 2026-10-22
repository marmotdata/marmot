<script lang="ts" module>
	export type ConnectTab = 'agent' | 'source' | 'cli' | 'api';
</script>

<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Blade from '$components/ui/Blade.svelte';
	import { m } from '$lib/paraglide/messages';
	import ConnectGuide from './ConnectGuide.svelte';

	let { tab = 'agent', onClose }: { tab?: ConnectTab; onClose: () => void } = $props();

	// The blade is created each time it opens, so the tab it opens on is read once
	let active = $state<ConnectTab>((() => tab)());

	const tabs: { id: ConnectTab; label: string; icon: string }[] = [
		{ id: 'agent', label: m.home_connect_tab_agent(), icon: 'material-symbols:smart-toy-outline' },
		{ id: 'source', label: m.home_connect_tab_source(), icon: 'material-symbols:database-outline' },
		{ id: 'cli', label: m.home_connect_tab_cli(), icon: 'material-symbols:terminal' },
		{ id: 'api', label: m.home_connect_tab_api(), icon: 'material-symbols:api' }
	];
</script>

<Blade title={m.home_connect_title()} subtitle={m.home_connect_subtitle()} {onClose} width="2xl">
	<div class="-mt-1 mb-6 flex gap-1 border-b border-gray-200 dark:border-gray-700" role="tablist">
		{#each tabs as t (t.id)}
			<button
				type="button"
				role="tab"
				aria-selected={active === t.id}
				onclick={() => (active = t.id)}
				class="-mb-px inline-flex items-center gap-2 border-b-2 px-3 py-2.5 text-sm font-medium transition {active ===
				t.id
					? 'border-earthy-terracotta-500 text-gray-900 dark:text-gray-100'
					: 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}"
			>
				<IconifyIcon icon={t.icon} class="h-4 w-4" />
				{t.label}
			</button>
		{/each}
	</div>

	<ConnectGuide tab={active} />
</Blade>
