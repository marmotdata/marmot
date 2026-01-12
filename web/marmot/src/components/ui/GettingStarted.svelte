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
		title = 'Start Populating Your Catalog',
		description = 'Connect to your data sources and discover assets. Pick the method that works best for your workflow.',
		primaryButtonText = 'Populating Your Catalog',
		primaryButtonIcon = 'material-symbols:book',
		primaryButtonUrl = 'https://marmotdata.io/docs/Populating/',
		secondaryButtonText = 'Plugins',
		secondaryButtonIcon = 'material-symbols:extension',
		secondaryButtonUrl = 'https://marmotdata.io/docs/Plugins/',
		showSteps = true,
		condensed = false
	}: Props = $props();

	const methods = [
		{
			icon: 'material-symbols:web',
			title: 'UI',
			description: 'Run discovery jobs directly from the interface',
			url: 'https://marmotdata.io/docs/Populating/UI'
		},
		{
			icon: 'material-symbols:terminal',
			title: 'CLI',
			description: 'YAML config for CI/CD pipelines',
			url: 'https://marmotdata.io/docs/Populating/CLI'
		},
		{
			icon: 'material-symbols:code',
			title: 'Terraform / Pulumi',
			description: 'Infrastructure as code',
			url: 'https://marmotdata.io/docs/Populating/Terraform'
		},
		{
			icon: 'material-symbols:api',
			title: 'REST API',
			description: 'Custom integrations',
			url: 'https://marmotdata.io/docs/Populating/API'
		}
	];
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
				class="{condensed
					? 'h-6 w-6'
					: 'h-8 w-8'} text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
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
			class="text-gray-600 dark:text-gray-400 {condensed ? 'mb-4 text-sm' : 'mb-8'} {condensed
				? 'max-w-full'
				: 'max-w-2xl'} mx-auto"
		>
			{description}
		</p>

		{#if showSteps && !condensed}
			<div class="grid grid-cols-2 md:grid-cols-4 gap-4 max-w-4xl mx-auto mb-8">
				{#each methods as method}
					<a
						href={method.url}
						target="_blank"
						rel="noopener noreferrer"
						class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5 hover:border-earthy-terracotta-500 dark:hover:border-earthy-terracotta-600 hover:shadow-md transition-all group text-center"
					>
						<div
							class="w-12 h-12 bg-gray-100 dark:bg-gray-700 rounded-full flex items-center justify-center mb-3 mx-auto group-hover:bg-earthy-terracotta-100 dark:group-hover:bg-earthy-terracotta-900/30 transition-colors"
						>
							<IconifyIcon
								icon={method.icon}
								class="h-6 w-6 text-gray-600 dark:text-gray-400 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
							/>
						</div>
						<h4
							class="font-semibold text-gray-900 dark:text-gray-100 text-sm mb-1 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-500 transition-colors"
						>
							{method.title}
						</h4>
						<p class="text-xs text-gray-500 dark:text-gray-400">
							{method.description}
						</p>
					</a>
				{/each}
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
