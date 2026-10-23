<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import AgentClientSnippet from './AgentClientSnippet.svelte';
	import HeadlessAgentHint from './HeadlessAgentHint.svelte';
	import { skippedSteps } from '$lib/stores/setup.svelte';

	export type SetupStep = 'source' | 'agent' | 'query' | 'context' | 'team';

	interface Props {
		done: Record<SetupStep, boolean>;
		selected?: SetupStep | null;
		onDismiss?: () => void;
		onRemove?: () => void;
		canManageIngestion: boolean;
		canQuery: boolean;
		isAdmin: boolean;
	}

	let {
		done,
		selected = $bindable(null),
		onDismiss,
		onRemove,
		canManageIngestion,
		canQuery,
		isAdmin
	}: Props = $props();

	// Each step shows one thing Marmot does; none is mandatory, so any can be skipped and each is marked by its icon rather than a number
	const ORDER: SetupStep[] = ['source', 'agent', 'query', 'context', 'team'];

	interface StepView {
		id: SetupStep;
		title: string;
		body?: string;
		waiting: string;
		icon: string;
	}

	const STEPS: Record<SetupStep, StepView> = {
		source: {
			id: 'source',
			title: m.home_step_source_title(),
			waiting: m.home_step_source_waiting(),
			icon: 'material-symbols:database-outline'
		},
		agent: {
			id: 'agent',
			title: m.home_step_agent_title(),
			body: m.home_step_agent_body(),
			waiting: m.home_step_agent_waiting(),
			icon: 'material-symbols:smart-toy-outline'
		},
		query: {
			id: 'query',
			title: m.home_step_query_title(),
			body: m.home_step_query_body(),
			waiting: m.home_step_query_waiting(),
			icon: 'material-symbols:terminal'
		},
		context: {
			id: 'context',
			title: m.home_step_context_title(),
			body: m.home_step_context_body(),
			waiting: m.home_step_context_waiting(),
			icon: 'material-symbols:menu-book-outline'
		},
		team: {
			id: 'team',
			title: m.home_step_team_title(),
			body: m.home_step_team_body(),
			waiting: m.home_step_team_waiting(),
			icon: 'material-symbols:group-add-outline'
		}
	};

	const steps = ORDER.map((id) => STEPS[id]);
	let settled = (id: SetupStep) => done[id] || skippedSteps.steps.includes(id);
	let completed = $derived(steps.filter((s) => settled(s.id)).length);
	let firstOpen = $derived(steps.find((s) => !settled(s.id))?.id ?? null);
	// The parent can pick a step too, so the homepage's own buttons open the matching instructions here
	let current = $derived(selected ?? firstOpen);

	const PRIMARY =
		'inline-flex items-center gap-1.5 rounded-lg bg-earthy-terracotta-600 px-3.5 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500';
	// Every way to populate Marmot besides the UI, each pointing at its guide
	const DOCS = 'https://marmotdata.io/docs';
	const codeRoutes = [
		{ label: 'CLI', icon: 'material-symbols:terminal', href: `${DOCS}/Populating/CLI` },
		{ label: 'API', icon: 'material-symbols:api', href: `${DOCS}/Populating/API` },
		{ label: 'SDK', icon: 'material-symbols:package-2-outline', href: `${DOCS}/SDK` },
		{ label: 'Terraform', icon: 'logos:terraform-icon', href: `${DOCS}/Populating/Terraform` },
		{ label: 'Pulumi', icon: 'logos:pulumi-icon', href: `${DOCS}/Populating/Pulumi` }
	];

	const LINK =
		'inline-flex items-center gap-1 text-sm font-medium text-gray-600 hover:text-gray-900 hover:underline dark:text-gray-400 dark:hover:text-gray-100';
</script>

