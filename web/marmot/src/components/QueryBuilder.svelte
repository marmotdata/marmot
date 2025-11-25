<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import QueryInput from './QueryInput.svelte';
	import { fetchApi } from '$lib/api';

	let {
		query = '',
		onQueryChange,
		initiallyExpanded = false
	}: {
		query?: string;
		onQueryChange: (query: string) => void;
		initiallyExpanded?: boolean;
	} = $props();

	type Operator = '=' | 'contains' | '!=' | '>' | '<' | '>=' | '<=' | 'range';
	type BooleanOperator = 'AND' | 'OR' | 'NOT';

	interface FilterRow {
		id: string;
		field: string;
		operator: Operator;
		value: string;
		booleanOp: BooleanOperator;
		rangeFrom?: string;
		rangeTo?: string;
	}

	let mode: 'builder' | 'code' = $state('builder');
	let filters: FilterRow[] = $state([
		{
			id: crypto.randomUUID(),
			field: '',
			operator: '=',
			value: '',
			booleanOp: 'AND',
			rangeFrom: '',
			rangeTo: ''
		}
	]);
	let freeText = $state('');
	let rawQuery = $state(query || '');
	let showFieldSuggestions = $state(false);
	let activeFieldIndex: number | null = $state(null);
	let selectedSuggestionIndex = $state(-1);
	let inputElement: HTMLInputElement | null = $state(null);
	let dropdownPosition = $state({ top: 0, left: 0, width: 0 });

	// Value suggestions state
	let showValueSuggestions = $state(false);
	let activeValueIndex: number | null = $state(null);
	let valueSuggestions: { value: string }[] = $state([]);
	let allValueSuggestions: { value: string }[] = $state([]); // Store all fetched values
	let selectedValueIndex = $state(-1);
	let valueInputElement: HTMLInputElement | null = $state(null);
	let valueDropdownPosition = $state({ top: 0, left: 0, width: 0 });
	let valueFetchCache: { [key: string]: { value: string }[] } = {};

	// Operator dropdown state
	let showOperatorDropdown = $state(false);
	let activeOperatorIndex: number | null = $state(null);
	let selectedOperatorIndex = $state(-1);
	let operatorDropdownPosition = $state({ top: 0, left: 0, width: 0 });

	// Collapsible sections
	let expanded = $state(initiallyExpanded);
	let freeTextExpanded = $state(true);
	let filtersExpanded = $state(true);

	// Field structure - separate simple fields from nested metadata
	const simpleFields = [
		{
			value: 'kind',
			label: '@kind',
			description: 'Resource kind (asset, glossary, team)',
			category: 'Simple Field'
		},
		{ value: 'type', label: '@type', description: 'Asset type', category: 'Simple Field' },
		{
			value: 'provider',
			label: '@provider',
			description: 'Data provider',
			category: 'Simple Field'
		}
	];

	const metadataFields = [
		{
			value: 'metadata.type',
			label: '@metadata.type',
			description: 'Asset type',
			category: 'Metadata'
		},
		{
			value: 'metadata.name',
			label: '@metadata.name',
			description: 'Asset name',
			category: 'Metadata'
		},
		{
			value: 'metadata.description',
			label: '@metadata.description',
			description: 'Description',
			category: 'Metadata'
		},
		{
			value: 'metadata.environment',
			label: '@metadata.environment',
			description: 'Environment',
			category: 'Metadata'
		},
		{
			value: 'metadata.owner',
			label: '@metadata.owner',
			description: 'Owner',
			category: 'Metadata'
		},
		{ value: 'metadata.team', label: '@metadata.team', description: 'Team', category: 'Metadata' },
		{
			value: 'metadata.status',
			label: '@metadata.status',
			description: 'Status',
			category: 'Metadata'
		},
		{
			value: 'metadata.provider',
			label: '@metadata.provider',
			description: 'Provider',
			category: 'Metadata'
		},
		{
			value: 'metadata.created_at',
			label: '@metadata.created_at',
			description: 'Created date',
			category: 'Metadata'
		},
		{
			value: 'metadata.updated_at',
			label: '@metadata.updated_at',
			description: 'Updated date',
			category: 'Metadata'
		},
		{
			value: 'metadata.created_by',
			label: '@metadata.created_by',
			description: 'Created by user',
			category: 'Metadata'
		},
		{ value: 'metadata.tags', label: '@metadata.tags', description: 'Tags', category: 'Metadata' }
	];

	const allFields = [...simpleFields, ...metadataFields];

	const operators: { value: Operator; label: string }[] = [
		{ value: '=', label: 'Equals (=)' },
		{ value: 'contains', label: 'Contains' },
		{ value: '!=', label: 'Not Equals (!=)' },
		{ value: '>', label: 'Greater Than (>)' },
		{ value: '<', label: 'Less Than (<)' },
		{ value: '>=', label: 'Greater or Equal (>=)' },
		{ value: '<=', label: 'Less or Equal (<=)' },
		{ value: 'range', label: 'Range [FROM TO]' }
	];

	const booleanOperators: BooleanOperator[] = ['AND', 'OR', 'NOT'];

	// Parse existing query into builder format when switching to builder mode
	$effect(() => {
		if (mode === 'builder' && rawQuery) {
			parseQueryToBuilder(rawQuery);
		}
	});

	function parseQueryToBuilder(queryStr: string) {
		// Simple parser for basic queries
		// This is a simplified version - the full parser is on the backend
		const parts = queryStr.split(/\s+(AND|OR|NOT)\s+/i);
		const newFilters: FilterRow[] = [];
		let currentBoolOp: BooleanOperator = 'AND';

		for (let i = 0; i < parts.length; i++) {
			const part = parts[i].trim();
			if (part === 'AND' || part === 'OR' || part === 'NOT') {
				currentBoolOp = part as BooleanOperator;
				continue;
			}

			// Match any field starting with @ (e.g., @metadata.*, @kind, @type, @provider)
			if (part.startsWith('@')) {
				const match = part.match(/@(\S+)\s*(=|:|contains|!=|>|<|>=|<=)\s*(.+)/);
				if (match) {
					newFilters.push({
						id: crypto.randomUUID(),
						field: match[1],
						operator: (match[2] === ':' ? '=' : match[2]) as Operator,
						value: match[3].replace(/['"]/g, ''),
						booleanOp: currentBoolOp
					});
				}
			} else if (part && !part.startsWith('@')) {
				freeText = part;
			}
		}

		if (newFilters.length > 0) {
			filters = newFilters;
		}
	}

	function buildQueryString(): string {
		let queryParts: string[] = [];

		if (freeText.trim()) {
			queryParts.push(freeText.trim());
		}

		filters.forEach((filter, index) => {
			if (!filter.field || !filter.value) return;

			// Add boolean operator before this filter (except for first filter)
			if (index > 0 || freeText.trim()) {
				queryParts.push(filter.booleanOp);
			}

			let filterStr = `@${filter.field}`;

			if (filter.operator === 'range' && filter.rangeFrom && filter.rangeTo) {
				filterStr += ` range [${filter.rangeFrom} TO ${filter.rangeTo}]`;
			} else {
				const needsQuotes = filter.value.includes(' ');
				const valueStr = needsQuotes ? `"${filter.value}"` : filter.value;
				filterStr += ` ${filter.operator} ${valueStr}`;
			}

			queryParts.push(filterStr);
		});

		return queryParts.join(' ');
	}

	function addFilter() {
		filters = [
			...filters,
			{
				id: crypto.randomUUID(),
				field: '',
				operator: '=',
				value: '',
				booleanOp: 'AND',
				rangeFrom: '',
				rangeTo: ''
			}
		];
	}

	function removeFilter(id: string) {
		filters = filters.filter((f) => f.id !== id);
		if (filters.length === 0) {
			addFilter();
		}
	}

	function applyQuery() {
		if (mode === 'builder') {
			rawQuery = buildQueryString();
		}
		onQueryChange(rawQuery);
	}

	function toggleMode() {
		if (mode === 'builder') {
			// Switching to code mode - build query string
			rawQuery = buildQueryString();
			mode = 'code';
		} else {
			// Switching to builder mode - parse query
			parseQueryToBuilder(rawQuery);
			mode = 'builder';
		}
	}

	function handleFieldFocus(index: number, element: HTMLInputElement) {
		activeFieldIndex = index;
		showFieldSuggestions = true;
		selectedSuggestionIndex = -1;
		inputElement = element;
		updateDropdownPosition(element);
	}

	function updateDropdownPosition(element: HTMLInputElement) {
		const rect = element.getBoundingClientRect();
		dropdownPosition = {
			top: rect.bottom + window.scrollY + 4,
			left: rect.left + window.scrollX,
			width: Math.max(rect.width, 400)
		};
	}

	function handleFieldBlur() {
		// Delay to allow click on suggestion
		setTimeout(() => {
			showFieldSuggestions = false;
			activeFieldIndex = null;
			selectedSuggestionIndex = -1;
		}, 200);
	}

	function selectField(index: number, fieldValue: string) {
		filters[index].field = fieldValue;
		showFieldSuggestions = false;
		selectedSuggestionIndex = -1;

		// Focus the operator button after selecting a field
		setTimeout(() => {
			const operatorBtn = document.querySelector(`#operator-btn-${index}`) as HTMLButtonElement;
			if (operatorBtn) {
				operatorBtn.focus();
			}
		}, 50);
	}

	function getFilteredSuggestions(searchText: string) {
		const filtered = allFields.filter(
			(f) =>
				f.value.toLowerCase().includes(searchText.toLowerCase()) ||
				f.label.toLowerCase().includes(searchText.toLowerCase())
		);
		return filtered;
	}

	function handleFieldKeydown(event: KeyboardEvent, index: number) {
		const filteredSuggestions = getFilteredSuggestions(filters[index].field);

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			if (!showFieldSuggestions) {
				showFieldSuggestions = true;
				activeFieldIndex = index;
			}
			selectedSuggestionIndex = Math.min(
				selectedSuggestionIndex + 1,
				filteredSuggestions.length - 1
			);
			scrollSuggestionIntoView();
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedSuggestionIndex = Math.max(selectedSuggestionIndex - 1, -1);
			scrollSuggestionIntoView();
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (selectedSuggestionIndex >= 0 && filteredSuggestions[selectedSuggestionIndex]) {
				selectField(index, filteredSuggestions[selectedSuggestionIndex].value);
			}
		} else if (event.key === 'Escape') {
			event.preventDefault();
			showFieldSuggestions = false;
			selectedSuggestionIndex = -1;
		}
	}

	function scrollSuggestionIntoView() {
		setTimeout(() => {
			const activeElement = document.querySelector('.suggestion-active');
			if (activeElement) {
				activeElement.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
			}
		}, 0);
	}

	async function fetchValueSuggestions(field: string, prefix: string) {
		try {
			const cacheKey = `${field}-${prefix}`;
			if (valueFetchCache[cacheKey]) {
				return valueFetchCache[cacheKey];
			}

			const params = new URLSearchParams({
				field: field,
				prefix: prefix,
				limit: '10'
			});
			const response = await fetchApi(`/assets/suggestions/metadata/values?${params}`);

			if (!response.ok) {
				console.error('Failed to fetch value suggestions:', response.statusText);
				return [];
			}

			const data = await response.json();

			// Handle different response formats
			let suggestions: { value: string }[] = [];
			if (Array.isArray(data)) {
				suggestions = data
					.filter((d: any) => d && d.value !== null && d.value !== undefined)
					.map((d: any) => ({ value: String(d.value) }));
			} else if (data && typeof data === 'object') {
				// If it's an object with a nested array, try to extract it
				if (Array.isArray(data.values)) {
					suggestions = data.values
						.filter((d: any) => d && d.value !== null && d.value !== undefined)
						.map((d: any) => ({ value: String(d.value) }));
				} else if (Array.isArray(data.suggestions)) {
					suggestions = data.suggestions
						.filter((d: any) => d && d.value !== null && d.value !== undefined)
						.map((d: any) => ({ value: String(d.value) }));
				}
			}

			valueFetchCache[cacheKey] = suggestions;
			return suggestions;
		} catch (error) {
			console.error('Error fetching value suggestions:', error);
			return [];
		}
	}

	async function handleValueFocus(index: number, element: HTMLInputElement) {
		const filter = filters[index];
		if (!filter.field) return;

		activeValueIndex = index;
		valueInputElement = element;
		updateValueDropdownPosition(element);

		const fieldName = filter.field.replace(/^(@)?metadata\./, '');

		const allSuggestions = await fetchValueSuggestions(fieldName, '');
		allValueSuggestions = allSuggestions;

		filterValueSuggestions(filter.value || '');
		selectedValueIndex = -1;
	}

	function updateValueDropdownPosition(element: HTMLInputElement) {
		const rect = element.getBoundingClientRect();
		valueDropdownPosition = {
			top: rect.bottom + window.scrollY + 4,
			left: rect.left + window.scrollX,
			width: Math.max(rect.width, 300)
		};
	}

	function handleValueBlur() {
		setTimeout(() => {
			showValueSuggestions = false;
			activeValueIndex = null;
			selectedValueIndex = -1;
		}, 200);
	}

	function selectValue(index: number, value: string) {
		filters[index].value = value;
		showValueSuggestions = false;
		selectedValueIndex = -1;
	}

	function filterValueSuggestions(searchText: string) {
		if (!searchText.trim()) {
			valueSuggestions = allValueSuggestions;
		} else {
			const lowerSearch = searchText.toLowerCase();
			valueSuggestions = allValueSuggestions.filter((s) =>
				s.value.toLowerCase().includes(lowerSearch)
			);
		}
		showValueSuggestions = valueSuggestions.length > 0;
	}

	async function handleValueInput(index: number) {
		const filter = filters[index];
		if (!filter.field) return;

		filterValueSuggestions(filter.value || '');
		selectedValueIndex = -1;
	}

	function handleOperatorFocus(index: number, element: HTMLButtonElement) {
		activeOperatorIndex = index;
		const rect = element.getBoundingClientRect();
		operatorDropdownPosition = {
			top: rect.bottom + window.scrollY + 4,
			left: rect.left + window.scrollX,
			width: rect.width
		};
		showOperatorDropdown = true;
		selectedOperatorIndex = -1;
	}

	function handleOperatorBlur() {
		setTimeout(() => {
			showOperatorDropdown = false;
			activeOperatorIndex = null;
			selectedOperatorIndex = -1;
		}, 200);
	}

	function selectOperator(index: number, operatorValue: Operator) {
		filters[index].operator = operatorValue;
		showOperatorDropdown = false;
		selectedOperatorIndex = -1;

		setTimeout(() => {
			const valueInput = document.querySelector(`#value-input-${index}`) as HTMLInputElement;
			if (valueInput) {
				valueInput.focus();
			}
		}, 50);
	}

	function handleOperatorKeydown(event: KeyboardEvent, index: number) {
		if (!showOperatorDropdown) {
			if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown') {
				event.preventDefault();
				const btn = document.querySelector(`#operator-btn-${index}`) as HTMLButtonElement;
				if (btn) {
					handleOperatorFocus(index, btn);
				}
			}
			return;
		}

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedOperatorIndex = Math.min(selectedOperatorIndex + 1, operators.length - 1);
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedOperatorIndex = Math.max(selectedOperatorIndex - 1, -1);
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (selectedOperatorIndex >= 0 && operators[selectedOperatorIndex]) {
				selectOperator(index, operators[selectedOperatorIndex].value);
			}
		} else if (event.key === 'Escape') {
			event.preventDefault();
			showOperatorDropdown = false;
			selectedOperatorIndex = -1;
		}
	}

	function handleValueKeydown(event: KeyboardEvent, index: number) {
		if (!showValueSuggestions || valueSuggestions.length === 0) return;

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedValueIndex = Math.min(selectedValueIndex + 1, valueSuggestions.length - 1);
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedValueIndex = Math.max(selectedValueIndex - 1, -1);
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (selectedValueIndex >= 0 && valueSuggestions[selectedValueIndex]) {
				selectValue(index, valueSuggestions[selectedValueIndex].value);
			}
		} else if (event.key === 'Escape') {
			event.preventDefault();
			showValueSuggestions = false;
			selectedValueIndex = -1;
		}
	}

	onMount(() => {
		if (query) {
			rawQuery = query;
			if (mode === 'builder') {
				parseQueryToBuilder(query);
			}
		}
	});
