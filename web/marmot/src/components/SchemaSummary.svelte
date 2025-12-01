<script lang="ts">
	import CodeBlock from './CodeBlock.svelte';
	import {
		parseSchemaResponse,
		processSchema,
		validateSchema,
		prettyPrintSchema
	} from '$lib/schema/utils';
	import type { SchemaSection, Field } from '$lib/schema/types';

	let { schema = undefined, showRawSchema = false }: { schema?: any; showRawSchema?: boolean } = $props();
	let activeTab = $state('');
	let schemaSections = $state<SchemaSection[]>([]);
	let processedSchemas = $state<Record<string, { fields: Field[]; example: any }>>({});
	let validationErrors = $state<Record<string, any[]>>({});
	let expandedFields = $state(new Set<string>());

	$effect(() => {
		if (schema && schema !== null && schema !== undefined) {
			const sections = parseSchemaResponse(schema);
			const processed: Record<string, { fields: Field[]; example: any }> = {};
			const errors: Record<string, any[]> = {};

			sections.forEach((section) => {
				processed[section.name] = processSchema(section.schema);
				errors[section.name] = validateSchema(section.schema);
			});

			schemaSections = sections;
			processedSchemas = processed;
			validationErrors = errors;

			// Always set activeTab to first section when schema changes
			if (sections.length > 0) {
				activeTab = sections[0].name;
			} else {
				activeTab = '';
			}
		} else {
			schemaSections = [];
			processedSchemas = {};
			validationErrors = {};
			activeTab = '';
		}
	});

	function setActiveTab(tabName: string) {
		activeTab = tabName;
		expandedFields = new Set();
	}

	function toggleFieldExpansion(fieldPath: string) {
		const newExpanded = new Set(expandedFields);

		if (newExpanded.has(fieldPath)) {
			newExpanded.delete(fieldPath);

			const descendants = activeFields.filter((field) => {
				const parentField = activeFields.find((f) => f.name === fieldPath);
				if (!parentField) return false;

				const parentIndex = activeFields.indexOf(parentField);
				const fieldIndex = activeFields.indexOf(field);

				return fieldIndex > parentIndex && field.indentLevel > parentField.indentLevel;
			});

			descendants.forEach((descendant) => {
				newExpanded.delete(descendant.name);
			});
		} else {
			newExpanded.add(fieldPath);
		}

		expandedFields = newExpanded;
	}

	function isFieldExpanded(fieldPath: string): boolean {
		return expandedFields.has(fieldPath);
	}

	function hasChildren(field: Field, allFields: Field[]): boolean {
		const fieldIndex = allFields.indexOf(field);
		if (fieldIndex === -1) return false;

		for (let i = fieldIndex + 1; i < allFields.length; i++) {
			const nextField = allFields[i];
			if (nextField.indentLevel <= field.indentLevel) break;
			if (nextField.indentLevel === field.indentLevel + 1) {
				return true;
			}
		}
		return false;
	}

	function getVisibleFields(fields: Field[]): Field[] {
		if (!fields || fields.length === 0) return [];

		const visible: Field[] = [];

		for (let i = 0; i < fields.length; i++) {
			const field = fields[i];

			if (field.indentLevel === 0) {
				visible.push(field);
				continue;
			}

			let shouldShow = false;

			for (let j = i - 1; j >= 0; j--) {
				const potentialParent = fields[j];
				if (potentialParent.indentLevel < field.indentLevel) {
					if (potentialParent.indentLevel === field.indentLevel - 1) {
						shouldShow = expandedFields.has(potentialParent.name);
						break;
					}
				}
			}

			if (shouldShow) {
				visible.push(field);
			}
		}

		return visible;
	}

	function formatEnumDisplay(enumValues: string[]): string {
		const joined = enumValues.join(', ');
		return joined.length > 50 ? joined.substring(0, 47) + '...' : joined;
	}

	function getTypeStyle(fieldType: string): string {
		const typeMap: Record<string, string> = {
			anyOf: 'bg-blue-100 dark:bg-blue-900/20 text-blue-800 dark:text-blue-100',
			oneOf: 'bg-purple-100 dark:bg-purple-900/20 text-purple-800 dark:text-purple-100',
			allOf: 'bg-green-100 dark:bg-green-900/20 text-green-800 dark:text-green-100',
			not: 'bg-red-100 dark:bg-red-900/20 text-red-800 dark:text-red-100',
			error: 'bg-red-100 dark:bg-red-900/20 text-red-800 dark:text-red-100',
			object: 'bg-indigo-100 dark:bg-indigo-900/20 text-indigo-800 dark:text-indigo-100',
			array: 'bg-teal-100 dark:bg-teal-900/20 text-teal-800 dark:text-teal-100',
			string: 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-100',
			number: 'bg-emerald-100 dark:bg-emerald-900/20 text-emerald-800 dark:text-emerald-100',
			integer: 'bg-emerald-100 dark:bg-emerald-900/20 text-emerald-800 dark:text-emerald-100',
			boolean: 'bg-pink-100 dark:bg-pink-900/20 text-pink-800 dark:text-pink-100'
		};

		return (
			typeMap[fieldType] ||
			'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-100'
		);
	}

	function getSchemaExamples(schema: any): any[] {
		if (!schema) return [];

		const examples: any[] = [];

		if (schema.example !== undefined) {
			examples.push(schema.example);
		}

		if (schema.allOf) {
			schema.allOf.forEach((subSchema: any) => {
				if (subSchema?.example !== undefined) {
					examples.push(subSchema.example);
				}
			});
		}

		return examples;
	}

	let activeFields = $derived(
		activeTab && processedSchemas[activeTab] ? processedSchemas[activeTab].fields : []
	);
	let visibleFields = $derived(getVisibleFields(activeFields));
	let activeExample = $derived(
		activeTab && processedSchemas[activeTab] ? processedSchemas[activeTab].example : null
	);
	let activeSchema = $derived(
		activeTab ? schemaSections.find((s) => s.name === activeTab)?.schema : null
	);
	let activeErrors = $derived(
		activeTab && validationErrors[activeTab] ? validationErrors[activeTab] : []
	);
	let hasValidationErrors = $derived(activeErrors.length > 0);
	let showSchemaTabs = $derived(schemaSections.length > 1);
	let schemaExamples = $derived(activeSchema ? getSchemaExamples(activeSchema) : []);
