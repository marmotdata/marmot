<script lang="ts">
	import CodeBlock from './CodeBlock.svelte';
	import {
		parseSchemaResponse,
		processSchema,
		isSchemaAvailable,
		detectSchemaType,
		validateSchema,
		prettyPrintSchema
	} from '$lib/schema/utils';
	import type { SchemaSection, Field } from '$lib/schema/types';

	export let schema: any | undefined = undefined;

	let showRawSchema = false;
	let activeTab = '';
	let schemaSections: SchemaSection[] = [];
	let processedSchemas: Record<string, { fields: Field[]; example: any }> = {};
	let validationErrors: Record<string, any[]> = {};

	$: {
		if (schema) {
			schemaSections = parseSchemaResponse(schema);
			processedSchemas = {};
			validationErrors = {};

			schemaSections.forEach((section) => {
				processedSchemas[section.name] = processSchema(section.schema);
				validationErrors[section.name] = validateSchema(section.schema);
			});

			if (schemaSections.length > 0 && !activeTab) {
				activeTab = schemaSections[0].name;
			}
		} else {
			schemaSections = [];
			processedSchemas = {};
			validationErrors = {};
			activeTab = '';
		}
	}

	function setActiveTab(tabName: string) {
		activeTab = tabName;
	}

	function formatEnumDisplay(enumValues: string[]): string {
		const joined = enumValues.join(', ');
		return joined.length > 50 ? joined.substring(0, 47) + '...' : joined;
	}

	$: activeFields =
		activeTab && processedSchemas[activeTab] ? processedSchemas[activeTab].fields : [];
	$: activeExample =
		activeTab && processedSchemas[activeTab] ? processedSchemas[activeTab].example : null;
	$: activeSchema = activeTab ? schemaSections.find((s) => s.name === activeTab)?.schema : null;
	$: schemaType = activeSchema ? detectSchemaType(activeSchema) : 'json';
	$: activeErrors = activeTab && validationErrors[activeTab] ? validationErrors[activeTab] : [];
	$: hasValidationErrors = activeErrors.length > 0;
	$: showSchemaTabs = schemaSections.length > 1;
	$: hasExample =
		activeExample && typeof activeExample === 'object' && Object.keys(activeExample).length > 0;
</script>

