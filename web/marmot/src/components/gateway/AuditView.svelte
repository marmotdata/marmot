<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import CodeBlock from '$components/editor/CodeBlock.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import { getErrorMessage } from '$lib/stores/toast';
	import { listAudit } from '$lib/gateway/api';
	import type { GatewayAuditEntry } from '$lib/gateway/types';

	const PAGE_SIZE = 25;
	const ASSET_CHIP_CAP = 24;

	let entries = $state<GatewayAuditEntry[]>([]);
	let loading = $state(true);
	let error = $state('');
	let decisionFilter = $state('');
	let targetFilter = $state('');
	let expanded = $state<string | null>(null);
	let showAllAssets = $state<Record<string, boolean>>({});
	let page = $state(0);
	let hasMore = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			// Fetch one extra row to learn whether a next page exists, so we
			// never run a COUNT over what can be millions of audit rows.
			const rows = await listAudit({
				decision: decisionFilter || undefined,
				target: targetFilter.trim() || undefined,
				limit: PAGE_SIZE + 1,
				offset: page * PAGE_SIZE
			});
			hasMore = rows.length > PAGE_SIZE;
			entries = rows.slice(0, PAGE_SIZE);
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	function applyFilters() {
		page = 0;
		load();
	}

	function nextPage() {
		if (!hasMore) return;
		page += 1;
		expanded = null;
		load();
	}

	function prevPage() {
		if (page === 0) return;
		page -= 1;
		expanded = null;
		load();
	}

	function toggleRow(id: string) {
		expanded = expanded === id ? null : id;
	}

	// mrn://type/service/name -> /discover/type/service/name. Returns null for
	// pseudo-MRNs (whole-target grants, engine-generated pushdown names) that
	// don't map to a catalogued asset.
	function assetPath(mrn: string): string | null {
		if (!mrn.startsWith('mrn://')) return null;
		const parts = mrn.replace('mrn://', '').split('/');
		if (parts.length < 3) return null;
		const [type, service, ...rest] = parts;
		const name = rest.join('/');
		if (type === 'target' || name === '_generated_query') return null;
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(name)}`;
	}

	function assetLabel(mrn: string): string {
		const parts = mrn.replace('mrn://', '').split('/');
		return parts.slice(2).join('/') || mrn;
	}

	// The MRN's service segment is the provider (e.g. mrn://table/postgresql/orders
	// -> postgresql), which the Icon component renders as the provider's logo.
	function assetProvider(mrn: string): string {
		const parts = mrn.replace('mrn://', '').split('/');
		return parts[1] || '';
	}

	function decisionClasses(entry: GatewayAuditEntry): string {
		return entry.decision === 'allowed'
			? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
			: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300';
	}

	function statusClasses(status: string): string {
		switch (status) {
			case 'completed':
				return 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300';
			case 'failed':
			case 'denied':
				return 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300';
			default:
				return 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300';
		}
	}

	function formatTime(value?: string): string {
		if (!value) return '—';
		return new Date(value).toLocaleString(undefined, {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});
	}

	function subjectParts(subject: string): { kind: string; name: string } {
		const idx = subject.indexOf(':');
		if (idx === -1) return { kind: '', name: subject };
		return { kind: subject.slice(0, idx), name: subject.slice(idx + 1) };
	}

	onMount(load);
</script>

<div class="space-y-5">
	<div class="flex flex-col lg:flex-row lg:items-end lg:justify-between gap-4">
		<div>
			<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Query audit log</h2>
			<p class="text-sm text-gray-500 dark:text-gray-400 mt-1 max-w-prose">
				Every query an agent ran or was refused. Who asked, what they asked, whether it was allowed,
				and what came back.
			</p>
		</div>
		<div class="flex items-center gap-2">
			<div class="relative">
				<select
					bind:value={decisionFilter}
					onchange={applyFilters}
					class="appearance-none pl-3 pr-9 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent"
				>
					<option value="">All decisions</option>
					<option value="allowed">Allowed</option>
					<option value="denied">Denied</option>
				</select>
				<IconifyIcon
					icon="material-symbols:expand-more"
					class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400"
				/>
			</div>
			<div class="relative">
				<IconifyIcon
					icon="material-symbols:search"
					class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400"
				/>
				<input
					type="text"
					bind:value={targetFilter}
					onchange={applyFilters}
					placeholder="Filter by target"
					class="pl-8 pr-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent"
				/>
			</div>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-16">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if error}
		<div
			class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
		>
			<p class="text-sm text-red-700 dark:text-red-300">{error}</p>
		</div>
	{:else if entries.length === 0}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-dashed border-gray-300 dark:border-gray-600 p-12 text-center"
		>
			<IconifyIcon
				icon="material-symbols:receipt-long-outline"
				class="h-10 w-10 mx-auto text-gray-300 dark:text-gray-600"
			/>
			<p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
				{page > 0 || decisionFilter || targetFilter
					? 'No audit entries match these filters.'
					: 'No queries have run through the gateway yet.'}
			</p>
		</div>
	{:else}
		<div
			class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden shadow-sm"
		>
			<div class="overflow-x-auto">
				<table class="w-full text-sm border-collapse">
					<thead>
						<tr
							class="text-left text-[11px] uppercase tracking-wider text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700"
						>
							<th class="pl-5 pr-2 py-3 font-semibold w-8"></th>
							<th class="px-3 py-3 font-semibold">When</th>
							<th class="px-3 py-3 font-semibold">Who</th>
							<th class="px-3 py-3 font-semibold">Target</th>
							<th class="px-3 py-3 font-semibold">Decision</th>
							<th class="px-3 py-3 font-semibold">Status</th>
							<th class="px-3 py-3 font-semibold text-right">Rows</th>
							<th class="px-3 py-3 font-semibold">Query</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-700/60">
						{#each entries as entry (entry.id)}
							{@const isDenied = entry.decision !== 'allowed'}
							{@const who = subjectParts(entry.audit_subject)}
							<tr
								class="group cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40 {expanded ===
								entry.id
									? 'bg-gray-50 dark:bg-gray-700/40'
									: ''}"
								onclick={() => toggleRow(entry.id)}
							>
								<td class="pl-5 pr-2 py-3 relative">
									{#if isDenied}
										<span
											class="absolute left-0 top-0 bottom-0 w-[3px] bg-red-400 dark:bg-red-500"
										></span>
									{/if}
									<IconifyIcon
										icon="material-symbols:chevron-right"
										class="h-4 w-4 text-gray-400 transition-transform {expanded === entry.id
											? 'rotate-90'
											: ''}"
									/>
								</td>
								<td class="px-3 py-3 whitespace-nowrap text-gray-600 dark:text-gray-400 tabular-nums">
									{formatTime(entry.started_at)}
								</td>
								<td class="px-3 py-3 whitespace-nowrap">
									{#if who.kind}
										<span class="text-gray-400 dark:text-gray-500">{who.kind}:</span>
									{/if}<span class="text-gray-900 dark:text-gray-100 font-medium">{who.name}</span>
								</td>
								<td class="px-3 py-3 whitespace-nowrap text-gray-700 dark:text-gray-300"
									>{entry.target_name}</td
								>
								<td class="px-3 py-3">
									<span
										class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {decisionClasses(
											entry
										)}">{entry.decision}</span
									>
								</td>
								<td class="px-3 py-3">
									<span
										class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {statusClasses(
											entry.status
										)}">{entry.status}</span
									>
								</td>
								<td
									class="px-3 py-3 text-right tabular-nums text-gray-700 dark:text-gray-300"
									>{entry.rows_returned ?? '—'}</td
								>
								<td class="px-3 py-3 max-w-md">
									<code
										class="block truncate text-xs text-gray-500 dark:text-gray-400 font-mono"
										>{entry.query_text}</code
									>
								</td>
							</tr>
							{#if expanded === entry.id}
								<tr class="bg-gray-50/70 dark:bg-gray-900/40">
									<td colspan="8" class="px-5 pb-5 pt-1">
										<div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
											<CodeBlock code={entry.query_text} language="sql" wrap />
										</div>

										<div class="mt-4 grid grid-cols-1 lg:grid-cols-3 gap-5">
											<div class="lg:col-span-2">
												<div
													class="text-[11px] uppercase tracking-wider font-semibold text-gray-500 dark:text-gray-400 mb-2"
												>
													Referenced assets
												</div>
												{#if (entry.referenced_mrns || []).length === 0}
													<p class="text-xs text-gray-400 dark:text-gray-500">None recorded.</p>
												{:else}
													{@const mrns = entry.referenced_mrns || []}
													{@const shown = showAllAssets[entry.id]
														? mrns
														: mrns.slice(0, ASSET_CHIP_CAP)}
													<div class="flex flex-wrap gap-1.5">
														{#each shown as mrn (mrn)}
															{@const path = assetPath(mrn)}
															{#if path}
																<a
																	href={path}
																	onclick={(e) => e.stopPropagation()}
																	class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-mono bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:border-earthy-terracotta-400 hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20 transition-colors"
																	title={mrn}
																>
																	<Icon name={assetProvider(mrn)} size="xs" showLabel={false} />
																	{assetLabel(mrn)}
																</a>
															{:else}
																<span
																	class="inline-flex items-center gap-1 px-2 py-1 rounded-md text-xs font-mono bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 text-gray-500 dark:text-gray-400"
																	title={mrn}
																>
																	{assetLabel(mrn)}
																</span>
															{/if}
														{/each}
														{#if mrns.length > ASSET_CHIP_CAP && !showAllAssets[entry.id]}
															<button
																onclick={(e) => {
																	e.stopPropagation();
																	showAllAssets = { ...showAllAssets, [entry.id]: true };
																}}
																class="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
															>
																+{mrns.length - ASSET_CHIP_CAP} more
															</button>
														{/if}
													</div>
												{/if}
											</div>

											<div class="space-y-2.5 text-xs">
												{#if entry.session_id}
													<div>
														<span class="text-gray-400 dark:text-gray-500">Session</span>
														<div class="font-mono text-gray-700 dark:text-gray-300">
															{entry.session_id.slice(0, 8)}
														</div>
													</div>
												{/if}
												{#if entry.decision_detail?.reason}
													<div>
														<span class="text-gray-400 dark:text-gray-500">Reason</span>
														<div class="text-gray-700 dark:text-gray-300">
															{entry.decision_detail.reason}
														</div>
													</div>
												{/if}
												{#if entry.error}
													<div>
														<span class="text-gray-400 dark:text-gray-500">Error</span>
														<div class="text-red-600 dark:text-red-400 font-mono break-words">
															{entry.error}
														</div>
													</div>
												{/if}
												<div>
													<span class="text-gray-400 dark:text-gray-500">Completed</span>
													<div class="text-gray-700 dark:text-gray-300 tabular-nums">
														{formatTime(entry.completed_at)}
													</div>
												</div>
											</div>
										</div>
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			</div>

			<div
				class="flex items-center justify-between px-5 py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/30"
			>
				<span class="text-xs text-gray-500 dark:text-gray-400 tabular-nums">
					Showing {page * PAGE_SIZE + 1}–{page * PAGE_SIZE + entries.length}
				</span>
				<div class="flex items-center gap-1">
					<button
						onclick={prevPage}
						disabled={page === 0}
						class="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-md text-xs font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
					>
						<IconifyIcon icon="material-symbols:chevron-left" class="h-4 w-4" />
						Previous
					</button>
					<button
						onclick={nextPage}
						disabled={!hasMore}
						class="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-md text-xs font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
					>
						Next
						<IconifyIcon icon="material-symbols:chevron-right" class="h-4 w-4" />
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
