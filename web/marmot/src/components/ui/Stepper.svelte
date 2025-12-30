<script lang="ts" module>
	export interface StepConfig {
		title: string;
		icon: string;
	}
</script>

<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import type { Snippet } from 'svelte';
	import { setContext } from 'svelte';

	interface Props {
		currentStep: number;
		totalSteps?: number;
		onStepClick?: (stepNumber: number) => void;
		canNavigateToStep?: (stepNumber: number) => boolean;
		children: Snippet;
	}

	let {
		currentStep,
		totalSteps = $bindable(0),
		onStepClick,
		canNavigateToStep,
		children
	}: Props = $props();

	// Steps registered by child Step components
	let steps = $state<StepConfig[]>([]);

	function registerStep(config: StepConfig): number {
		const stepNumber = steps.length + 1;
		steps = [...steps, config];
		totalSteps = steps.length;
		return stepNumber;
	}

	function handleStepClick(stepNumber: number) {
		if (!onStepClick) return;

		const canNavigate =
			stepNumber < currentStep || (canNavigateToStep ? canNavigateToStep(stepNumber) : false);

		if (canNavigate) {
			onStepClick(stepNumber);
		}
	}

	// Provide context for child Step components
	setContext('stepper', { registerStep });
</script>

<!-- Hidden container to render Step children and register them -->
<div class="hidden">
	{@render children()}
</div>

<!-- Stepper UI -->
<div class="flex items-center justify-between">
	{#each steps as step, index}
		{@const stepNumber = index + 1}
		<div class="flex items-center {index < steps.length - 1 ? 'flex-1' : ''}">
			<button
				onclick={() => handleStepClick(stepNumber)}
				class="flex items-center gap-3 {currentStep === stepNumber
					? ''
					: 'opacity-60 hover:opacity-80'} transition-opacity"
			>
				<div
					class="flex items-center justify-center w-10 h-10 rounded-full {currentStep === stepNumber
						? 'bg-earthy-terracotta-600 text-white'
						: currentStep > stepNumber
							? 'bg-green-600 text-white'
							: 'bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400'}"
				>
					{#if currentStep > stepNumber}
						<IconifyIcon icon="material-symbols:check" class="h-5 w-5" />
					{:else}
						<IconifyIcon icon={step.icon} class="h-5 w-5" />
					{/if}
				</div>
				<span class="text-sm font-medium text-gray-900 dark:text-gray-100 hidden sm:block">
					{step.title}
				</span>
			</button>
			{#if index < steps.length - 1}
				<div
					class="flex-1 h-0.5 mx-4 {currentStep > stepNumber
						? 'bg-green-600'
						: 'bg-gray-200 dark:bg-gray-700'}"
				></div>
			{/if}
		</div>
	{/each}
</div>