<div class="space-y-4">
	<div class="flex justify-between mb-4">
		<div class="flex space-x-4 overflow-x-auto">
			{#if showSchemaTabs}
				{#each schemaSections as section}
					<button
						class="px-3 py-2 text-sm font-medium rounded-md {activeTab === section.name
							? 'bg-orange-100 dark:bg-orange-900/20 text-orange-800 dark:text-orange-100'
							: 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'}"
						on:click={() => setActiveTab(section.name)}
					>
						{section.name.charAt(0).toUpperCase() + section.name.slice(1)} Schema
					</button>
				{/each}
			{/if}
		</div>
		{#if schemaSections.length > 0}
			<button
				class="inline-flex items-center px-3 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-orange-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500 dark:focus:ring-orange-400"
				on:click={() => (showRawSchema = !showRawSchema)}
			>
				{showRawSchema ? 'Show Formatted' : 'Show Raw'}
			</button>
		{/if}
	</div>

	{#if schemaSections.length > 0}
		{#if hasValidationErrors}
			<div
				class="mb-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-md p-4"
			>
				<div class="flex">
					<div class="flex-shrink-0">
						<svg
							class="h-5 w-5 text-yellow-400"
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 20 20"
							fill="currentColor"
						>
							<path
								fill-rule="evenodd"
								d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
								clip-rule="evenodd"
							/>
						</svg>
					</div>
					<div class="ml-3">
						<h3 class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
							Schema Validation Warnings
						</h3>
						<div class="mt-2 text-sm text-yellow-700 dark:text-yellow-300">
							<ul class="list-disc pl-5 space-y-1">
								{#each activeErrors.slice(0, 3) as error}
									<li>{error.message || JSON.stringify(error)}</li>
								{/each}
								{#if activeErrors.length > 3}
									<li>...and {activeErrors.length - 3} more issues</li>
								{/if}
							</ul>
						</div>
					</div>
				</div>
			</div>
		{/if}

		{#key activeTab + '-' + showRawSchema}
			{#if showRawSchema && activeSchema}
				{#if typeof activeSchema === 'string'}
					{#if activeSchema.includes('syntax = "proto') || activeSchema.includes('message ') || activeSchema.includes('type: record')}
						<CodeBlock code={activeSchema} />
					{:else}
						<CodeBlock code={prettyPrintSchema(activeSchema)} />
					{/if}
				{:else}
					<CodeBlock code={JSON.stringify(activeSchema, null, 2)} />
				{/if}
			{:else if activeFields.length > 0}
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700"
				>
					<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
						<thead class="bg-gray-50 dark:bg-gray-900">
							<tr>
								<th
									scope="col"
									class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>Field</th
								>
								<th
									scope="col"
									class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>Type</th
								>
								<th
									scope="col"
									class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>Required</th
								>
								<th
									scope="col"
									class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>Format</th
								>
								<th
									scope="col"
									class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
									>Description</th
								>
							</tr>
						</thead>
						<tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
							{#each activeFields as field}
								<tr>
									<td
										class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100"
									>
										{#if field.indentLevel > 0}
											<span class="inline-block" style="width: {field.indentLevel * 16}px"></span>
											<span class="text-gray-400">└─</span>
										{/if}
										<span class="ml-1">{field.name}</span>
									</td>
									<td class="px-6 py-4 whitespace-nowrap text-sm">
										<span
											class="px-2 py-1 text-xs font-medium bg-orange-100 dark:bg-orange-900/20 text-orange-800 dark:text-orange-100 rounded-full"
										>
											{field.type}
										</span>
										{#if field.enum}
											<div class="mt-1 text-xs text-gray-500 dark:text-gray-400 relative group">
												<span class="cursor-help">
													{formatEnumDisplay(field.enum)}
												</span>
												<div
													class="absolute z-10 invisible group-hover:visible bg-gray-900 dark:bg-gray-700 text-white text-xs rounded px-3 py-2 mt-1 left-0 min-w-max max-w-sm shadow-lg"
												>
													{#each field.enum as value}
														<div class="py-0.5">{value}</div>
													{/each}
												</div>
											</div>
										{/if}
									</td>
									<td class="px-6 py-4 whitespace-nowrap text-sm">
										{#if field.required}
											<span
												class="px-2 py-1 text-xs font-medium bg-red-100 dark:bg-red-900/20 text-red-800 dark:text-red-100 rounded-full"
												>Required</span
											>
										{:else}
											<span
												class="px-2 py-1 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded-full"
												>Optional</span
											>
										{/if}
									</td>
									<td class="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
										{#if field.format}
											{field.format}
										{:else if field.pattern}
											<span title={field.pattern}>Pattern</span>
										{:else if field.minimum !== undefined || field.maximum !== undefined}
											{field.minimum !== undefined ? `min: ${field.minimum}` : ''}
											{field.minimum !== undefined && field.maximum !== undefined ? ', ' : ''}
											{field.maximum !== undefined ? `max: ${field.maximum}` : ''}
										{:else if field.minLength !== undefined || field.maxLength !== undefined}
											{field.minLength !== undefined ? `minLen: ${field.minLength}` : ''}
											{field.minLength !== undefined && field.maxLength !== undefined ? ', ' : ''}
											{field.maxLength !== undefined ? `maxLen: ${field.maxLength}` : ''}
										{:else}
											—
										{/if}
									</td>
									<td class="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
										{#if field.description}
											{field.description}
										{:else if field.default !== undefined}
											<span class="text-gray-400">Default: {JSON.stringify(field.default)}</span>
										{:else}
											—
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				{#if hasExample}
					<div class="mt-6">
						<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-gray-100 mb-4">
							Example
						</h3>
						<CodeBlock code={JSON.stringify(activeExample, null, 2)} />
					</div>
				{/if}
			{:else}
				<p class="text-gray-500 dark:text-gray-400 italic">
					No fields available for {activeTab} schema
				</p>
			{/if}
		{/key}
	{:else}
		<p class="text-gray-500 dark:text-gray-400 italic">No schema available</p>
	{/if}
</div>