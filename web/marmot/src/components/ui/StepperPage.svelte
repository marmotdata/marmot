<script lang="ts" module>
	export interface StepperPageStep {
		title: string;
		icon: string;
	}
</script>

<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Alert from './Alert.svelte';
	import Stepper from './Stepper.svelte';
	import Step from './Step.svelte';
	import type { Snippet } from 'svelte';
	import { m } from '$lib/paraglide/messages';
	import { BTN_GHOST, BTN_PRIMARY, BTN_SECONDARY, LABEL, PANEL } from '$lib/ui/classes';

	interface Props {
		title: string;
		steps: StepperPageStep[];
		currentStep: number;
		onBack?: () => void;
		onCancel?: () => void;
		onPrevious?: () => void;
		onNext?: () => void;
		onSave?: () => void;
		canProceed?: boolean;
		saving?: boolean;
		saveLabel?: string;
		savingLabel?: string;
		saveIcon?: string;
		error?: string | null;
		hideFooter?: boolean;
		canNavigateToStep?: (step: number) => boolean;
		onStepClick?: (step: number) => void;
		banner?: Snippet;
		// Set inside a layout that already pads the page, such as Settings, so the stepper does not add its own container
		embedded?: boolean;
		children: Snippet;
	}

	let {
		title,
		steps,
		currentStep,
		onBack,
		onCancel,
		onPrevious,
		onNext,
		onSave,
		canProceed = true,
		saving = false,
		saveLabel = m.common_save(),
		savingLabel = m.ui_stepper_saving(),
		saveIcon = 'material-symbols:check',
		error = null,
		hideFooter = false,
		canNavigateToStep,
		onStepClick,
		banner,
		embedded = false,
		children
	}: Props = $props();

	let totalSteps = $state(0);

	function handleBack() {
		if (onBack) onBack();
	}

	function handlePrevious() {
		if (onPrevious) onPrevious();
	}
</script>

<div class={embedded ? '' : 'container mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8'}>
	<header class="mb-6">
		{#if onBack}
			<button
				type="button"
				onclick={handleBack}
				class="mb-2 inline-flex items-center gap-1 text-sm text-gray-500 transition hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100"
			>
				<IconifyIcon icon="material-symbols:arrow-back" class="h-4 w-4" />
				{m.common_back()}
			</button>
		{/if}
		<p class={LABEL}>
			{m.ui_stepper_step_of({
				current: currentStep,
				total: steps.length,
				title: steps[currentStep - 1]?.title ?? ''
			})}
		</p>
		<h1 class="truncate text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
			{title}
		</h1>
	</header>

	<div class="{PANEL} mb-6 px-5 py-4">
		<Stepper {currentStep} bind:totalSteps {onStepClick} {canNavigateToStep}>
			{#each steps as step (step.title)}
				<Step title={step.title} icon={step.icon} />
			{/each}
		</Stepper>
	</div>

	{#if banner}
		{@render banner()}
	{/if}

	{#if error}
		<div class="mb-6"><Alert>{error}</Alert></div>
	{/if}

	{@render children()}

	{#if !hideFooter}
		<div
			class="mt-8 flex items-center justify-between border-t border-gray-200 pt-6 dark:border-gray-700"
		>
			<div>
				{#if currentStep > 1 && onPrevious}
					<button type="button" class={BTN_SECONDARY} onclick={handlePrevious}>
						<IconifyIcon icon="material-symbols:arrow-back" class="h-5 w-5" />
						{m.common_previous()}
					</button>
				{:else if onCancel}
					<button type="button" class={BTN_GHOST} onclick={onCancel}>{m.common_cancel()}</button>
				{/if}
			</div>
			<div class="flex items-center gap-3">
				{#if currentStep < steps.length && onNext}
					<button type="button" class={BTN_PRIMARY} onclick={onNext} disabled={!canProceed}>
						{m.common_next()}
						<IconifyIcon icon="material-symbols:arrow-forward" class="h-5 w-5" />
					</button>
				{:else if onSave}
					<button
						type="button"
						class={BTN_PRIMARY}
						onclick={onSave}
						disabled={saving || !canProceed}
					>
						<IconifyIcon icon={saveIcon} class="h-5 w-5" />
						{saving ? savingLabel : saveLabel}
					</button>
				{/if}
			</div>
		</div>
	{/if}
</div>
