<script lang="ts">
	import IconifyIcon from '@iconify/svelte';

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
	<div class="flex gap-6" role="tablist" aria-label="Tabs">
		{#each tabs as tab}
			<button
				onclick={() => handleTabClick(tab.id)}
				role="tab"
				aria-selected={activeTab === tab.id}
				aria-controls="tabpanel-{tab.id}"
				id="tab-{tab.id}"
				class="flex items-center gap-2 py-4 px-1 border-b-2 text-sm font-medium transition-colors {activeTab ===
				tab.id
					? 'border-earthy-terracotta-600 text-earthy-terracotta-600 dark:text-earthy-terracotta-400'
					: 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300 hover:border-gray-300'}"
			>
				{#if tab.icon}
					<IconifyIcon icon={tab.icon} class="h-4 w-4" aria-hidden="true" />
				{/if}
				{tab.label}
			</button>
		{/each}
	</div>
</div>
