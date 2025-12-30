<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { writable, type Writable } from 'svelte/store';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import type { DataProduct } from '$lib/dataproducts/types';
	import ProductBlade from '$components/product/ProductBlade.svelte';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import { auth } from '$lib/stores/auth';

	const recentProducts: Writable<DataProduct[]> = writable([]);
	const allProducts: Writable<DataProduct[]> = writable([]);
	const totalProducts: Writable<number> = writable(0);
	const isLoading: Writable<boolean> = writable(true);
	const error: Writable<{ status: number; message: string } | null> = writable(null);

	let selectedProduct = $state<DataProduct | null>(null);
	let canManageProducts = $derived(auth.hasPermission('assets', 'manage'));

	$effect(() => {
		if (browser) {
			fetchProducts();
		}
	});

	async function fetchProducts() {
		$isLoading = true;
		$error = null;

		try {
			const response = await fetchApi('/products/list?limit=100');

			if (!response.ok) {
				const errorData = await response.json();
				throw {
					status: response.status,
					message: errorData.error || 'Unable to complete your request'
				};
			}

			const data = await response.json();

			const products = (data.data_products || []).map((product: DataProduct) => ({
				...product,
				tags: product.tags || [],
				metadata: product.metadata || {},
				owners: product.owners || [],
				rules: product.rules || []
			}));

			$totalProducts = data.total || 0;

			// Sort by updated_at for recent products (most recent first)
			const sortedByRecent = [...products].sort(
				(a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
			);

			$recentProducts = sortedByRecent.slice(0, 4);
			$allProducts = sortedByRecent;
		} catch (e: any) {
			const errorStatus = e.status || 500;
			$error = { status: errorStatus, message: e.message };
			console.error('Error fetching products:', e);
		} finally {
			$isLoading = false;
		}
	}

	async function handleProductClick(productId: string) {
		try {
			const response = await fetchApi(`/products/${productId}`);
			if (response.ok) {
				const product: DataProduct = await response.json();
				product.tags = product.tags || [];
				product.owners = product.owners || [];
				product.rules = product.rules || [];
				selectedProduct = product;
			} else {
				console.error('Failed to load product');
			}
		} catch (err) {
			console.error('Error loading product:', err);
		}
	}

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	function formatRelativeTime(dateString: string): string {
		const date = new Date(dateString);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'Just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		if (diffDays < 7) return `${diffDays}d ago`;
		return formatDate(dateString);
	}
</script>

<svelte:head>
	<title>Data Products - Marmot</title>
</svelte:head>

<div class="h-full overflow-y-auto">
	<div class="max-w-6xl mx-auto px-6 py-6">
		<!-- Header -->
		<div class="flex items-center justify-between mb-6">
			<div>
				<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Data Products</h1>
				<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
					Curated collections of data assets for your organization
				</p>
			</div>
			{#if canManageProducts}
				<Button
					click={() => goto('/products/new')}
					icon="material-symbols:add"
					text="New Product"
					variant="filled"
				/>
			{/if}
		</div>

		{#if $error}
			{#if String($error.status).startsWith('5')}
				<div
					class="bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-700 text-red-700 dark:text-red-100 px-4 py-3 rounded-lg"
				>
					Something went wrong on our end. Please try again later.
				</div>
			{:else}
				<div
					class="bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900 border border-earthy-terracotta-200 dark:border-earthy-terracotta-800 px-4 py-3 rounded-lg flex items-center gap-3"
				>
					<IconifyIcon
						icon="mdi:alert-circle-outline"
						class="w-5 h-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
					/>
					<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-100">
						{$error.message}
					</span>
				</div>
			{/if}
		{:else if $isLoading}
			<!-- Loading State -->
			<div class="space-y-6">
				<!-- Featured Cards Skeleton -->
				<div>
					<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-32 mb-4"></div>
					<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
						{#each Array(4) as _}
							<div
								class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 animate-pulse"
							>
								<div class="h-10 w-10 bg-gray-200 dark:bg-gray-700 rounded-lg mb-3"></div>
								<div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-3/4 mb-2"></div>
								<div class="h-3 bg-gray-200 dark:bg-gray-700 rounded w-full mb-3"></div>
								<div class="flex gap-2">
									<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-16"></div>
									<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-12"></div>
								</div>
							</div>
						{/each}
					</div>
				</div>
				<!-- List Skeleton -->
				<div>
					<div class="h-5 bg-gray-200 dark:bg-gray-700 rounded w-24 mb-4"></div>
					{#each Array(5) as _}
						<div
							class="bg-white dark:bg-gray-800 border-b border-gray-100 dark:border-gray-700 p-3 animate-pulse"
						>
							<div class="flex items-center gap-3">
								<div class="h-6 w-6 bg-gray-200 dark:bg-gray-700 rounded"></div>
								<div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-48"></div>
								<div class="flex-1"></div>
								<div class="h-3 bg-gray-200 dark:bg-gray-700 rounded w-20"></div>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{:else if $allProducts.length === 0}
			<!-- Empty State -->
			<div class="flex flex-col items-center justify-center py-16">
				<div
					class="w-20 h-20 rounded-2xl bg-gray-100 dark:bg-gray-800 flex items-center justify-center mb-4"
				>
					<IconifyIcon
						icon="mdi:package-variant-closed"
						class="text-4xl text-gray-400 dark:text-gray-500"
					/>
				</div>
				<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
					No data products yet
				</h2>
				<p class="text-sm text-gray-500 dark:text-gray-400 text-center max-w-md mb-6">
					Data products help you organize and share curated collections of data assets with your
					team.
				</p>
				{#if canManageProducts}
					<Button
						click={() => goto('/products/new')}
						icon="material-symbols:add"
						text="Create your first product"
						variant="filled"
					/>
				{/if}
			</div>
		{:else}
			<!-- Recently Updated Section -->
			<section class="mb-8">
				<h2
					class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider mb-4"
				>
					Recently Updated
				</h2>
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
					{#each $recentProducts as product}
						<button
							onclick={() => handleProductClick(product.id)}
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 text-left hover:shadow-lg hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all group"
						>
							<div
								class="w-10 h-10 rounded-lg bg-gradient-to-br from-earthy-terracotta-100 to-earthy-terracotta-200 dark:from-earthy-terracotta-900/50 dark:to-earthy-terracotta-800/30 flex items-center justify-center mb-3"
							>
								<IconifyIcon
									icon="mdi:package-variant-closed"
									class="text-xl text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
								/>
							</div>
							<h3
								class="font-semibold text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors mb-1"
							>
								{product.name}
							</h3>
							{#if product.description}
								<p class="text-xs text-gray-500 dark:text-gray-400 line-clamp-2 mb-3">
									{product.description.replace(/<[^>]*>/g, '').slice(0, 80)}
								</p>
							{:else}
								<p class="text-xs text-gray-400 dark:text-gray-500 italic mb-3">No description</p>
							{/if}
							<div class="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
								<span class="flex items-center gap-1">
									<IconifyIcon icon="material-symbols:database" class="w-3.5 h-3.5" />
									{product.asset_count || 0}
								</span>
								{#if product.owners && product.owners.length > 0}
									<span class="flex items-center gap-1">
										<IconifyIcon icon="material-symbols:person" class="w-3.5 h-3.5" />
										{product.owners.length}
									</span>
								{/if}
								<span class="flex-1"></span>
								<span class="text-gray-400 dark:text-gray-500">
									{formatRelativeTime(product.updated_at)}
								</span>
							</div>
						</button>
					{/each}
				</div>
			</section>

			<!-- All Products List -->
			<section>
				<div class="flex items-center justify-between mb-4">
					<h2
						class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider"
					>
						All Products
					</h2>
					<span class="text-xs text-gray-500 dark:text-gray-400">
						{$totalProducts}
						{$totalProducts === 1 ? 'product' : 'products'}
					</span>
				</div>
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
				>
					{#each $allProducts as product, index}
						<button
							onclick={() => handleProductClick(product.id)}
							class="w-full flex items-center gap-4 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group {index !==
							$allProducts.length - 1
								? 'border-b border-gray-100 dark:border-gray-700'
								: ''}"
						>
							<div
								class="w-8 h-8 rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 flex items-center justify-center flex-shrink-0"
							>
								<IconifyIcon
									icon="mdi:package-variant-closed"
									class="text-sm text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
								/>
							</div>
							<div class="flex-1 min-w-0">
								<h3
									class="font-medium text-sm text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors"
								>
									{product.name}
								</h3>
							</div>
							<div class="flex items-center gap-4 flex-shrink-0">
								<span
									class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1"
									title="Assets"
								>
									<IconifyIcon icon="material-symbols:database" class="w-3.5 h-3.5" />
									{product.asset_count || 0}
								</span>
								{#if product.tags && product.tags.length > 0}
									<div class="hidden sm:flex items-center gap-1">
										{#each product.tags.slice(0, 2) as tag}
											<span
												class="text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 px-2 py-0.5 rounded"
											>
												{tag}
											</span>
										{/each}
										{#if product.tags.length > 2}
											<span class="text-xs text-gray-400 dark:text-gray-500">
												+{product.tags.length - 2}
											</span>
										{/if}
									</div>
								{/if}
								<span
									class="text-xs text-gray-400 dark:text-gray-500 w-16 text-right"
									title={formatDate(product.updated_at)}
								>
									{formatRelativeTime(product.updated_at)}
								</span>
								<IconifyIcon
									icon="mdi:chevron-right"
									class="w-4 h-4 text-gray-400 dark:text-gray-500 opacity-0 group-hover:opacity-100 transition-opacity"
								/>
							</div>
						</button>
					{/each}
				</div>
			</section>
		{/if}
	</div>
</div>

{#if selectedProduct}
	<ProductBlade bind:product={selectedProduct} onClose={() => (selectedProduct = null)} />
{/if}
