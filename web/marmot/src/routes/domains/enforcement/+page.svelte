<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import Icon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { DomainRef, EnforcementPlan, EnforcementState } from '$lib/domains/types';
	import {
		DomainError,
		enforcementPlan,
		enforcementState,
		errorMessage,
		setEnforcement
	} from '$lib/domains/api';

	let state = $state<EnforcementState | null>(null);
	let plan = $state<EnforcementPlan | null>(null);
	let forbidden = $state(false);
	let loading = $state(true);
	let confirming = $state(false);
	let busy = $state(false);

	async function load() {
		loading = true;
		try {
			state = await enforcementState();
			plan = await enforcementPlan();
			forbidden = false;
		} catch (error) {
			if (error instanceof DomainError && error.code === 'forbidden') {
				forbidden = true;
			} else {
				toasts.error(errorMessage(error));
			}
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function apply() {
		if (!state || !plan) return;
		const on = !state.write;
		busy = true;
		try {
			state = await setEnforcement(on, on ? plan.hash : '');
			toasts.success(on ? m.domains_enforcement_enabled() : m.domains_enforcement_disabled());
			confirming = false;
			await load();
		} catch (error) {
			toasts.error(errorMessage(error));
			if (error instanceof DomainError && error.code === 'plan_changed') {
				confirming = false;
				await load();
			}
		} finally {
			busy = false;
		}
	}

	const paths = (refs: DomainRef[]) => refs.map((r) => r.path).join(', ') || '—';
	const subjectLabel = (type: string) =>
		type === 'service_account'
			? m.domains_enforcement_type_service_account()
			: m.domains_enforcement_type_user();
	const cell = 'px-4 py-2.5 text-sm text-gray-700 dark:text-gray-300 align-top';
	const head =
		'px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400';
</script>

<div class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
	<a
		href={resolve('/domains/[[id]]', { id: undefined })}
		class="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
	>
		<Icon icon="material-symbols:arrow-back" class="h-4 w-4" />
		{m.domains_title()}
	</a>
	<div class="mb-6">
		<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
			{m.domains_enforcement_title()}
		</h1>
		<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{m.domains_enforcement_subtitle()}</p>
	</div>

	{#if loading && !state}
		<div class="flex justify-center py-16">
			<div class="h-8 w-8 animate-spin rounded-full border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if state}
		<section
			class="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-lg border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800"
		>
			<div class="flex items-center gap-3">
				<Icon
					icon={state.write
						? 'material-symbols:lock-outline'
						: 'material-symbols:lock-open-outline'}
					class="h-6 w-6 {state.write ? 'text-earthy-terracotta-700' : 'text-gray-400'}"
				/>
				<div>
					<p class="text-sm font-medium text-gray-900 dark:text-gray-100">
						{m.domains_enforcement_state()}:
						<span
							class="ml-1 rounded-full px-2.5 py-0.5 text-xs font-medium {state.write
								? 'bg-earthy-terracotta-100 text-earthy-terracotta-800 dark:bg-earthy-terracotta-900/40 dark:text-earthy-terracotta-100'
								: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'}"
						>
							{state.write ? m.domains_enforcement_on() : m.domains_enforcement_off()}
						</span>
					</p>
					{#if state.updated_by && state.updated_at}
						<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
							{m.domains_enforcement_changed({
								by: state.updated_by,
								at: new Date(state.updated_at).toLocaleString()
							})}
						</p>
					{/if}
				</div>
			</div>
			{#if plan && !confirming}
				<Button
					icon={state.write
						? 'material-symbols:lock-open-outline'
						: 'material-symbols:lock-outline'}
					text={state.write ? m.domains_enforcement_disable() : m.domains_enforcement_enable()}
					variant={state.write ? 'clear' : 'filled'}
					click={() => (confirming = true)}
				/>
			{/if}
		</section>

		{#if confirming && plan}
			<div
				role="alertdialog"
				aria-labelledby="enforcement-confirm"
				class="mb-6 rounded-lg border border-earthy-terracotta-300 bg-earthy-terracotta-50 p-5 dark:border-earthy-terracotta-800 dark:bg-earthy-terracotta-900/20"
			>
				<p id="enforcement-confirm" class="text-sm text-gray-900 dark:text-gray-100">
					{state.write
						? m.domains_enforcement_confirm_disable()
						: m.domains_enforcement_confirm_enable({ hash: plan.hash })}
				</p>
				<div class="mt-4 flex gap-2">
					<Button text={m.common_confirm()} loading={busy} disabled={busy} click={apply} />
					<Button
						variant="clear"
						text={m.common_cancel()}
						disabled={busy}
						click={() => (confirming = false)}
					/>
				</div>
			</div>
		{/if}

		{#if forbidden}
			<p class="text-sm text-gray-600 dark:text-gray-400">{m.domains_enforcement_global_only()}</p>
		{:else if plan}
			<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
				<p class="font-mono text-xs text-gray-500 dark:text-gray-400">
					{m.domains_enforcement_plan_hash({ hash: plan.hash })}
				</p>
				<Button
					variant="clear"
					icon="material-symbols:refresh"
					text={m.domains_enforcement_refresh()}
					{loading}
					click={load}
				/>
			</div>

			<section class="mb-8" aria-labelledby="enforcement-principals">
				<h2
					id="enforcement-principals"
					class="text-base font-semibold text-gray-900 dark:text-gray-100"
				>
					{m.domains_enforcement_principals_heading()}
				</h2>
				<p class="mb-3 mt-1 text-sm text-gray-500 dark:text-gray-400">
					{m.domains_enforcement_principals_hint()}
				</p>
				{#if plan.principals.length === 0}
					<p class="text-sm italic text-gray-500 dark:text-gray-400">
						{m.domains_enforcement_principals_empty()}
					</p>
				{:else}
					<div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
						<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
							<thead class="bg-gray-50 dark:bg-gray-800">
								<tr>
									<th class={head}>{m.domains_enforcement_col_identity()}</th>
									<th class={head}>{m.domains_enforcement_col_permissions()}</th>
									<th class={head}>{m.domains_enforcement_col_keeps()}</th>
									<th class={head}>{m.domains_enforcement_col_loses()}</th>
								</tr>
							</thead>
							<tbody
								class="divide-y divide-gray-100 bg-white dark:divide-gray-700 dark:bg-gray-900"
							>
								{#each plan.principals as p (p.subject_type + p.subject_id)}
									<tr>
										<td class={cell}>
											<span class="font-medium text-gray-900 dark:text-gray-100">{p.name}</span>
											<span class="block text-xs text-gray-500">{subjectLabel(p.subject_type)}</span
											>
										</td>
										<td class="{cell} font-mono text-xs">{p.permissions.join(', ')}</td>
										<td class={cell}>{paths(p.keeps)}</td>
										<td class="{cell} text-red-700 dark:text-red-400">{paths(p.loses)}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</section>

			{#if plan.pipelines.length > 0}
				<section aria-labelledby="enforcement-pipelines">
					<h2
						id="enforcement-pipelines"
						class="text-base font-semibold text-gray-900 dark:text-gray-100"
					>
						{m.domains_enforcement_pipelines_heading()}
					</h2>
					<p class="mb-3 mt-1 text-sm text-gray-500 dark:text-gray-400">
						{m.domains_enforcement_pipelines_hint()}
					</p>
					<div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
						<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
							<thead class="bg-gray-50 dark:bg-gray-800">
								<tr>
									<th class={head}>{m.domains_enforcement_col_pipeline()}</th>
									<th class={head}>{m.domains_enforcement_col_domain()}</th>
									<th class={head}>{m.domains_enforcement_col_outside()}</th>
								</tr>
							</thead>
							<tbody
								class="divide-y divide-gray-100 bg-white dark:divide-gray-700 dark:bg-gray-900"
							>
								{#each plan.pipelines as pl (pl.schedule_id)}
									<tr>
										<td class="{cell} font-medium text-gray-900 dark:text-gray-100">{pl.name}</td>
										<td class={cell}>{pl.domain.path}</td>
										<td class={cell}>{pl.assets_outside}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</section>
			{/if}
		{/if}
	{/if}
</div>
