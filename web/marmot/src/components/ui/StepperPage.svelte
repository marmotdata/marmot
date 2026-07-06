<script lang="ts" module>
	export interface StepperPageStep {
		title: string;
		icon: string;
	}
</script>

<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import Button from './Button.svelte';
	import Stepper from './Stepper.svelte';
	import Step from './Step.svelte';
	import type { Snippet } from 'svelte';

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
		saveLabel = 'Save',
		savingLabel = 'Saving...',
		saveIcon = 'material-symbols:check',
		error = null,
		hideFooter = false,
		canNavigateToStep,
		onStepClick,
		banner,
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

<div class="min-h-screen">
	<!-- Header -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			<div class="flex items-center gap-4">
				{#if onBack}
					<button
						type="button"
						onclick={handleBack}
						class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
						aria-label="Back"
					>
						<IconifyIcon
							icon="material-symbols:arrow-back"
							class="h-6 w-6 text-gray-600 dark:text-gray-400"
						/>
					</button>
				{/if}
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">{title}</h1>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Step {currentStep} of {steps.length} — {steps[currentStep - 1]?.title}
					</p>
				</div>
			</div>
		</div>
	</div>

	<!-- Step Indicator -->
	<div class="border-b border-gray-200 dark:border-gray-700">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
			<Stepper {currentStep} bind:totalSteps {onStepClick} {canNavigateToStep}>
				{#each steps as step (step.title)}
					<Step title={step.title} icon={step.icon} />
				{/each}
			</Stepper>
		</div>
	</div>

	<!-- Main Content -->
	<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
		{#if banner}
			{@render banner()}
		{/if}

		{#if error}
			<div
				class="mb-6 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4"
			>
				<div class="flex items-start">
					<IconifyIcon
						icon="material-symbols:error"
						class="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0"
					/>
					<p class="ml-3 text-sm text-red-700 dark:text-red-300">{error}</p>
				</div>
			</div>
		{/if}

		{@render children()}

		{#if !hideFooter}
			<div
				class="mt-8 flex items-center justify-between border-t border-gray-200 dark:border-gray-700 pt-6"
			>
				<div>
					{#if currentStep > 1 && onPrevious}
						<Button
							variant="clear"
							click={handlePrevious}
							icon="material-symbols:arrow-back"
							text="Previous"
						/>
					{:else if onCancel}
						<Button variant="clear" click={onCancel} text="Cancel" />
					{/if}
				</div>
				<div class="flex items-center gap-3">
					{#if currentStep < steps.length && onNext}
						<Button
							variant="filled"
							click={onNext}
							text="Next"
							icon="material-symbols:arrow-forward"
							disabled={!canProceed}
						/>
					{:else if onSave}
						<Button
							variant="filled"
							click={onSave}
							text={saving ? savingLabel : saveLabel}
							disabled={saving || !canProceed}
							icon={saveIcon}
						/>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</div>
