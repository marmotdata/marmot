<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { writable, type Writable } from 'svelte/store';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import type { DataProduct } from '$lib/dataproducts/types';
	import ProductBlade from '$components/product/ProductBlade.svelte';
	import IconifyIcon from '@iconify/svelte';
	import AuthenticatedImage from '$components/ui/AuthenticatedImage.svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { formatDate, formatRelativeTime } from '$lib/utils';

	const allProducts: Writable<DataProduct[]> = writable([]);
	const totalProducts: Writable<number> = writable(0);
	const isLoading: Writable<boolean> = writable(true);
	const error: Writable<{ status: number; message: string } | null> = writable(null);

	let selectedProduct = $state<DataProduct | null>(null);
	let search = $state('');

	// The whole list is loaded at once, so search filters it in place without another request
	let shown = $derived.by(() => {
		const q = search.trim().toLowerCase();
		if (!q) return $allProducts;
		return $allProducts.filter(
			(p) =>
				p.name.toLowerCase().includes(q) ||
				(p.description ?? '').toLowerCase().includes(q) ||
				(p.tags ?? []).some((t) => t.toLowerCase().includes(q))
		);
	});
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
					message: errorData.error || m.products_error_generic()
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

			$allProducts = sortedByRecent;
		} catch (e: unknown) {
			const err = e as { status?: number; message?: string };
			const errorStatus = err.status || 500;
			$error = { status: errorStatus, message: err.message || m.products_error_unknown() };
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
</script>

<svelte:head>
	<title>{m.products_page_title()}</title>
</svelte:head>

<div class="w-full px-4 py-6 sm:px-6 lg:px-8">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
				{m.products_heading()}
			</h1>
			<p class="mt-1 max-w-3xl text-sm text-gray-600 dark:text-gray-400">
				{m.products_subheading()}
			</p>
		</div>
		{#if canManageProducts}
			<a
				href={resolve('/products/new')}
				class="inline-flex flex-shrink-0 items-center gap-2 self-start rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-earthy-terracotta-500"
			>
				<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
				{m.products_new_product()}
			</a>
		{/if}
	</div>

	<div class="mt-6">
		{#if $error}
			<div
				class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
			>
				{String($error.status).startsWith('5') ? m.products_error_server() : $error.message}
			</div>
		{:else if $isLoading}
			<div class="space-y-2">
				{#each Array(5) as _, i (i)}
					<div class="h-14 animate-pulse rounded-xl bg-gray-100 dark:bg-gray-800"></div>
				{/each}
			</div>
		{:else if $allProducts.length === 0}
			<div class="mx-auto max-w-lg py-16 text-center">
				<span
					class="mx-auto flex h-11 w-11 items-center justify-center rounded-xl bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400"
				>
					<IconifyIcon icon="material-symbols:inventory-2-outline" class="h-6 w-6" />
				</span>
				<h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
					{m.products_empty_title()}
				</h2>
				<p class="mx-auto mt-2 max-w-md text-sm text-gray-600 dark:text-gray-400">
					{m.products_empty_description()}
				</p>
				{#if canManageProducts}
					<a
						href={resolve('/products/new')}
						class="mt-6 inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500"
					>
						<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
						{m.products_create_first()}
					</a>
				{/if}
			</div>
		{:else}
			<div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
				<label
					class="flex w-full items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 py-2 focus-within:border-earthy-terracotta-500 focus-within:ring-2 focus-within:ring-earthy-terracotta-500/30 sm:w-80 dark:border-gray-600 dark:bg-gray-800"
				>
					<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
					<input
						bind:value={search}
						placeholder={m.products_search_placeholder()}
						aria-label={m.products_search_placeholder()}
						class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
					/>
				</label>
				<span class="text-sm text-gray-500 dark:text-gray-400">
					{m.products_product_count({ count: $totalProducts })}
				</span>
			</div>

			<div
				class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
			>
				<div
					class="hidden items-center gap-4 border-b border-gray-200 bg-gray-50 px-4 py-2 text-[11px] font-semibold uppercase tracking-wider text-gray-500 sm:flex dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-400"
					aria-hidden="true"
				>
					<span class="min-w-0 flex-1">{m.products_col_product()}</span>
					<span class="w-20 text-right">{m.products_assets()}</span>
					<span class="w-20 text-right">{m.products_col_owners()}</span>
					<span class="hidden w-48 lg:block">{m.common_tags()}</span>
					<span class="w-24 text-right">{m.products_col_updated()}</span>
				</div>
				{#if shown.length === 0}
					<p class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
						{m.products_no_match()}
					</p>
				{/if}
				<ul class="divide-y divide-gray-100 dark:divide-gray-700/60">
					{#each shown as product (product.id)}
						<li>
							<button
								type="button"
								onclick={() => handleProductClick(product.id)}
								class="group flex w-full items-center gap-4 px-4 py-3 text-left transition hover:bg-gray-50 dark:hover:bg-gray-700/30"
							>
								<span class="flex min-w-0 flex-1 items-center gap-3">
									<span
										class="flex h-9 w-9 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30"
									>
										{#if product.icon_url}
											<AuthenticatedImage
												src={product.icon_url}
												alt={m.products_icon_alt({ name: product.name })}
												class="h-full w-full object-cover"
											/>
										{:else}
											<IconifyIcon
												icon="material-symbols:inventory-2-outline"
												class="h-5 w-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-300"
											/>
										{/if}
									</span>
									<span class="min-w-0">
										<span
											class="block truncate font-medium text-gray-900 group-hover:text-earthy-terracotta-700 dark:text-gray-100 dark:group-hover:text-earthy-terracotta-400"
											>{product.name}</span
										>
										<span class="block truncate text-xs text-gray-500 dark:text-gray-400">
											{product.description
												? product.description.replace(/<[^>]*>/g, '')
												: m.products_no_description()}
										</span>
									</span>
								</span>
								<span class="w-20 text-right text-sm tabular-nums text-gray-700 dark:text-gray-300"
									>{product.asset_count || 0}</span
								>
								<span class="w-20 text-right text-sm tabular-nums text-gray-700 dark:text-gray-300"
									>{product.owners?.length || 0}</span
								>
								<span class="hidden w-48 items-center gap-1 overflow-hidden lg:flex">
									{#each (product.tags ?? []).slice(0, 2) as tag (tag)}
										<span
											class="truncate rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300"
											>{tag}</span
										>
									{/each}
									{#if (product.tags ?? []).length > 2}
										<span class="text-xs text-gray-400">+{(product.tags ?? []).length - 2}</span>
									{/if}
								</span>
								<span
									class="w-24 text-right text-xs text-gray-500 dark:text-gray-400"
									title={formatDate(product.updated_at)}
									>{formatRelativeTime(product.updated_at)}</span
								>
							</button>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	</div>
</div>

{#if selectedProduct}
	<ProductBlade bind:product={selectedProduct} onClose={() => (selectedProduct = null)} />
{/if}
