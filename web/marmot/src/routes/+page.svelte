<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Icon from '@iconify/svelte';
	import GettingStarted from '$components/ui/GettingStarted.svelte';
	import AssetBlade from '$components/asset/AssetBlade.svelte';
	import IconComponent from '$components/ui/Icon.svelte';

	interface QuickStat {
		label: string;
		value: number;
		icon: string;
		href: string;
	}

	interface QuickAction {
		title: string;
		description: string;
		icon: string;
		href: string;
		color: string;
	}

	interface RecentAsset {
		id: string;
		name: string;
		type: string;
		provider?: string;
		providers?: string[];
		updated_at?: string;
		mrn?: string;
	}

	interface PopularAsset {
		asset_id: string;
		asset_name: string;
		asset_type: string;
		asset_provider: string;
		count: number;
	}

	interface AssetSummaryResponse {
		types: { [key: string]: any };
		providers: { [key: string]: number };
		tags: { [key: string]: number };
	}

	interface UserProfile {
		id: string;
		username: string;
		name: string;
		email: string;
		display_name?: string;
	}

	let summary = $state<AssetSummaryResponse>({
		types: {},
		providers: {},
		tags: {}
	});
	let popularAssets = $state<PopularAsset[]>([]);
	let userAssets = $state<RecentAsset[]>([]);
	let isLoading = $state(true);
	let hasLoadedOnce = $state(false);
	let userProfile = $state<UserProfile | null>(null);
	let selectedAsset = $state<any>(null);

	let totalAssets = $derived(
		Object.values(summary.types).reduce((sum, type: any) => sum + (type.count || 0), 0)
	);

	let quickStats = $derived<QuickStat[]>([
		{
			label: 'Total Assets',
			value: totalAssets,
			icon: 'material-symbols:database',
			href: '/discover'
		},
		{
			label: 'Asset Types',
			value: Object.keys(summary.types).length,
			icon: 'material-symbols:category',
			href: '/discover'
		},
		{
			label: 'Data Sources',
			value: Object.keys(summary.providers).length,
			icon: 'material-symbols:cloud',
			href: '/discover'
		}
	]);

	const quickActions: QuickAction[] = [
		{
			title: 'Explore Assets',
			description: 'Browse and discover your data assets',
			icon: 'material-symbols:database',
			href: '/discover',
			color: 'terracotta'
		},
		{
			title: 'Check Metrics',
			description: 'Platform usage and insights',
			icon: 'material-symbols:area-chart-rounded',
			href: '/metrics',
			color: 'green'
		},
		{
			title: 'Browse Glossary',
			description: 'Business terminology and definitions',
			icon: 'material-symbols:book',
			href: '/glossary',
			color: 'purple'
		},
		{
			title: 'Query Marmot',
			description: 'Learn how to search and query',
			icon: 'material-symbols:search',
			href: 'https://marmotdata.io/docs/queries',
			color: 'blue'
		}
	];

	let topAssetTypes = $derived(
		Object.entries(summary.types)
			.map(([type, data]: [string, any]) => ({
				type,
				count: data.count || 0,
				service: data.service
			}))
			.sort((a, b) => b.count - a.count)
			.slice(0, 5)
	);

	let topProviders = $derived(
		Object.entries(summary.providers)
			.map(([provider, count]) => ({ provider, count }))
			.sort((a, b) => b.count - a.count)
			.slice(0, 5)
	);

	let topTags = $derived(
		Object.entries(summary.tags)
			.map(([tag, count]) => ({ tag, count }))
			.sort((a, b) => b.count - a.count)
			.slice(0, 12)
	);

	let hasAssets = $derived(
		hasLoadedOnce &&
			!isLoading &&
			(Object.keys(summary.types).length > 0 ||
				Object.keys(summary.providers).length > 0 ||
				Object.keys(summary.tags).length > 0)
	);

	let showGettingStarted = $derived(
		hasLoadedOnce &&
			!isLoading &&
			Object.keys(summary.types).length === 0 &&
			Object.keys(summary.providers).length === 0 &&
			Object.keys(summary.tags).length === 0
	);

	function capitalizeFirstLetter(str: string): string {
		return str.charAt(0).toUpperCase() + str.slice(1);
	}

	function getIconType(asset: RecentAsset): string {
		if (asset.providers && Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		if (asset.provider) {
			return asset.provider;
		}
		return asset.type;
	}

	let displayName = $derived.by(() => {
		if (userProfile?.name) {
			const firstName = userProfile.name.split(' ')[0];
			return capitalizeFirstLetter(firstName);
		}
		return capitalizeFirstLetter(userProfile?.display_name || userProfile?.username || 'there');
	});

	async function fetchData() {
		try {
			const [summaryRes, userAssetsRes] = await Promise.all([
				fetchApi('/assets/summary'),
				fetchApi('/assets/my-assets?limit=6')
			]);

			if (summaryRes.ok) {
				summary = await summaryRes.json();
			}
			if (userAssetsRes.ok) {
				const userData = await userAssetsRes.json();
				userAssets = userData.assets || [];
			}

			try {
				const now = new Date();
				const thirtyDaysAgo = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
				const popularRes = await fetchApi(
					`/metrics/top-assets?start=${thirtyDaysAgo.toISOString()}&end=${now.toISOString()}&limit=6`
				);
				if (popularRes.ok) {
					const data = await popularRes.json();
					popularAssets = data.assets || [];
				}
			} catch {
				// Metrics are optional
			}

			try {
				const profileRes = await fetchApi('/users/me');
				if (profileRes.ok) {
					userProfile = await profileRes.json();
				}
			} catch {
				// Profile fetch is optional
			}
		} catch (err) {
			console.error('Error fetching data:', err);
		} finally {
			isLoading = false;
			hasLoadedOnce = true;
		}
	}

	function handleAssetClick(asset: RecentAsset | PopularAsset) {
		selectedAsset = asset;
	}

	function getAssetUrl(asset: RecentAsset | PopularAsset): string {
		// Extract type, service, and full name from MRN
		let mrn = '';
		if ('mrn' in asset && asset.mrn) {
			mrn = asset.mrn;
		}

		if (!mrn) {
			// Fallback for assets without MRN
			const type = 'asset_type' in asset ? asset.asset_type : asset.type;
			const name = 'asset_name' in asset ? asset.asset_name : asset.name;
			let provider = '';
			if ('asset_provider' in asset) {
				provider = asset.asset_provider;
			} else if ('provider' in asset && asset.provider) {
				provider = asset.provider;
			} else if ('providers' in asset && asset.providers && asset.providers.length > 0) {
				provider = asset.providers[0];
			}
			return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(provider)}/${encodeURIComponent(name)}`;
		}

		// Parse MRN: mrn://type/service/full.qualified.name
		const mrnParts = mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '#';
		const type = mrnParts[0];
		const service = mrnParts[1];
		const fullName = mrnParts.slice(2).join('/');
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(fullName)}`;
	}

	function navigateToAsset(asset: RecentAsset | PopularAsset) {
		goto(getAssetUrl(asset));
	}

	function navigateToType(type: string) {
		goto(`/discover?types=${encodeURIComponent(type)}`);
	}

	function navigateToProvider(provider: string) {
		goto(`/discover?providers=${encodeURIComponent(provider)}`);
	}

	function navigateToTag(tag: string) {
		goto(`/discover?tags=${encodeURIComponent(tag)}`);
	}

	function getColorClasses(color: string) {
		const colors: Record<string, { bg: string; text: string }> = {
			terracotta: {
				bg: 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20',
				text: 'text-earthy-terracotta-700 dark:text-earthy-terracotta-500'
			},
			blue: {
				bg: 'bg-blue-100 dark:bg-blue-900/20',
				text: 'text-blue-700 dark:text-blue-500'
			},
			green: {
				bg: 'bg-green-100 dark:bg-green-900/20',
				text: 'text-green-700 dark:text-green-500'
			},
			purple: {
				bg: 'bg-purple-100 dark:bg-purple-900/20',
				text: 'text-purple-700 dark:text-purple-500'
			}
		};
		return colors[color] || colors.terracotta;
	}

	function getProviderIcon(provider: string): string {
		const iconMap: Record<string, string> = {
			bigquery: 'googlebigquery',
			kafka: 'apachekafka',
			spark: 'apachespark',
			airflow: 'apacheairflow',
			flink: 'apacheflink',
			hive: 'apachehive',
			openapi: 'openapiinitiative'
		};
		const normalizedProvider = provider.toLowerCase();
		return iconMap[normalizedProvider] || normalizedProvider;
	}

	onMount(fetchData);
</script>

<div class="container max-w-7xl mx-auto py-8 px-4 sm:px-6 lg:px-8">
	{#if !showGettingStarted}
		<!-- Greeting -->
		<div class="mb-8">
			<h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100">
				Hi {displayName} 👋
			</h1>
			<p class="text-gray-600 dark:text-gray-400 mt-2">Here's what's in your data catalog</p>
		</div>

		<!-- Quick Stats -->
		<div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
			{#each quickStats as stat}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<div class="flex items-center justify-between mb-2">
						<Icon icon={stat.icon} class="w-8 h-8 text-gray-400 dark:text-gray-500" />
					</div>
					{#if !isLoading}
						<div class="text-3xl font-bold text-gray-900 dark:text-gray-100 mb-1">
							{stat.value.toLocaleString()}
						</div>
					{:else}
						<div class="w-20 h-9 bg-gray-200 dark:bg-gray-700 animate-pulse rounded mb-1"></div>
					{/if}
					<p class="text-sm font-medium text-gray-600 dark:text-gray-400">{stat.label}</p>
				</div>
			{/each}
		</div>

		<!-- Quick Actions -->
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
			{#each quickActions as action}
				{@const colors = getColorClasses(action.color)}
				<a
					href={action.href}
					target={action.href.startsWith('http') ? '_blank' : undefined}
					rel={action.href.startsWith('http') ? 'noopener noreferrer' : undefined}
					class="group bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 hover:border-earthy-terracotta-500 dark:hover:border-earthy-terracotta-600 hover:shadow-lg transition-all"
				>
					<div class="flex items-start gap-4">
						<div
							class="flex-shrink-0 w-10 h-10 {colors.bg} rounded-lg flex items-center justify-center"
						>
							<Icon icon={action.icon} class="w-6 h-6 {colors.text}" />
						</div>
						<div class="flex-1 min-w-0">
							<h3
								class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-1 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
							>
								{action.title}
							</h3>
							<p class="text-xs text-gray-600 dark:text-gray-400 line-clamp-2">
								{action.description}
							</p>
						</div>
					</div>
				</a>
			{/each}
		</div>

		{#if hasAssets}
			<!-- Two Column Layout -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
				<!-- Popular Assets -->
				{#if popularAssets.length > 0}
					<div>
						<div class="flex items-center justify-between mb-4">
							<h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">Most Viewed</h2>
							<a
								href="/metrics"
								class="text-sm text-earthy-terracotta-700 dark:text-earthy-terracotta-500 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-400 font-medium"
							>
								See all →
							</a>
						</div>
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 divide-y divide-gray-200 dark:divide-gray-700"
						>
							{#each popularAssets as asset}
								<button
									onclick={() => navigateToAsset(asset)}
									class="w-full flex items-center gap-4 p-4 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group"
								>
									<div
										class="w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-700 flex items-center justify-center flex-shrink-0"
									>
										<Icon
											icon={`simple-icons:${getProviderIcon(asset.asset_provider)}`}
											class="w-6 h-6 text-gray-600 dark:text-gray-400"
										/>
									</div>
									<div class="flex-1 min-w-0 text-left">
										<h3
											class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
										>
											{asset.asset_name}
										</h3>
										<p class="text-xs text-gray-600 dark:text-gray-400 capitalize">
											{asset.asset_type}
										</p>
									</div>
									<div class="flex items-center gap-2 flex-shrink-0">
										<Icon icon="material-symbols:visibility" class="w-4 h-4 text-gray-400" />
										<span class="text-sm font-medium text-gray-600 dark:text-gray-400">
											{asset.count.toLocaleString()}
										</span>
									</div>
								</button>
							{/each}
						</div>
					</div>
				{:else}
					<!-- User Assets fallback -->
					<div>
						<div class="flex items-center justify-between mb-4">
							<h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">Your Assets</h2>
							<a
								href="/discover"
								class="text-sm text-earthy-terracotta-700 dark:text-earthy-terracotta-500 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-400 font-medium"
							>
								View all →
							</a>
						</div>
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700"
						>
							{#if userAssets.length > 0}
								<div class="divide-y divide-gray-200 dark:divide-gray-700">
									{#each userAssets as asset}
										<a
											href={getAssetUrl(asset)}
											onclick={(e) => {
												e.preventDefault();
												handleAssetClick(asset);
											}}
											class="flex items-center gap-4 p-4 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group"
										>
											<div class="flex-shrink-0">
												<IconComponent name={getIconType(asset)} showLabel={false} size="sm" />
											</div>
											<div class="flex-1 min-w-0 text-left">
												<h3
													class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
												>
													{asset.name}
												</h3>
												<p class="text-xs text-gray-600 dark:text-gray-400 truncate font-mono">
													{asset.mrn}
												</p>
											</div>
											<span
												class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"
											>
												{asset.type}
											</span>
										</a>
									{/each}
								</div>
							{:else}
								<div class="flex flex-col items-center justify-center py-12 px-4">
									<div
										class="w-16 h-16 rounded-full bg-gray-100 dark:bg-gray-700 flex items-center justify-center mb-4"
									>
										<Icon
											icon="material-symbols:inbox"
											class="w-8 h-8 text-gray-400 dark:text-gray-500"
										/>
									</div>
									<p class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-1">
										No assets currently assigned to you
									</p>
									<p class="text-xs text-gray-600 dark:text-gray-400">
										Check back later or browse all assets
									</p>
								</div>
							{/if}
						</div>
					</div>
				{/if}

				<!-- Data Breakdown -->
				<div>
					<h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-4">Overview</h2>
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
					>
						<!-- Asset Types -->
						<div class="mb-6">
							<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
								Top Asset Types
							</h3>
							<div class="space-y-3">
								{#each topAssetTypes as item}
									<button
										onclick={() => navigateToType(item.type)}
										class="w-full flex items-center justify-between group"
									>
										<span
											class="text-sm text-gray-900 dark:text-gray-100 capitalize group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
										>
											{item.type}
										</span>
										<div class="flex items-center gap-3">
											<div
												class="w-32 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden"
											>
												<div
													class="h-full bg-earthy-terracotta-500 transition-all"
													style="width: {(item.count / totalAssets) * 100}%"
												></div>
											</div>
											<span
												class="text-sm font-semibold text-gray-600 dark:text-gray-400 w-12 text-right"
											>
												{item.count}
											</span>
										</div>
									</button>
								{/each}
							</div>
						</div>

						<!-- Data Sources -->
						<div>
							<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
								Top Data Sources
							</h3>
							<div class="space-y-3">
								{#each topProviders as item}
									<button
										onclick={() => navigateToProvider(item.provider)}
										class="w-full flex items-center justify-between group"
									>
										<div class="flex items-center gap-2">
											<Icon
												icon={`simple-icons:${getProviderIcon(item.provider)}`}
												class="w-4 h-4 text-gray-600 dark:text-gray-400"
											/>
											<span
												class="text-sm text-gray-900 dark:text-gray-100 capitalize group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
											>
												{item.provider}
											</span>
										</div>
										<div class="flex items-center gap-3">
											<div
												class="w-32 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden"
											>
												<div
													class="h-full bg-blue-500 transition-all"
													style="width: {(item.count / totalAssets) * 100}%"
												></div>
											</div>
											<span
												class="text-sm font-semibold text-gray-600 dark:text-gray-400 w-12 text-right"
											>
												{item.count}
											</span>
										</div>
									</button>
								{/each}
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Popular Tags -->
			{#if topTags.length > 0}
				<div>
					<h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-4">Popular Tags</h2>
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
					>
						<div class="flex flex-wrap gap-2">
							{#each topTags as item}
								<button
									onclick={() => navigateToTag(item.tag)}
									class="inline-flex items-center gap-2 px-3 py-1.5 bg-gray-100 dark:bg-gray-700 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/20 rounded-full transition-colors group"
								>
									<Icon
										icon="material-symbols:label"
										class="w-3.5 h-3.5 text-gray-600 dark:text-gray-400 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500"
									/>
									<span
										class="text-sm font-medium text-gray-900 dark:text-gray-100 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500"
									>
										{item.tag}
									</span>
									<span
										class="text-xs font-semibold text-gray-600 dark:text-gray-400 bg-white dark:bg-gray-800 px-1.5 py-0.5 rounded-full"
									>
										{item.count}
									</span>
								</button>
							{/each}
						</div>
					</div>
				</div>
			{/if}
		{:else}
			<!-- Getting Started - shown in two column layout -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
				<div class="lg:col-span-2">
					<GettingStarted condensed={false} />
				</div>
			</div>
		{/if}
	{:else}
		<!-- Greeting -->
		<div class="mb-8">
			<h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100">
				Hi {displayName} 👋
			</h1>
			<p class="text-gray-600 dark:text-gray-400 mt-2">Here's what's in your data catalog</p>
		</div>

		<!-- Quick Stats -->
		<div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
			{#each quickStats as stat}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<div class="flex items-center justify-between mb-2">
						<Icon icon={stat.icon} class="w-8 h-8 text-gray-400 dark:text-gray-500" />
					</div>
					{#if !isLoading}
						<div class="text-3xl font-bold text-gray-900 dark:text-gray-100 mb-1">
							{stat.value.toLocaleString()}
						</div>
					{:else}
						<div class="w-20 h-9 bg-gray-200 dark:bg-gray-700 animate-pulse rounded mb-1"></div>
					{/if}
					<p class="text-sm font-medium text-gray-600 dark:text-gray-400">{stat.label}</p>
				</div>
			{/each}
		</div>

		<!-- Quick Actions -->
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
			{#each quickActions as action}
				{@const colors = getColorClasses(action.color)}
				<a
					href={action.href}
					target={action.href.startsWith('http') ? '_blank' : undefined}
					rel={action.href.startsWith('http') ? 'noopener noreferrer' : undefined}
					class="group bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 hover:border-earthy-terracotta-500 dark:hover:border-earthy-terracotta-600 hover:shadow-lg transition-all"
				>
					<div class="flex items-start gap-4">
						<div
							class="flex-shrink-0 w-10 h-10 {colors.bg} rounded-lg flex items-center justify-center"
						>
							<Icon icon={action.icon} class="w-6 h-6 {colors.text}" />
						</div>
						<div class="flex-1 min-w-0">
							<h3
								class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-1 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
							>
								{action.title}
							</h3>
							<p class="text-xs text-gray-600 dark:text-gray-400 line-clamp-2">
								{action.description}
							</p>
						</div>
					</div>
				</a>
			{/each}
		</div>

		<!-- Getting Started - shown in two column layout -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
			<div class="lg:col-span-2">
				<GettingStarted condensed={false} />
			</div>
		</div>
	{/if}
</div>

{#if selectedAsset}
	<AssetBlade asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}