<!-- A vertical checklist: only the current step is open, with one line and one action, and the rest are just titles you can click -->
<section id="setup" class="scroll-mt-20" aria-labelledby="setup-heading">
	<div class="flex items-center justify-between gap-4">
		<h2
			id="setup-heading"
			class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
		>
			{m.home_setup_heading()}
			<span class="ml-1.5 font-normal normal-case tracking-normal tabular-nums"
				>{m.home_setup_progress({ done: completed, total: steps.length })}</span
			>
		</h2>
		<div class="flex items-center gap-1.5">
			{#if onRemove}
				<button
					type="button"
					onclick={onRemove}
					class="inline-flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium text-red-600 transition hover:bg-red-50 hover:text-red-700 dark:text-red-400 dark:hover:bg-red-900/20 dark:hover:text-red-300"
				>
					<IconifyIcon icon="material-symbols:block" class="h-3.5 w-3.5" />
					{m.home_setup_remove()}
				</button>
			{/if}
			{#if onDismiss}
				<button
					type="button"
					onclick={onDismiss}
					class="inline-flex items-center gap-1 rounded-md border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-600 transition hover:border-gray-300 hover:text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:text-gray-100"
				>
					<IconifyIcon icon="material-symbols:visibility-off-outline" class="h-3.5 w-3.5" />
					{m.home_setup_hide()}
				</button>
			{/if}
		</div>
	</div>

	{#if completed === steps.length}
		<p class="mt-4 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
			<IconifyIcon icon="material-symbols:check-circle" class="h-5 w-5 text-earthy-green-600" />
			{m.home_setup_done_title()}
		</p>
	{:else}
		<ol class="mt-4">
			{#each steps as s, i (s.id)}
				{@const isDone = done[s.id]}
				{@const isSkipped = !isDone && skippedSteps.steps.includes(s.id)}
				{@const open = current === s.id}
				<li class="relative pb-4 pl-10 last:pb-0">
					{#if i < steps.length - 1}
						<span
							class="absolute bottom-0 left-[13px] top-8 w-px bg-gray-200 dark:bg-gray-700"
							aria-hidden="true"
						></span>
					{/if}
					<button
						type="button"
						onclick={() => (selected = s.id)}
						aria-expanded={open}
						class="group -ml-10 flex w-[calc(100%+2.5rem)] items-center gap-3 text-left"
					>
						<span
							class="relative z-10 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full text-xs font-semibold {isDone
								? 'bg-earthy-green-500 text-white'
								: open
									? 'bg-earthy-terracotta-600 text-white'
									: 'border border-gray-300 bg-white text-gray-500 group-hover:border-gray-400 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-400'}"
						>
							{#if isDone}
								<IconifyIcon icon="material-symbols:check" class="h-4 w-4" />
							{:else if isSkipped}
								<IconifyIcon icon="material-symbols:remove" class="h-4 w-4" />
							{:else}
								<IconifyIcon icon={s.icon} class="h-4 w-4" />
							{/if}
						</span>
						<span
							class="text-sm {open
								? 'font-semibold text-gray-900 dark:text-gray-100'
								: isDone
									? 'text-gray-400 line-through dark:text-gray-500'
									: isSkipped
										? 'text-gray-400 dark:text-gray-500'
										: 'font-medium text-gray-600 group-hover:text-gray-900 dark:text-gray-400 dark:group-hover:text-gray-100'}"
							>{s.title}</span
						>
						{#if isSkipped}
							<span class="text-xs text-gray-400">{m.home_setup_skipped()}</span>
						{/if}
						{#if open && !isDone && !isSkipped}
							<span class="relative flex h-2 w-2" title={s.waiting}>
								<span
									class="absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-400 opacity-60 motion-reduce:animate-none"
								></span>
								<span class="relative inline-flex h-2 w-2 rounded-full bg-amber-500"></span>
							</span>
						{/if}
					</button>

					{#if open}
						<div class="mt-2 space-y-3">
							{#if s.body}
								<p class="text-sm text-gray-600 dark:text-gray-400">{s.body}</p>
							{/if}
							{#if s.id === 'agent'}
								<AgentClientSnippet />
							{/if}
							<div class="flex flex-wrap items-center gap-x-4 gap-y-2">
								{#if s.id === 'source'}
									{#if canManageIngestion}
										<a href={resolve('/pipelines/new')} class={PRIMARY}>
											<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
											{m.home_step_source_action()}
										</a>
										<span class="flex flex-wrap items-center gap-1.5">
											<span class="text-sm text-gray-500 dark:text-gray-400"
												>{m.home_step_source_code()}</span
											>
											{#each codeRoutes as route (route.label)}
												<a
													href={route.href}
													target="_blank"
													rel="external noopener noreferrer"
													class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 shadow-sm transition hover:-translate-y-px hover:border-gray-300 hover:shadow dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
												>
													<IconifyIcon
														icon={route.icon}
														class="h-4 w-4 {route.icon.startsWith('logos:')
															? ''
															: 'text-earthy-terracotta-600 dark:text-earthy-terracotta-400'}"
													/>
													{route.label}
												</a>
											{/each}
										</span>
									{:else}
										<span class="text-sm text-amber-700 dark:text-amber-300"
											>{m.home_needs_admin()}</span
										>
									{/if}
								{:else if s.id === 'agent'}
									{#if isAdmin}
										<HeadlessAgentHint />
									{/if}
								{:else if s.id === 'query'}
									{#if canQuery}
										<a href={resolve('/query')} class={PRIMARY}>
											<IconifyIcon icon="material-symbols:play-arrow-rounded" class="h-4 w-4" />
											{m.home_step_query_action()}
										</a>
									{:else}
										<span class="text-sm text-amber-700 dark:text-amber-300"
											>{m.home_needs_query()}</span
										>
									{/if}
								{:else if s.id === 'context'}
									<a href={resolve('/glossary')} class={PRIMARY}>
										<IconifyIcon icon="material-symbols:book-outline" class="h-4 w-4" />
										{m.home_step_context_glossary()}
									</a>
									<a href={resolve('/products')} class={LINK}>
										{m.home_step_context_product()}
									</a>
								{:else if s.id === 'team'}
									{#if isAdmin}
										<a href={resolve('/settings/members')} class={PRIMARY}>
											<IconifyIcon icon="material-symbols:group-add-outline" class="h-4 w-4" />
											{m.home_step_team_action()}
										</a>
									{:else}
										<span class="text-sm text-amber-700 dark:text-amber-300"
											>{m.home_needs_admin()}</span
										>
									{/if}
								{/if}
								{#if !isDone}
									<button
										type="button"
										onclick={() => {
											skippedSteps.toggle(s.id);
											selected = null;
										}}
										class="text-sm font-medium text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
									>
										{isSkipped ? m.home_setup_unskip() : m.home_setup_skip()}
									</button>
								{/if}
							</div>
						</div>
					{/if}
				</li>
			{/each}
		</ol>
	{/if}
</section>
