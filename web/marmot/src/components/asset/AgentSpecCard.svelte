<script lang="ts">
	import type { Asset } from '$lib/assets/types';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import CodeBlock from '$components/editor/CodeBlock.svelte';

	let { asset }: { asset: Asset } = $props();

	let framework = $derived(asset.metadata?.framework as string | undefined);
	let model = $derived(asset.metadata?.model as string | undefined);
	let toolNames = $derived<string[]>(
		Array.isArray(asset.metadata?.tool_names) ? asset.metadata.tool_names : []
	);
	let prompt = $derived(
		(asset.metadata?.system_prompt || asset.metadata?.prompt || asset.metadata?.instructions) as
			| string
			| undefined
	);

	let promptExpanded = $state(false);
</script>

<div class="space-y-6">
	<!-- Spec strip -->
	<div
		class="rounded-xl border border-gray-200 dark:border-gray-700 bg-gradient-to-br from-white to-gray-50 dark:from-gray-800 dark:to-gray-900 overflow-hidden"
	>
		<div
			class="grid grid-cols-1 md:grid-cols-3 divide-y md:divide-y-0 md:divide-x divide-gray-200 dark:divide-gray-700"
		>
			<!-- Framework -->
			<div class="p-5 flex items-center gap-4">
				<div
					class="flex-shrink-0 w-12 h-12 rounded-lg bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 flex items-center justify-center"
				>
					{#if framework}
						<Icon name={framework} size="md" showLabel={false} />
					{:else}
						<IconifyIcon icon="material-symbols:layers-outline" class="w-7 h-7 text-gray-400" />
					{/if}
				</div>
				<div class="min-w-0">
					<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">
						Framework
					</div>
					<div class="text-base font-semibold text-gray-900 dark:text-gray-100 truncate">
						{framework || '—'}
					</div>
				</div>
			</div>

			<!-- Model -->
			<div class="p-5 flex items-center gap-4">
				<div
					class="flex-shrink-0 w-12 h-12 rounded-lg bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 flex items-center justify-center"
				>
					<IconifyIcon
						icon="material-symbols:neurology-outline"
						class="w-7 h-7 text-earthy-terracotta-700 dark:text-earthy-terracotta-500"
					/>
				</div>
				<div class="min-w-0">
					<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">Model</div>
					<div class="text-base font-semibold text-gray-900 dark:text-gray-100 font-mono truncate">
						{model || '—'}
					</div>
				</div>
			</div>

			<!-- Tools count -->
			<div class="p-5 flex items-center gap-4">
				<div
					class="flex-shrink-0 w-12 h-12 rounded-lg bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 flex items-center justify-center"
				>
					<IconifyIcon
						icon="material-symbols:build-outline"
						class="w-7 h-7 text-earthy-terracotta-700 dark:text-earthy-terracotta-500"
					/>
				</div>
				<div class="min-w-0">
					<div class="text-[10px] font-semibold uppercase tracking-wider text-gray-400">
						Capabilities
					</div>
					<div class="text-base font-semibold text-gray-900 dark:text-gray-100">
						{toolNames.length}
						<span class="text-sm font-normal text-gray-500 dark:text-gray-400">
							{toolNames.length === 1 ? 'tool' : 'tools'}
						</span>
					</div>
				</div>
			</div>
		</div>
	</div>

	<!-- Prompt (if present) -->
	{#if prompt}
		<div
			class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden"
		>
			<button
				class="w-full flex items-center justify-between px-5 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors"
				onclick={() => (promptExpanded = !promptExpanded)}
			>
				<div class="flex items-center gap-2">
					<IconifyIcon icon="material-symbols:format-quote-outline" class="w-5 h-5 text-gray-400" />
					<span class="text-sm font-medium text-gray-900 dark:text-gray-100">System Prompt</span>
					<span class="text-xs text-gray-500 dark:text-gray-400">
						{prompt.length} chars
					</span>
				</div>
				<IconifyIcon
					icon={promptExpanded
						? 'material-symbols:keyboard-arrow-up'
						: 'material-symbols:keyboard-arrow-down'}
					class="w-5 h-5 text-gray-400"
				/>
			</button>
			{#if promptExpanded}
				<div class="border-t border-gray-200 dark:border-gray-700 p-4">
					<CodeBlock code={prompt} language="markdown" />
				</div>
			{/if}
		</div>
	{/if}

	<MetadataView {asset} />
</div>
