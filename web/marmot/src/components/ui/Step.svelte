<script lang="ts">
	import { getContext, onMount } from 'svelte';
	import type { StepConfig } from './Stepper.svelte';

	interface Props {
		title: string;
		icon: string;
	}

	let { title, icon }: Props = $props();

	const stepper = getContext<{
		registerStep: (config: StepConfig) => number;
		currentStep: number;
	}>('stepper');

	onMount(() => {
		stepper?.registerStep({ title, icon });
	});
</script>
