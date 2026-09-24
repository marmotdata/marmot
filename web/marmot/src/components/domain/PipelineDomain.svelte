<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import {
		assignPipeline,
		domainsEnabled,
		errorMessage,
		loadTree,
		pipelineAssignment,
		type PipelineAssignment
	} from '$lib/domains/api';
	import { UNASSIGNED_DOMAIN_ID } from '$lib/domains/types';
	import { domainOptions, type DomainOption } from '$lib/domains/options';
	import Button from '$components/ui/Button.svelte';
	import DomainPicker from './DomainPicker.svelte';

	let {
		scheduleId,
		value = $bindable('')
	}: {
		/** Existing pipeline: changes apply at once. Without it, the parent assigns `value` after creating. */
		scheduleId?: string;
		value?: string;
	} = $props();

	let options = $state<DomainOption[] | null>(null);
	let assignment = $state<PipelineAssignment | null>(null);
	let pending = $state<string | null>(null);
	let busy = $state(false);

	const canChange = auth.hasPermission('ingestion', 'manage');
	const canMoveAssets = auth.hasPermission('assets', 'manage');
	const pathOf = (id: string) => options?.find((o) => o.id === id)?.path ?? '';

	$effect(() => {
		const id = scheduleId;
		let cancelled = false;
		domainsEnabled().then(async (enabled) => {
			if (!enabled || cancelled) return;
			try {
				const [forest, current] = await Promise.all([
					loadTree(),
					id ? pipelineAssignment(id) : Promise.resolve(null)
				]);
				if (cancelled) return;
				options = domainOptions(forest, { includeUnassigned: !!id });
				assignment = current;
			} catch {
				// Without the tree the pipeline keeps its current domain.
			}
		});
		return () => {
			cancelled = true;
		};
	});

	function requestChange(domainId: string) {
		if (!assignment || !scheduleId) return;
		if (assignment.assets_in_domain > 0) {
			pending = domainId;
		} else {
			apply(domainId, false);
		}
	}

	async function apply(domainId: string, moveAssets: boolean) {
		if (!scheduleId) return;
		busy = true;
		try {
			const result = await assignPipeline(scheduleId, domainId, moveAssets);
			toasts.success(
				result.moved_assets > 0
					? m.domains_pipeline_moved({ count: result.moved_assets })
					: m.domains_pipeline_changed()
			);
			pending = null;
			assignment = await pipelineAssignment(scheduleId);
		} catch (error) {
			toasts.error(errorMessage(error));
		} finally {
			busy = false;
		}
	}
</script>

{#if options && (!scheduleId || assignment)}
	<div class="mt-6">
		<label
			for="pipeline-domain"
			class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300"
		>
			{m.domains_pipeline_label()}
		</label>
		<p class="mb-2 text-xs text-gray-500 dark:text-gray-400">{m.domains_pipeline_hint()}</p>
		{#if scheduleId && assignment}
			<DomainPicker
				id="pipeline-domain"
				value={assignment.domain_id}
				{options}
				label={m.domains_pipeline_label()}
				disabled={!canChange || busy || pending !== null}
				onSelect={requestChange}
			/>
		{:else}
			<DomainPicker
				id="pipeline-domain"
				bind:value
				{options}
				label={m.domains_pipeline_label()}
				noneLabel={m.domains_unassigned()}
				disabled={!canChange}
			/>
		{/if}

		{#if pending !== null && assignment}
			<div
				class="mt-3 rounded-md border border-earthy-terracotta-200 bg-earthy-terracotta-50 p-4 text-sm text-gray-800 dark:border-earthy-terracotta-800 dark:bg-earthy-terracotta-900/20 dark:text-gray-200"
				role="alertdialog"
				aria-labelledby="pipeline-domain-question"
			>
				<p id="pipeline-domain-question">
					{m.domains_pipeline_move_question({
						count: assignment.assets_in_domain,
						from: pathOf(assignment.domain_id || UNASSIGNED_DOMAIN_ID),
						to: pathOf(pending)
					})}
				</p>
				<div class="mt-3 flex flex-wrap gap-2">
					{#if canMoveAssets}
						<Button
							text={m.domains_pipeline_move_all({ count: assignment.assets_in_domain })}
							loading={busy}
							disabled={busy}
							click={() => pending !== null && apply(pending, true)}
						/>
					{/if}
					<Button
						variant="clear"
						text={m.domains_pipeline_move_only()}
						disabled={busy}
						click={() => pending !== null && apply(pending, false)}
					/>
					<Button
						variant="clear"
						text={m.domains_cancel()}
						disabled={busy}
						click={() => (pending = null)}
					/>
				</div>
			</div>
		{/if}
	</div>
{/if}
