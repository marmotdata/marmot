<script lang="ts" module>
	export type SetupStep = 'source' | 'agent' | 'query' | 'context' | 'team';
</script>

<script lang="ts">
	import '@marmotdata/elements/progress.js';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { skippedSteps } from '$lib/stores/setup.svelte';

	interface Props {
		done: Record<SetupStep, boolean>;
		onConnectAgent: () => void;
		onHide: () => void;
		onRemove: () => void;
		canManageIngestion: boolean;
		canQuery: boolean;
		isAdmin: boolean;
	}

	let { done, onConnectAgent, onHide, onRemove, canManageIngestion, canQuery, isAdmin }: Props =
		$props();

	interface StepView {
		id: SetupStep;
		title: string;
		todo: string;
		done: string;
		icon: string;
		action: string;
		// A route, or undefined when the step opens the connect drawer or the viewer may not do it
		href?: string;
		allowed: boolean;
	}

	let steps = $derived<StepView[]>([
		{
			id: 'source',
			title: m.home_step_source_title(),
			todo: m.home_step_source_todo(),
			done: m.home_step_source_done(),
			icon: 'database',
			action: m.home_step_source_action(),
			href: resolve('/pipelines/new'),
			allowed: canManageIngestion
		},
		{
			id: 'agent',
			title: m.home_step_agent_title(),
			todo: m.home_step_agent_todo(),
			done: m.home_step_agent_done(),
			icon: 'smart-toy',
			action: m.home_step_agent_action(),
			allowed: true
		},
		{
			id: 'query',
			title: m.home_step_query_title(),
			todo: m.home_step_query_todo(),
			done: m.home_step_query_done(),
			icon: 'terminal',
			action: m.home_step_query_action(),
			href: resolve('/query'),
			allowed: canQuery
		},
		{
			id: 'context',
			title: m.home_step_context_title(),
			todo: m.home_step_context_todo(),
			done: m.home_step_context_done(),
			icon: 'menu-book',
			action: m.home_step_context_glossary(),
			href: resolve('/glossary'),
			allowed: true
		},
		{
			id: 'team',
			title: m.home_step_team_title(),
			todo: m.home_step_team_todo(),
			done: m.home_step_team_done(),
			icon: 'person-add',
			action: m.home_step_team_action(),
			href: resolve('/settings/members'),
			allowed: isAdmin
		}
	]);

	// Skipped optional steps count as settled so the checklist stops asking about things the team does not use
	let settled = (id: SetupStep) => done[id] || skippedSteps.steps.includes(id);
	let completed = $derived(steps.filter((s) => settled(s.id)).length);
	let next = $derived(steps.find((s) => !settled(s.id))?.id ?? null);

	function onPanelMenu(e: CustomEvent<{ value: string }>) {
		if (e.detail.value === 'remove') onRemove();
	}
</script>

<mm-panel heading={m.home_setup_heading()} icon="checklist">
	<mm-button slot="actions" variant="ghost" size="sm" onclick={onHide}
		>{m.home_setup_hide()}</mm-button
	>
	<mm-menu-button
		slot="actions"
		variant="ghost"
		size="sm"
		icon="more-horiz"
		icon-only
		label={m.home_setup_more()}
	>
		<mm-menu onmm-select={onPanelMenu}>
			<mm-menu-item value="remove" icon="visibility-off">{m.home_setup_remove()}</mm-menu-item>
		</mm-menu>
	</mm-menu-button>
	<div class="px-space-4 pb-space-2 pt-space-3">
		<mm-progress
			label={m.home_setup_progress_label()}
			value={completed}
			max={steps.length}
			unit={m.home_setup_progress_unit()}
		></mm-progress>
	</div>
	<mm-list>
		{#each steps as s (s.id)}
			{@const isDone = done[s.id]}
			{@const isSkipped = !isDone && skippedSteps.steps.includes(s.id)}
			<mm-list-item
				secondary={isDone
					? s.done
					: isSkipped
						? m.home_setup_skipped()
						: s.allowed
							? s.todo
							: s.id === 'query'
								? m.home_needs_query()
								: m.home_needs_admin()}
			>
				<mm-icon
					slot="start"
					name={isDone ? 'check-circle' : s.icon}
					size="20"
					label={isDone ? m.home_setup_done() : m.home_setup_todo()}
				></mm-icon>
				{s.title}
				{#if isDone}
					<mm-text slot="end" variant="caption" tone="secondary">{m.home_setup_done()}</mm-text>
				{:else}
					<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-1); flex-wrap: nowrap">
						{#if s.allowed && !isSkipped}
							{#if s.href}
								<mm-button size="sm" variant={s.id === next ? 'primary' : 'secondary'} href={s.href}
									>{s.action}</mm-button
								>
							{:else}
								<mm-button
									size="sm"
									variant={s.id === next ? 'primary' : 'secondary'}
									onclick={onConnectAgent}>{s.action}</mm-button
								>
							{/if}
						{/if}
						<mm-menu-button
							variant="ghost"
							size="sm"
							icon="more-horiz"
							icon-only
							label={m.home_setup_step_more({ step: s.title })}
						>
							<mm-menu onmm-select={() => skippedSteps.toggle(s.id)}>
								<mm-menu-item value="skip"
									>{isSkipped ? m.home_setup_unskip() : m.home_setup_skip()}</mm-menu-item
								>
							</mm-menu>
						</mm-menu-button>
					</span>
				{/if}
			</mm-list-item>
		{/each}
	</mm-list>
</mm-panel>