</script>

<div class="space-y-6 overflow-x-auto">
	{#if showSchemaTabs}
		<div class="flex flex-wrap gap-2">
			{#each schemaSections as section}
				<button
					class="px-4 py-2 text-sm font-medium rounded-lg border transition-colors {activeTab ===
					section.name
						? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border-earthy-terracotta-200 dark:border-earthy-terracotta-800'
						: 'text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800'}"
					onclick={() => setActiveTab(section.name)}
				>
					{section.name.charAt(0).toUpperCase() + section.name.slice(1)} Schema
				</button>
			{/each}
		</div>
	{/if}

	{#if schemaSections.length > 0}
		{#if hasValidationErrors}
			<div
				class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4"
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
			{:else if visibleFields.length > 0}
			<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
					<div class="divide-y divide-gray-100 dark:divide-gray-700">
						{#each visibleFields as field}
							{@const fieldPath = field.name}
							{@const hasChildFields = hasChildren(field, activeFields)}
							{@const isExpanded = isFieldExpanded(fieldPath)}
							{@const indentLevel = field.indentLevel || 0}
							{@const showExpandedBg = isExpanded && hasChildFields}

							<div
								class="group transition-colors cursor-pointer {showExpandedBg
									? 'bg-blue-50 dark:bg-blue-950/50 hover:bg-blue-75 dark:hover:bg-blue-950/70'
									: 'hover:bg-gray-50 dark:hover:bg-gray-800/50'}"
								onclick={() => (hasChildFields ? toggleFieldExpansion(fieldPath) : undefined)}
							>
								<!-- Left border and indentation guides -->
								<div class="flex items-start relative">
									{#if indentLevel > 0}
										<div class="absolute left-0 top-0 bottom-0 flex">
											{#each Array(indentLevel) as _, level}
												<div class="w-8 flex justify-center relative">
													<div
														class="w-px bg-gray-200 dark:bg-gray-700 absolute top-0 bottom-0"
													></div>
													{#if level === indentLevel - 1}
														<div
															class="w-4 h-px bg-gray-200 dark:bg-gray-700 absolute top-6 left-1/2"
														></div>
													{/if}
												</div>
											{/each}
										</div>
									{/if}

									<!-- Content area -->
									<div
										class="flex items-start gap-3 px-4 py-3 w-full"
										style="margin-left: {indentLevel * 24}px"
									>
										<!-- Expansion toggle -->
										<div class="flex-shrink-0 w-6 flex items-center justify-center">
											{#if hasChildFields}
												<div
													class="w-5 h-5 flex items-center justify-center rounded text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 transition-colors"
												>
													<svg
														class="w-3 h-3 transition-transform {isExpanded ? 'rotate-90' : ''}"
														fill="currentColor"
														viewBox="0 0 20 20"
													>
														<path
															fill-rule="evenodd"
															d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 111.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
															clip-rule="evenodd"
														/>
													</svg>
												</div>
											{:else}
												<div class="w-2 h-2 rounded-full bg-gray-300 dark:bg-gray-600"></div>
											{/if}
										</div>

										<!-- Field content -->
										<div class="min-w-0 flex-1">
											<div class="flex items-start justify-between gap-3">
												<div class="min-w-0 flex-1">
													<div class="flex items-center gap-2 mb-2">
														<h4
															class="text-sm font-semibold text-gray-900 dark:text-gray-100 font-mono"
														>
															{field.name.split('.').pop()}
														</h4>

														{#if field.required}
															<span
																class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-red-100 dark:bg-red-900/20 text-red-800 dark:text-red-100"
															>
																Required
															</span>
														{:else}
															<span
																class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400"
															>
																Optional
															</span>
														{/if}
													</div>

													{#if field.description}
														<p class="text-sm text-gray-600 dark:text-gray-400 mb-2">
															{field.description}
														</p>
													{/if}

													<div class="flex flex-wrap gap-2 text-xs">
														{#if field.format}
															<span
																class="inline-flex items-center px-2 py-1 rounded bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300"
															>
																Format: {field.format}
															</span>
														{/if}
														{#if field.pattern}
															<span
																class="inline-flex items-center px-2 py-1 rounded bg-purple-50 dark:bg-purple-900/20 text-purple-700 dark:text-purple-300"
																title={field.pattern}
															>
																Pattern
															</span>
														{/if}
														{#if field.minimum !== undefined || field.maximum !== undefined}
															<span
																class="inline-flex items-center px-2 py-1 rounded bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300"
															>
																{field.minimum !== undefined ? `min: ${field.minimum}` : ''}
																{field.minimum !== undefined && field.maximum !== undefined
																	? ', '
																	: ''}
																{field.maximum !== undefined ? `max: ${field.maximum}` : ''}
															</span>
														{/if}
														{#if field.minLength !== undefined || field.maxLength !== undefined}
															<span
																class="inline-flex items-center px-2 py-1 rounded bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
															>
																{field.minLength !== undefined ? `minLen: ${field.minLength}` : ''}
																{field.minLength !== undefined && field.maxLength !== undefined
																	? ', '
																	: ''}
																{field.maxLength !== undefined ? `maxLen: ${field.maxLength}` : ''}
															</span>
														{/if}
														{#if field.default !== undefined}
															<span
																class="inline-flex items-center px-2 py-1 rounded bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-400"
															>
																Default: {JSON.stringify(field.default)}
															</span>
														{/if}
													</div>
												</div>

												<div class="flex-shrink-0">
													<span
														class="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold {getTypeStyle(
															field.type
														)}"
													>
														{field.type}
													</span>

													{#if field.const !== undefined}
														<div
															class="mt-2 font-mono text-xs text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/20 px-2 py-1 rounded"
														>
															{typeof field.const === 'string'
																? `"${field.const}"`
																: JSON.stringify(field.const)}
														</div>
													{:else if field.enum}
														<div class="mt-2 text-xs">
															<div class="cursor-help group relative">
																<div
																	class="bg-gray-50 dark:bg-gray-800 px-2 py-1 rounded text-gray-600 dark:text-gray-400"
																>
																	{formatEnumDisplay(field.enum)}
																</div>
																<div
																	class="absolute z-10 invisible group-hover:visible bg-gray-900 dark:bg-gray-700 text-white text-xs rounded px-3 py-2 mt-1 left-0 min-w-max max-w-sm shadow-lg"
																>
																	{#each field.enum as value}
																		<div class="py-0.5">{value}</div>
																	{/each}
																</div>
															</div>
														</div>
													{/if}
												</div>
											</div>
										</div>
									</div>
								</div>
							</div>
						{/each}
					</div>
			</div>

			{#if activeExample && typeof activeExample === 'object' && Object.keys(activeExample).length > 0}
				<div class="space-y-4">
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
						{schemaExamples.length > 1 ? 'Examples' : 'Example'}
					</h3>
					<CodeBlock code={JSON.stringify(activeExample, null, 2)} />
				</div>
			{/if}

			{#if schemaExamples.length > 1}
				{#each schemaExamples.slice(1) as example, i}
					<div class="mt-4">
						<CodeBlock code={JSON.stringify(example, null, 2)} />
					</div>
				{/each}
			{/if}
			{:else}
				<div class="text-center py-12 text-gray-500 dark:text-gray-400">
					<p>No fields available for {activeTab} schema</p>
				</div>
			{/if}
		{/key}
	{:else}
		<div class="text-center py-12 text-gray-500 dark:text-gray-400">
			<p>No schema available</p>
		</div>
	{/if}
</div>
