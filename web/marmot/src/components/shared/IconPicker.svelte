<script lang="ts">
	import IconifyIcon from '@iconify/svelte';

	let {
		value = $bindable(''),
		onselect
	}: {
		value?: string;
		onselect?: (icon: string) => void;
	} = $props();

	let open = $state(false);
	let searchQuery = $state('');
	let searchResults = $state<{ prefix: string; name: string }[]>([]);
	let searching = $state(false);
	let iconNamesData: Record<string, string[]> | null = null;
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;
	let pickerEl: HTMLDivElement | undefined = $state();

	const commonIcons: { label: string; icons: { name: string; id: string }[] }[] = [
		{
			label: 'Services',
			icons: [
				{ name: 'GitHub', id: 'simple-icons:github' },
				{ name: 'GitLab', id: 'simple-icons:gitlab' },
				{ name: 'Bitbucket', id: 'simple-icons:bitbucket' },
				{ name: 'Jira', id: 'simple-icons:jira' },
				{ name: 'Confluence', id: 'simple-icons:confluence' },
				{ name: 'Slack', id: 'simple-icons:slack' },
				{ name: 'Linear', id: 'simple-icons:linear' },
				{ name: 'Asana', id: 'simple-icons:asana' },
				{ name: 'Trello', id: 'simple-icons:trello' },
				{ name: 'Figma', id: 'simple-icons:figma' },
				{ name: 'Miro', id: 'simple-icons:miro' },
				{ name: 'Notion', id: 'simple-icons:notion' }
			]
		},
		{
			label: 'Data & Infra',
			icons: [
				{ name: 'Snowflake', id: 'simple-icons:snowflake' },
				{ name: 'dbt', id: 'simple-icons:dbt' },
				{ name: 'Airflow', id: 'simple-icons:apacheairflow' },
				{ name: 'Spark', id: 'simple-icons:apachespark' },
				{ name: 'Kafka', id: 'simple-icons:apachekafka' },
				{ name: 'PostgreSQL', id: 'simple-icons:postgresql' },
				{ name: 'MongoDB', id: 'simple-icons:mongodb' },
				{ name: 'Redis', id: 'simple-icons:redis' },
				{ name: 'Elasticsearch', id: 'simple-icons:elasticsearch' },
				{ name: 'Databricks', id: 'simple-icons:databricks' },
				{ name: 'BigQuery', id: 'simple-icons:googlebigquery' },
				{ name: 'Redshift', id: 'simple-icons:amazonredshift' }
			]
		},
		{
			label: 'Cloud & DevOps',
			icons: [
				{ name: 'AWS', id: 'simple-icons:amazonaws' },
				{ name: 'GCP', id: 'simple-icons:googlecloud' },
				{ name: 'Azure', id: 'simple-icons:microsoftazure' },
				{ name: 'Terraform', id: 'simple-icons:terraform' },
				{ name: 'Docker', id: 'simple-icons:docker' },
				{ name: 'Kubernetes', id: 'simple-icons:kubernetes' },
				{ name: 'Jenkins', id: 'simple-icons:jenkins' },
				{ name: 'CircleCI', id: 'simple-icons:circleci' },
				{ name: 'ArgoCD', id: 'simple-icons:argo' },
				{ name: 'Datadog', id: 'simple-icons:datadog' },
				{ name: 'Grafana', id: 'simple-icons:grafana' },
				{ name: 'PagerDuty', id: 'simple-icons:pagerduty' }
			]
		},
		{
			label: 'General',
			icons: [
				{ name: 'Link', id: 'material-symbols:link' },
				{ name: 'Docs', id: 'material-symbols:description' },
				{ name: 'Dashboard', id: 'material-symbols:dashboard' },
				{ name: 'Code', id: 'material-symbols:code' },
				{ name: 'Database', id: 'material-symbols:database' },
				{ name: 'API', id: 'material-symbols:api' },
				{ name: 'Web', id: 'material-symbols:globe' },
				{ name: 'Monitor', id: 'material-symbols:monitor-heart' },
				{ name: 'Alert', id: 'material-symbols:notifications' },
				{ name: 'Chart', id: 'material-symbols:bar-chart' },
				{ name: 'Table', id: 'material-symbols:table' },
				{ name: 'Settings', id: 'material-symbols:settings' },
				{ name: 'Book', id: 'material-symbols:menu-book' },
				{ name: 'Bug', id: 'material-symbols:bug-report' },
				{ name: 'Lock', id: 'material-symbols:lock' },
				{ name: 'Key', id: 'material-symbols:key' },
				{ name: 'Email', id: 'material-symbols:mail' },
				{ name: 'Folder', id: 'material-symbols:folder' }
			]
		}
	];

	function selectIcon(iconId: string) {
		value = iconId;
		onselect?.(iconId);
		open = false;
		searchQuery = '';
		searchResults = [];
	}

	function handleSearchInput() {
		if (searchTimeout) clearTimeout(searchTimeout);
		if (!searchQuery.trim()) {
			searchResults = [];
			return;
		}
		searching = true;
		searchTimeout = setTimeout(() => searchIcons(searchQuery.trim()), 300);
	}

	async function loadIconNames(): Promise<Record<string, string[]>> {
		if (!iconNamesData) {
			const mod = await import('$lib/icon-names.generated');
			iconNamesData = mod.iconNames;
		}
		return iconNamesData;
	}

	async function searchIcons(query: string) {
		try {
			const data = await loadIconNames();
			const q = query.toLowerCase();
			const results: { prefix: string; name: string }[] = [];

			for (const [prefix, names] of Object.entries(data)) {
				for (const name of names) {
					if (name.includes(q)) {
						results.push({ prefix, name });
						if (results.length >= 24) break;
					}
				}
				if (results.length >= 24) break;
			}

			searchResults = results;
		} catch {
			searchResults = [];
		} finally {
			searching = false;
		}
	}

	function handleClickOutside(event: MouseEvent) {
		if (pickerEl && !pickerEl.contains(event.target as Node)) {
			open = false;
		}
	}

	$effect(() => {
		if (open) {
			document.addEventListener('click', handleClickOutside, true);
			return () => document.removeEventListener('click', handleClickOutside, true);
		}
	});
