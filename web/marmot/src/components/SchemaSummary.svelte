<script lang="ts">
	import CodeBlock from './CodeBlock.svelte';

	export let schema: any | undefined = undefined;

	let showRawSchema = false;
	let activeTab: 'message' | 'headers' = 'message';

	interface Field {
		name: string;
		type: string;
		description?: string;
		format?: string;
		required?: boolean;
		fields?: Field[];
		items?: {
			type: string;
			fields?: Field[];
		};
		$ref?: string;
		indentLevel?: number;
	}

	function resolveRef(ref: string, definitions: any): any {
		if (!ref?.startsWith('#/definitions/')) return null;
		const definitionName = ref.substring('#/definitions/'.length);
		return definitions?.[definitionName];
	}

	function getFieldType(fieldSchema: any): string {
		if (fieldSchema.$ref) return 'object';
		if (fieldSchema.type === 'array') {
			const itemType = fieldSchema.items?.type || 'object';
			return `array<${itemType}>`;
		}
		return fieldSchema.type || 'unknown';
	}

	function processField(
		fieldName: string,
		fieldSchema: any,
		required: string[] = [],
		definitions: any = {},
		depth = 0
	): Field[] {
		const fields: Field[] = [];

		// Base field info
		const field: Field = {
			name: fieldName,
			type: getFieldType(fieldSchema),
			description: fieldSchema.description,
			format: fieldSchema.format,
			required: required.includes(fieldName)
		};

		fields.push({
			...field,
			indentLevel: depth
		});

		// Handle $ref
		if (fieldSchema.$ref) {
			const refSchema = resolveRef(fieldSchema.$ref, definitions);
			if (refSchema?.properties) {
				Object.entries(refSchema.properties).forEach(([name, schema]) => {
					fields.push(
						...processField(name, schema, refSchema.required || [], definitions, depth + 1)
					);
				});
			}
		}

		// Handle object properties
		if (fieldSchema.type === 'object' && fieldSchema.properties) {
			Object.entries(fieldSchema.properties).forEach(([name, schema]) => {
				fields.push(
					...processField(name, schema, fieldSchema.required || [], definitions, depth + 1)
				);
			});
		}

		// Handle array items
		if (fieldSchema.type === 'array' && fieldSchema.items) {
			if (fieldSchema.items.$ref) {
				const refSchema = resolveRef(fieldSchema.items.$ref, definitions);
				if (refSchema?.properties) {
					Object.entries(refSchema.properties).forEach(([name, schema]) => {
						fields.push(
							...processField(
								`${fieldName}[].${name}`,
								schema,
								refSchema.required || [],
								definitions,
								depth + 1
							)
						);
					});
				}
			} else if (fieldSchema.items.properties) {
				Object.entries(fieldSchema.items.properties).forEach(([name, schema]) => {
					fields.push(
						...processField(
							`${fieldName}[].${name}`,
							schema,
							fieldSchema.items.required || [],
							definitions,
							depth + 1
						)
					);
				});
			}
		}

		return fields;
	}

	function processSchema(schemaSection: any): Field[] {
		if (!schemaSection) return [];

		let fields: Field[] = [];

		// Handle allOf
		if (schemaSection.allOf) {
			schemaSection.allOf.forEach((subSchema: any) => {
				if (subSchema.properties) {
					Object.entries(subSchema.properties).forEach(([fieldName, fieldSchema]) => {
						fields.push(
							...processField(
								fieldName,
								fieldSchema,
								subSchema.required || [],
								schemaSection.definitions
							)
						);
					});
				}
			});
		}
		// Handle regular schema
		else if (schemaSection.properties) {
			Object.entries(schemaSection.properties).forEach(([fieldName, fieldSchema]) => {
				fields.push(
					...processField(
						fieldName,
						fieldSchema,
						schemaSection.required || [],
						schemaSection.definitions
					)
				);
			});
		}
		// Handle schema with direct fields (for headers)
		else if (schemaSection.schema) {
			if (schemaSection.schema.$ref) {
				// Just show the ref for now
				fields.push({
					name: 'Schema Reference',
					type: 'ref',
					description: schemaSection.schema.$ref
				});
			} else {
				fields = processSchema(schemaSection.schema);
			}
		}

		return fields;
	}

	$: hasMessageSchema =
		schema?.message &&
		(schema.message.properties || (schema.message.allOf && schema.message.allOf.length > 0));
	$: hasHeaderSchema = schema?.headers;
	$: hasSchema = hasMessageSchema || hasHeaderSchema;

	$: messageFields = hasMessageSchema ? processSchema(schema.message) : [];
	$: headerFields = hasHeaderSchema ? processSchema(schema.headers) : [];

	$: messageExample = schema?.message?.example;
	$: headerExample = schema?.headers?.example;
</script>

<div class="space-y-4">
	<div class="flex justify-between mb-4">
		<div class="flex space-x-4">
			{#if hasMessageSchema}
				<button
					class="px-3 py-2 text-sm font-medium rounded-md {activeTab === 'message'
						? 'bg-orange-100 dark:bg-orange-900/20 text-orange-800 dark:text-orange-100'
						: 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'}"
					on:click={() => (activeTab = 'message')}
				>
					Message Schema
				</button>
			{/if}
			{#if hasHeaderSchema}
				<button
					class="px-3 py-2 text-sm font-medium rounded-md {activeTab === 'headers'
						? 'bg-orange-100 dark:bg-orange-900/20 text-orange-800 dark:text-orange-100'
						: 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'}"
					on:click={() => (activeTab = 'headers')}
				>
					Headers Schema
				</button>
			{/if}
		</div>
		<button
			class="inline-flex items-center px-3 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-orange-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500 dark:focus:ring-orange-400"
			on:click={() => (showRawSchema = !showRawSchema)}
		>
			{showRawSchema ? 'Show Formatted' : 'Show Raw'}
		</button>
	</div>

	{#if showRawSchema}
		{#if activeTab === 'message' && schema?.message}
			<CodeBlock code={schema.message} />
		{:else if activeTab === 'headers' && schema?.headers}
			<CodeBlock code={schema.headers} />
		{:else}
			<p class="text-gray-500 dark:text-gray-400 italic">No schema available for {activeTab}</p>
		{/if}
	{:else if hasSchema}
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
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
					{#each activeTab === 'message' ? messageFields : headerFields as field}
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
							<td class="px-6 py-4 text-sm text-gray-500 dark:text-gray-400"
								>{field.format || '—'}</td
							>
							<td class="px-6 py-4 text-sm text-gray-500 dark:text-gray-400"
								>{field.description || '—'}</td
							>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<div class="mt-6">
			<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-gray-100 mb-4">Example</h3>

			{#if activeTab === 'message'}
				{#if messageExample}
					<CodeBlock code={messageExample} />
				{:else}
					<p class="text-gray-500 dark:text-gray-400 italic">No example available for Message</p>
				{/if}
			{:else if activeTab === 'headers'}
				{#if headerExample}
					<CodeBlock code={headerExample} />
				{:else}
					<p class="text-gray-500 dark:text-gray-400 italic">No example available for Headers</p>
				{/if}
			{/if}
		</div>
	{:else}
		<p class="text-gray-500 dark:text-gray-400 italic">No schema available</p>
	{/if}
</div>
