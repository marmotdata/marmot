<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import type { DataProduct, DataProductsListResponse } from '$lib/dataproducts/types';
	import Button from '../../components/Button.svelte';
	import Icon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';

	let dataProducts = $state<DataProduct[]>([]);
	let totalProducts = $state(0);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let showDeleteConfirm = $state(false);
	let productToDelete = $state<DataProduct | null>(null);

	let canManageDataProducts = $derived(auth.hasPermission('assets', 'manage'));

	async function fetchProducts() {
		isLoading = true;
		error = null;

		try {
			const response = await fetchApi('/products/list?limit=100&offset=0');

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to fetch data products');
			}

			const data: DataProductsListResponse = await response.json();
			dataProducts = (data.data_products || []).map((product) => ({
				...product,
				tags: product.tags || [],
				metadata: product.metadata || {},
				owners: product.owners || [],
				rules: product.rules || []
			}));
			totalProducts = data.total || 0;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to fetch data products';
		} finally {
			isLoading = false;
		}
	}

	function handleNewProduct() {
		goto('/products/new');
	}

	function handleViewProduct(product: DataProduct) {
		goto(`/products/${product.id}`);
	}

	function confirmDelete(product: DataProduct) {
		productToDelete = product;
		showDeleteConfirm = true;
	}

	async function deleteProduct() {
		if (!productToDelete) return;

		try {
			const response = await fetchApi(`/products/${productToDelete.id}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to delete data product');
			}

			showDeleteConfirm = false;
			productToDelete = null;
			fetchProducts();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete data product';
		}
	}

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	onMount(() => {
		fetchProducts();
	});
</script>

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center justify-between">
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Data Products</h1>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Organize and manage collections of related data assets
					</p>
				</div>
				{#if canManageDataProducts}
					<Button
						click={handleNewProduct}
						icon="material-symbols:add"
						text="New Data Product"
						variant="filled"
					/>
				{/if}
			</div>
		</div>
	</div>

	<!-- Main Content -->
	<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		{#if error}
			<div
				class="mb-6 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
			>
				<div class="flex items-start">
					<Icon icon="material-symbols:error" class="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0" />
					<p class="ml-3 text-sm text-red-700 dark:text-red-300">{error}</p>
				</div>
			</div>
		{/if}

		{#if isLoading}
			<div class="flex justify-center items-center py-20">
				<div
					class="animate-spin rounded-full h-10 w-10 border-b-2 border-earthy-terracotta-700 dark:border-earthy-terracotta-500"
				></div>
			</div>
		{:else if dataProducts.length === 0}
			<div
				class="text-center py-20 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700"
			>
				<Icon
					icon="material-symbols:inventory-2-outline"
					class="mx-auto h-16 w-16 text-gray-300 dark:text-gray-600"
				/>
				<h3 class="mt-4 text-lg font-medium text-gray-900 dark:text-gray-100">
					No Data Products Yet
				</h3>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400 max-w-md mx-auto">
					Data products help you organize related assets into logical collections. Create your first
					one to get started.
				</p>
				{#if canManageDataProducts}
					<div class="mt-6">
						<Button
							click={handleNewProduct}
							icon="material-symbols:add"
							text="Create Your First Data Product"
							variant="filled"
						/>
					</div>
				{/if}
			</div>
		{:else}
			<!-- Stats Bar -->
			<div class="mb-6 text-sm text-gray-500 dark:text-gray-400">
				{totalProducts}
				{totalProducts === 1 ? 'data product' : 'data products'}
			</div>

			<!-- Data Products Grid -->
			<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
				{#each dataProducts as product}
					<div
						class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 hover:shadow-lg hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all group"
					>
						<button
							onclick={() => handleViewProduct(product)}
							class="w-full text-left p-5"
						>
							<div class="flex items-start justify-between">
								<div class="flex-1 min-w-0">
									<h3
										class="text-base font-semibold text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors"
									>
										{product.name}
									</h3>
									{#if product.description}
										<p
											class="mt-1 text-sm text-gray-500 dark:text-gray-400 line-clamp-2"
										>
											{product.description.replace(/<[^>]*>/g, '').slice(0, 120)}
										</p>
									{:else}
										<p class="mt-1 text-sm text-gray-400 dark:text-gray-500 italic">
											No description
										</p>
									{/if}
								</div>
								<Icon
									icon="material-symbols:chevron-right"
									class="h-5 w-5 text-gray-400 group-hover:text-earthy-terracotta-600 transition-colors flex-shrink-0 ml-2"
								/>
							</div>

							<!-- Stats -->
							<div class="mt-4 flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
								<span class="flex items-center gap-1">
									<Icon icon="material-symbols:database" class="h-3.5 w-3.5" />
									{product.asset_count || 0} assets
								</span>
								{#if product.rules && product.rules.length > 0}
									<span class="flex items-center gap-1">
										<Icon icon="material-symbols:filter-list" class="h-3.5 w-3.5" />
										{product.rules.length} rule{product.rules.length === 1 ? '' : 's'}
									</span>
								{/if}
								{#if product.owners && product.owners.length > 0}
									<span class="flex items-center gap-1">
										<Icon icon="material-symbols:person" class="h-3.5 w-3.5" />
										{product.owners.length}
									</span>
								{/if}
							</div>

							<!-- Tags -->
							{#if product.tags && product.tags.length > 0}
								<div class="mt-3 flex flex-wrap gap-1.5">
									{#each product.tags.slice(0, 3) as tag}
										<span
											class="px-2 py-0.5 text-xs rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300"
										>
											{tag}
										</span>
									{/each}
									{#if product.tags.length > 3}
										<span
											class="px-2 py-0.5 text-xs rounded-full bg-gray-100 dark:bg-gray-700 text-gray-500"
										>
											+{product.tags.length - 3}
										</span>
									{/if}
								</div>
							{/if}
						</button>

						<!-- Footer with actions -->
						<div
							class="px-5 py-3 border-t border-gray-100 dark:border-gray-700/50 flex items-center justify-between"
						>
							<span class="text-xs text-gray-400 dark:text-gray-500">
								Updated {formatDate(product.updated_at)}
							</span>
							{#if canManageDataProducts}
								<button
									onclick={(e) => {
										e.stopPropagation();
										confirmDelete(product);
									}}
									class="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
									title="Delete"
								>
									<Icon icon="material-symbols:delete-outline" class="h-4 w-4" />
								</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<!-- Delete Confirm Modal -->
{#if showDeleteConfirm && productToDelete}
	<div class="fixed inset-0 z-50 overflow-y-auto">
		<div class="flex items-center justify-center min-h-screen px-4">
			<button
				class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity"
				onclick={() => (showDeleteConfirm = false)}
			></button>

			<div
				class="relative bg-white/95 dark:bg-gray-800/95 backdrop-blur-md rounded-xl shadow-2xl max-w-lg w-full p-6 z-10 border border-gray-200/50 dark:border-gray-700/50"
			>
				<h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
					Delete Data Product
				</h3>
				<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
					Are you sure you want to delete "{productToDelete.name}"? This action cannot be undone.
				</p>
				<div class="flex justify-end gap-3">
					<button
						onclick={() => (showDeleteConfirm = false)}
						class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
					>
						Cancel
					</button>
					<button
						onclick={deleteProduct}
						class="px-4 py-2 rounded-md text-sm font-medium text-white bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600"
					>
						Delete
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