</script>

<div class="relative" bind:this={pickerEl}>
	<button
		type="button"
		onclick={() => (open = !open)}
		class="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
		title={value || 'Choose icon'}
	>
		<IconifyIcon
			icon={value || 'material-symbols:image-outline'}
			class="w-4 h-4 {value
				? 'text-gray-700 dark:text-gray-200'
				: 'text-gray-400 dark:text-gray-500'}"
		/>
		<span class="text-gray-500 dark:text-gray-400">{value ? 'Change icon' : 'Icon'}</span>
	</button>

	{#if open}
		<div
			class="absolute top-full left-0 mt-1 z-50 w-72 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700"
		>
			<div class="p-2 border-b border-gray-200 dark:border-gray-700">
				<input
					type="text"
					bind:value={searchQuery}
					oninput={handleSearchInput}
					placeholder="Search icons..."
					class="w-full px-2.5 py-1.5 text-xs border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 focus:ring-1 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-600"
					autofocus
				/>
			</div>

			<div class="max-h-56 overflow-y-auto p-2">
				{#if searchQuery.trim()}
					{#if searching}
						<p class="text-xs text-gray-400 text-center py-4">Searching...</p>
					{:else if searchResults.length > 0}
						<div class="grid grid-cols-6 gap-1">
							{#each searchResults as result}
								{@const fullId = `${result.prefix}:${result.name}`}
								<button
									type="button"
									onclick={() => selectIcon(fullId)}
									class="flex items-center justify-center w-10 h-10 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors {value ===
									fullId
										? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 ring-1 ring-earthy-terracotta-500'
										: ''}"
									title={fullId}
								>
									<IconifyIcon icon={fullId} class="w-5 h-5 text-gray-700 dark:text-gray-200" />
								</button>
							{/each}
						</div>
					{:else}
						<p class="text-xs text-gray-400 text-center py-4">No icons found</p>
					{/if}
				{:else}
					{#each commonIcons as category}
						<div class="mb-2 last:mb-0">
							<p class="text-[10px] font-medium text-gray-400 uppercase tracking-wide mb-1 px-0.5">
								{category.label}
							</p>
							<div class="grid grid-cols-6 gap-1">
								{#each category.icons as icon}
									<button
										type="button"
										onclick={() => selectIcon(icon.id)}
										class="flex items-center justify-center w-10 h-10 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors {value ===
										icon.id
											? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 ring-1 ring-earthy-terracotta-500'
											: ''}"
										title={icon.name}
									>
										<IconifyIcon icon={icon.id} class="w-5 h-5 text-gray-700 dark:text-gray-200" />
									</button>
								{/each}
							</div>
						</div>
					{/each}
				{/if}
			</div>

			{#if value}
				<div
					class="border-t border-gray-200 dark:border-gray-700 px-2.5 py-1.5 flex items-center justify-between"
				>
					<span class="text-[10px] text-gray-400 font-mono truncate">{value}</span>
					<button
						type="button"
						onclick={() => selectIcon('')}
						class="text-xs text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300"
					>
						Remove
					</button>
				</div>
			{/if}
		</div>
	{/if}
</div>