</script>

<div
	class="query-builder bg-gradient-to-br from-white to-gray-50 dark:from-gray-800 dark:to-gray-850 rounded-xl border border-gray-200 dark:border-gray-700 shadow-lg overflow-hidden"
>
	<button
		onclick={() => (expanded = !expanded)}
		class="w-full flex items-center justify-between px-4 py-3 bg-white/80 dark:bg-gray-800/80 backdrop-blur-sm border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors"
	>
		<div class="flex items-center gap-3">
			<div
				class="p-1.5 bg-gradient-to-br from-earthy-terracotta-500 to-earthy-terracotta-600 rounded-lg"
			>
				<IconifyIcon icon="mdi:database-search" class="text-white w-4 h-4" />
			</div>
			<div class="text-left">
				<span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
					{expanded ? 'Hide' : 'Show'} Advanced Search
				</span>
				{#if !expanded && rawQuery}
					<p
						class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-400 font-mono mt-0.5"
					>
						{rawQuery}
					</p>
				{/if}
			</div>
		</div>
		<div class="flex items-center gap-2">
			{#if expanded}
				<button
					onclick={(e) => {
						e.stopPropagation();
						toggleMode();
					}}
					class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg shadow-sm transition-colors bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-800 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600"
				>
					<IconifyIcon
						icon={mode === 'builder' ? 'mdi:code-braces' : 'mdi:tune-variant'}
						class="w-4 h-4"
					/>
					{mode === 'builder' ? 'Code' : 'Builder'}
				</button>
			{/if}
			<IconifyIcon
				icon={expanded ? 'mdi:chevron-up' : 'mdi:chevron-down'}
				class="text-gray-500 w-5 h-5"
			/>
		</div>
	</button>

	{#if expanded}
		{#if mode === 'builder'}
			<div class="p-4 space-y-3">
				<!-- Free text search (collapsible) -->
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
				>
					<button
						onclick={() => (freeTextExpanded = !freeTextExpanded)}
						class="w-full flex items-center justify-between px-3 py-2.5 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors"
					>
						<div class="flex items-center gap-2">
							<IconifyIcon icon="mdi:text-search" class="w-4 h-4" />
							<span class="text-sm font-medium text-gray-700 dark:text-gray-300"
								>Full-Text Search</span
							>
							<span class="text-xs text-gray-500 dark:text-gray-400">(Optional)</span>
						</div>
						<IconifyIcon
							icon={freeTextExpanded ? 'mdi:chevron-up' : 'mdi:chevron-down'}
							class="w-4 h-4"
						/>
					</button>
					{#if freeTextExpanded}
						<div class="px-3 pb-3">
							<input
								type="text"
								bind:value={freeText}
								placeholder="Search across all fields..."
								class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
							/>
						</div>
					{/if}
				</div>

				<!-- Field filters (collapsible) -->
				<div
					class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden"
				>
					<button
						onclick={() => (filtersExpanded = !filtersExpanded)}
						class="w-full flex items-center justify-between px-3 py-2.5 hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors"
					>
						<div class="flex items-center gap-2">
							<IconifyIcon icon="mdi:filter-variant" class="w-4 h-4" />
							<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Field Filters</span
							>
							<span
								class="text-xs px-2 py-0.5 bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 rounded-full font-medium"
							>
								{filters.filter((f) => f.field && f.value).length}
							</span>
						</div>
						<IconifyIcon
							icon={filtersExpanded ? 'mdi:chevron-up' : 'mdi:chevron-down'}
							class="w-4 h-4"
						/>
					</button>
					{#if filtersExpanded}
						<div class="px-3 pb-3 space-y-2">
							{#each filters as filter, index (filter.id)}
								<div
									class="flex items-start gap-2 bg-gray-50 dark:bg-gray-750 p-2 rounded-lg border border-gray-200 dark:border-gray-700"
								>
									{#if index > 0 || freeText.trim()}
										<select
											bind:value={filter.booleanOp}
											class="px-2 py-2 text-xs font-semibold border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-600 transition-all"
										>
											{#each booleanOperators as op}
												<option value={op}>{op}</option>
											{/each}
										</select>
									{/if}

									<div class="relative flex-1">
										<input
											type="text"
											bind:value={filter.field}
											onfocus={(e) => handleFieldFocus(index, e.currentTarget)}
											onblur={handleFieldBlur}
											onkeydown={(e) => handleFieldKeydown(e, index)}
											placeholder="Field (e.g., kind, type, metadata.owner)"
											class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-600 focus:border-transparent transition-all font-mono"
										/>
									</div>

									<div class="relative">
										<button
											id="operator-btn-{index}"
											type="button"
											onclick={(e) => handleOperatorFocus(index, e.currentTarget)}
											onblur={handleOperatorBlur}
											onkeydown={(e) => handleOperatorKeydown(e, index)}
											class="px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-600 focus:border-transparent transition-all flex items-center justify-between gap-2 min-w-[140px]"
										>
											<span
												>{operators.find((op) => op.value === filter.operator)?.label ||
													'Select...'}</span
											>
											<IconifyIcon icon="mdi:chevron-down" class="w-4 h-4 text-gray-500" />
										</button>
									</div>

									{#if filter.operator === 'range'}
										<div class="flex items-center gap-2 flex-1">
											<input
												type="text"
												bind:value={filter.rangeFrom}
												placeholder="From"
												class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
											/>
											<span class="text-xs text-gray-500 dark:text-gray-400 font-semibold">TO</span>
											<input
												type="text"
												bind:value={filter.rangeTo}
												placeholder="To"
												class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
											/>
										</div>
									{:else}
										<div class="relative flex-1">
											<input
												id="value-input-{index}"
												type="text"
												bind:value={filter.value}
												onfocus={(e) => handleValueFocus(index, e.currentTarget)}
												onblur={handleValueBlur}
												oninput={() => handleValueInput(index)}
												onkeydown={(e) => handleValueKeydown(e, index)}
												placeholder="Value (use * for wildcards)"
												class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-600 focus:border-transparent transition-all"
											/>
										</div>
									{/if}

									<button
										onclick={() => removeFilter(filter.id)}
										class="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-all"
										title="Remove filter"
									>
										<IconifyIcon icon="mdi:close-circle" class="w-4 h-4" />
									</button>
								</div>
							{/each}

							{#if showFieldSuggestions && activeFieldIndex !== null}
								{@const currentIndex = activeFieldIndex}
								{@const filter = filters[currentIndex]}
								{@const filteredSuggestions = getFilteredSuggestions(filter.field)}
								{@const simpleFiltered = simpleFields.filter(
									(f) =>
										f.value.toLowerCase().includes(filter.field.toLowerCase()) ||
										f.label.toLowerCase().includes(filter.field.toLowerCase())
								)}
								{@const metadataFiltered = metadataFields.filter(
									(f) =>
										f.value.toLowerCase().includes(filter.field.toLowerCase()) ||
										f.label.toLowerCase().includes(filter.field.toLowerCase())
								)}
								<div
									class="fixed z-[9999] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-2xl max-h-72 overflow-auto"
									style="left: {dropdownPosition.left}px; top: {dropdownPosition.top}px; width: {dropdownPosition.width}px;"
								>
									{#if filteredSuggestions.length === 0}
										<div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
											No suggestions found
										</div>
									{:else}
										{#if simpleFiltered.length > 0}
											<div
												class="p-2 bg-gray-50 dark:bg-gray-750 border-b border-gray-200 dark:border-gray-700 sticky top-0"
											>
												<span
													class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wide"
													>Simple Fields</span
												>
											</div>
											{#each simpleFiltered as field, idx}
												{@const globalIdx = idx}
												<button
													onclick={() => selectField(currentIndex, field.value)}
													class="w-full text-left px-3 py-2 text-sm text-gray-900 dark:text-gray-100 transition-colors border-b border-gray-100 dark:border-gray-700 {selectedSuggestionIndex ===
													globalIdx
														? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40 suggestion-active'
														: 'hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20'}"
												>
													<div class="flex flex-col gap-1">
														<span
															class="font-mono text-sm font-semibold text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
															>{field.label}</span
														>
														<span class="text-xs text-gray-500 dark:text-gray-400"
															>{field.description}</span
														>
													</div>
												</button>
											{/each}
										{/if}

										{#if metadataFiltered.length > 0}
											<div
												class="p-2 bg-gray-50 dark:bg-gray-750 border-b border-gray-200 dark:border-gray-700 sticky top-0"
											>
												<span
													class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wide"
													>Metadata Fields</span
												>
											</div>
											{#each metadataFiltered as field, idx}
												{@const globalIdx = simpleFiltered.length + idx}
												<button
													onclick={() => selectField(currentIndex, field.value)}
													class="w-full text-left px-3 py-2 text-sm text-gray-900 dark:text-gray-100 transition-colors border-b border-gray-100 dark:border-gray-700 last:border-b-0 {selectedSuggestionIndex ===
													globalIdx
														? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40 suggestion-active'
														: 'hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20'}"
												>
													<div class="flex flex-col gap-1">
														<span
															class="font-mono text-sm font-semibold text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
															>{field.label}</span
														>
														<span class="text-xs text-gray-500 dark:text-gray-400"
															>{field.description}</span
														>
													</div>
												</button>
											{/each}
										{/if}
									{/if}
								</div>
							{/if}

							{#if showValueSuggestions && activeValueIndex !== null}
								<div
									class="fixed z-[9999] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-2xl max-h-60 overflow-auto"
									style="left: {valueDropdownPosition.left}px; top: {valueDropdownPosition.top}px; width: {valueDropdownPosition.width}px;"
								>
									{#if valueSuggestions.length === 0}
										<div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
											No suggestions found
										</div>
									{:else}
										{#each valueSuggestions as suggestion, idx}
											<button
												onclick={() => selectValue(activeValueIndex, suggestion.value)}
												class="w-full text-left px-3 py-2 text-sm text-gray-900 dark:text-gray-100 transition-colors border-b border-gray-100 dark:border-gray-700 last:border-b-0 {selectedValueIndex ===
												idx
													? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40'
													: 'hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20'}"
											>
												<span class="font-mono text-sm">{suggestion.value}</span>
											</button>
										{/each}
									{/if}
								</div>
							{/if}

							{#if showOperatorDropdown && activeOperatorIndex !== null}
								<div
									class="fixed z-[9999] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-2xl max-h-60 overflow-auto"
									style="left: {operatorDropdownPosition.left}px; top: {operatorDropdownPosition.top}px; width: {operatorDropdownPosition.width}px;"
								>
									{#each operators as op, idx}
										<button
											onclick={() => selectOperator(activeOperatorIndex, op.value)}
											class="w-full text-left px-3 py-2 text-sm text-gray-900 dark:text-gray-100 transition-colors border-b border-gray-100 dark:border-gray-700 last:border-b-0 {selectedOperatorIndex ===
											idx
												? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/40'
												: 'hover:bg-earthy-terracotta-50 dark:hover:bg-earthy-terracotta-900/20'}"
										>
											<span class="font-medium">{op.label}</span>
										</button>
									{/each}
								</div>
							{/if}

							<button
								onclick={addFilter}
								class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-lg shadow-sm transition-colors bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-800 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600"
							>
								<IconifyIcon icon="mdi:plus-circle" class="w-4 h-4" />
								Add Filter
							</button>
						</div>
					{/if}
				</div>
			</div>
		{:else}
			<div class="p-4">
				<div class="space-y-3">
					<div class="flex items-center justify-between">
						<label
							class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300"
						>
							<IconifyIcon icon="mdi:code-tags" class="w-4 h-4" />
							Raw Query
						</label>
					</div>
					<QueryInput
						bind:value={rawQuery}
						placeholder={`e.g @kind = "asset" AND @type = "table"`}
						onQueryChange={(q) => (rawQuery = q)}
						onSubmit={applyQuery}
					/>
				</div>
			</div>
		{/if}

		<div
			class="flex items-center justify-between px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-gradient-to-r from-gray-50 to-gray-100 dark:from-gray-900 dark:to-gray-850"
		>
			<a
				href="https://marmotdata.io/docs/queries/"
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-gray-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-300 transition-colors"
			>
				<IconifyIcon icon="mdi:book-open-variant" class="w-4 h-4" />
				View Documentation
			</a>
			<button
				onclick={applyQuery}
				class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-lg shadow-sm transition-colors bg-earthy-terracotta-700 dark:bg-earthy-terracotta-700 text-white hover:bg-earthy-terracotta-800 dark:hover:bg-earthy-terracotta-800 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600"
			>
				<IconifyIcon icon="mdi:play-circle" class="w-4 h-4" />
				Run Query
			</button>
		</div>
	{/if}
</div>

<style>
	.query-builder {
		max-width: 100%;
	}
</style>
