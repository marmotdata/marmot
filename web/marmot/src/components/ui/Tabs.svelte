<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';

	export type Tab = {
		id: string;
		label: string;
		icon?: string;
	};

	let {
		tabs,
		activeTab = $bindable(''),
		onTabChange
	}: {
		tabs: Tab[];
		activeTab?: string;
		onTabChange?: (tabId: string) => void;
	} = $props();

	function handleTabClick(tabId: string) {
		activeTab = tabId;
		onTabChange?.(tabId);
	}
</script>

<div class="border-b border-gray-200 dark:border-gray-700">
	<div class="flex gap-1" role="tablist" aria-label={m.ui_tabs_aria()}>
		{#each tabs as tab (tab.id)}
			<button
				type="button"
				onclick={() => handleTabClick(tab.id)}
				role="tab"
				aria-selected={activeTab === tab.id}
				aria-controls="tabpanel-{tab.id}"
				id="tab-{tab.id}"
				class="relative inline-flex h-10 items-center gap-1.5 px-2.5 text-sm font-medium transition {activeTab ===
				tab.id
					? 'text-gray-900 dark:text-gray-100'
					: 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
			>
				{#if tab.icon}
					<IconifyIcon icon={tab.icon} class="h-4 w-4" aria-hidden="true" />
				{/if}
				{tab.label}
				{#if activeTab === tab.id}
					<span class="absolute inset-x-2 bottom-0 h-0.5 rounded-full bg-earthy-terracotta-500"
					></span>
				{/if}
			</button>
		{/each}
	</div>
</div>
