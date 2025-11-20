<script lang="ts">
	import Button from './Button.svelte';
	import IconifyIcon from '@iconify/svelte';

	interface Props {
		title?: string;
		description?: string;
		primaryButtonText?: string;
		primaryButtonIcon?: string;
		primaryButtonUrl?: string;
		secondaryButtonText?: string;
		secondaryButtonIcon?: string;
		secondaryButtonUrl?: string;
		showSteps?: boolean;
		condensed?: boolean;
	}

	let {
		title = 'Get Started with Marmot',
		description = 'The quickest way to start populating Marmot is using the CLI. Configure your data sources and run ingestion jobs to populate your data catalog.',
		primaryButtonText = 'View Documentation',
		primaryButtonIcon = 'material-symbols:book',
		primaryButtonUrl = 'https://marmotdata.io/docs/introduction',
		secondaryButtonText = 'Plugins',
		secondaryButtonIcon = 'material-symbols:extension',
		secondaryButtonUrl = 'https://marmotdata.io/docs/Plugins/',
		showSteps = true,
		condensed = false
	}: Props = $props();
</script>

<div
	class="bg-gradient-to-br from-earthy-terracotta-50 to-earthy-yellow-50 dark:from-earthy-terracotta-900/20 dark:to-earthy-yellow-900/20 border border-earthy-terracotta-200 dark:border-earthy-terracotta-800/50 rounded-lg {condensed
		? 'p-6'
		: 'p-8'}"
>
	<div class="text-center">
		<div
			class="mx-auto {condensed
				? 'w-12 h-12 mb-4'
				: 'w-16 h-16 mb-6'} bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 rounded-full flex items-center justify-center"
		>
			<IconifyIcon
				icon="material-symbols:rocket-launch"
				class="{condensed ? 'h-6 w-6' : 'h-8 w-8'} text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
			/>
		</div>

		<h3
			class="{condensed
				? 'text-lg'
				: 'text-xl'} font-semibold text-gray-900 dark:text-gray-100 {condensed ? 'mb-3' : 'mb-4'}"
		>
			{title}
		</h3>

		<p
			class="text-gray-600 dark:text-gray-400 {condensed ? 'mb-4 text-sm' : 'mb-6'} {condensed
				? 'max-w-full'
				: 'max-w-2xl'} mx-auto"
		>
			{description}
		</p>

		{#if showSteps && !condensed}
			<div class="grid md:grid-cols-2 gap-6 {condensed ? 'max-w-full' : 'max-w-4xl'} mx-auto mb-8">
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6"
				>
					<div class="flex items-center mb-4">
						<div
							class="w-8 h-8 bg-blue-100 dark:bg-blue-900/30 rounded-full flex items-center justify-center mr-3"
						>
							<span class="text-blue-600 dark:text-blue-400 font-semibold text-sm">1</span>
						</div>
						<h4 class="font-semibold text-gray-900 dark:text-gray-100">Install CLI</h4>
					</div>
					<div class="bg-gray-900 dark:bg-gray-950 rounded-md p-3 mb-4">
						<code class="text-green-400 text-sm">curl -fsSL get.marmotdata.io | sh</code>
					</div>
					<p class="text-sm text-gray-600 dark:text-gray-400">
						Download and install the Marmot CLI.
					</p>
				</div>

				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6"
				>
					<div class="flex items-center mb-4">
						<div
							class="w-8 h-8 bg-green-100 dark:bg-green-900/30 rounded-full flex items-center justify-center mr-3"
						>
							<span class="text-green-600 dark:text-green-400 font-semibold text-sm">2</span>
						</div>
						<h4 class="font-semibold text-gray-900 dark:text-gray-100">Configure Sources</h4>
					</div>
					<div class="bg-gray-900 dark:bg-gray-950 rounded-md p-3 mb-4">
						<code class="text-green-400 text-sm">marmot ingest --config pipeline.yaml</code>
					</div>
					<p class="text-sm text-gray-600 dark:text-gray-400">
						Create configuration files for your data sources and run ingestion jobs.
					</p>
				</div>
			</div>
		{/if}

		<div class="flex flex-col sm:flex-row gap-{condensed ? '3' : '4'} justify-center">
			<Button
				variant="filled"
				text={primaryButtonText}
				icon={primaryButtonIcon}
				click={() => window.open(primaryButtonUrl, '_blank')}
			/>
			<Button
				variant="clear"
				text={secondaryButtonText}
				icon={secondaryButtonIcon}
				click={() => window.open(secondaryButtonUrl, '_blank')}
			/>
		</div>
	</div>
</div>
