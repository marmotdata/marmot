<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';

	export let tabs: { id: string; label: string }[];

	$: activeTab = $page.url.searchParams.get('tab') || tabs[0]?.id;

	function handleTabChange(tabId: string) {
		const newUrl = new URL($page.url);
		newUrl.searchParams.set('tab', tabId);
		goto(newUrl.toString());
	}
</script>

<div class="w-full lg:w-64 flex-shrink-0">
	<div
		class="bg-earthy-brown-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
	>
		<nav class="space-y-1 p-2" aria-label="Admin navigation">
			{#each tabs as tab}
				<button
					class="w-full flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors {activeTab ===
					tab.id
						? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-100'
						: 'text-gray-600 dark:text-gray-400 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/10 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-100'}"
					on:click={() => handleTabChange(tab.id)}
				>
					{tab.label}
				</button>
			{/each}
		</nav>
	</div>
</div>
