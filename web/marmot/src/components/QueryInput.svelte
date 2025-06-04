<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { fetchApi } from '$lib/api';
	import type { MetadataFieldSuggestion, MetadataValueSuggestion } from '$lib/assets/types';

	const operators = [
		{ value: ':', display: 'equals (:)', type: 'operator' },
		{ value: '>', display: 'greater than (>)', type: 'operator' },
		{ value: '<', display: 'less than (<)', type: 'operator' },
		{ value: '>=', display: 'greater equals (>=)', type: 'operator' },
		{ value: '<=', display: 'less equals (<=)', type: 'operator' },
		{ value: 'contains', display: 'contains', type: 'operator' },
		{ value: 'range', display: 'range', type: 'operator' }
	];

	let metadataFieldsCache: MetadataFieldSuggestion[] | null = null;

	export let value = '';
	export let placeholder = 'Search assets...';
	export let isLoading = false;
	export let onQueryChange: (query: string) => void = () => {};
	export let onSubmit: () => void = () => {};
	export let showDropdown = false;

	let input: HTMLTextAreaElement;
	let debounceTimer: NodeJS.Timeout;
	let dropdownContainer: HTMLDivElement;
	let suggestions: { type: string; value: string; display: string; count?: number }[] = [];
	let selectedIndex = -1;
	let suggestionStartPos = 0;

	const tokenColors: { [key: string]: string } = {
		field: 'text-blue-500',
		value: 'text-green-600',
		operator: 'text-purple-500',
		boolean: 'text-orange-500'
	};

	let clickOutsideHandler: (event: MouseEvent) => void;

	function debounce<T extends (...args: any[]) => any>(
		fn: T,
		delay: number
	): (...args: Parameters<T>) => Promise<ReturnType<T>> {
		return (...args: Parameters<T>) => {
			return new Promise((resolve) => {
				clearTimeout(debounceTimer);
				debounceTimer = setTimeout(() => resolve(fn(...args)), delay);
			});
		};
	}

	function calculatePixelOffset(position: number): { top: number; left: number } {
		if (!input) return { top: 0, left: 0 };

		const tempDiv = document.createElement('div');
		const inputStyle = window.getComputedStyle(input);

		tempDiv.style.cssText = inputStyle.cssText;
		tempDiv.style.position = 'absolute';
		tempDiv.style.visibility = 'hidden';
		tempDiv.style.height = 'auto';
		tempDiv.style.whiteSpace = 'pre-wrap';
		tempDiv.style.wordWrap = 'break-word';

		document.body.appendChild(tempDiv);

		const textBefore = value.substring(0, position);
		tempDiv.textContent = textBefore;

		const inputRect = input.getBoundingClientRect();

		const lines = textBefore.split('\n');
		const lastLine = lines[lines.length - 1];

		const tempLine = document.createElement('span');
		tempLine.style.cssText = inputStyle.cssText;
		tempLine.style.position = 'absolute';
		tempLine.style.visibility = 'hidden';
		tempLine.style.whiteSpace = 'pre';
		tempLine.textContent = lastLine;

		document.body.appendChild(tempLine);
		const lineRect = tempLine.getBoundingClientRect();

		const top = inputRect.top + window.scrollY + inputRect.height + 4;
		const left = inputRect.left + lineRect.width + parseFloat(inputStyle.paddingLeft);

		document.body.removeChild(tempDiv);
		document.body.removeChild(tempLine);

		return { top, left };
	}

	function getHighlightedText(text: string): { text: string; class: string }[] {
		const regex =
			/@metadata\.[a-zA-Z0-9_.]+|"[^"]*"|'[^']*'|[:=<>!]+|\b(AND|OR|NOT|contains|range)\b/g;
		const parts: { text: string; class: string }[] = [];
		let lastIndex = 0;
		let match;

		while ((match = regex.exec(text)) !== null) {
			if (match.index > lastIndex) {
				parts.push({
					text: text.slice(lastIndex, match.index),
					class: 'text-gray-900 dark:text-gray-100'
				});
			}

			const matchText = match[0];
			let colorClass = 'text-gray-900 dark:text-gray-100';

			if (matchText.startsWith('@metadata.')) {
				colorClass = tokenColors.field;
			} else if (matchText.match(/^["'][^"']*["']$/)) {
				colorClass = tokenColors.value;
			} else if (matchText.match(/[:=<>!]+|contains|range/)) {
				colorClass = tokenColors.operator;
			} else if (matchText.match(/\b(AND|OR|NOT)\b/)) {
				colorClass = tokenColors.boolean;
			}

			parts.push({
				text: matchText,
				class: colorClass
			});

			lastIndex = match.index + matchText.length;
		}

		if (lastIndex < text.length) {
			parts.push({
				text: text.slice(lastIndex),
				class: 'text-gray-900 dark:text-gray-100'
			});
		}

		return parts;
	}

	function getOperatorAtPosition(
		value: string,
		position: number
	): { startPosition: number } | null {
		const beforeCursor = value.substring(0, position);
		const lastMetadataField = beforeCursor.lastIndexOf('@metadata.');

		if (lastMetadataField === -1) return null;

		const afterMetadata = value.substring(lastMetadataField);
		const fieldMatch = afterMetadata.match(/^@metadata\.([a-zA-Z0-9_.]*)\s*$/);

		if (!fieldMatch) return null;

		return {
			startPosition: lastMetadataField + fieldMatch[0].length
		};
	}

	function getMetadataFieldAtPosition(
		value: string,
		position: number
	): {
		field: string | null;
		startPosition: number;
		hasOperator: boolean;
		valuePrefix?: string;
		needsOperator?: boolean;
	} {
		const metadataMatches = Array.from(value.matchAll(/@metadata\./g));
		let relevantMetadataMatch = null;
		for (const match of metadataMatches) {
			if (
				match.index !== undefined &&
				position >= match.index &&
				position <= match.index + '@metadata.'.length
			) {
				relevantMetadataMatch = match;
				break;
			}
		}

		if (relevantMetadataMatch) {
			return {
				field: '',
				startPosition: relevantMetadataMatch.index,
				hasOperator: false
			};
		}

		let currentMatch = null;
		for (const match of metadataMatches) {
			if (match.index !== undefined && match.index < position) {
				currentMatch = match;
			}
		}

		if (!currentMatch || currentMatch.index === undefined) {
			return { field: null, startPosition: 0, hasOperator: false };
		}

		const metadataStart = currentMatch.index;
		const afterMetadata = value.substring(metadataStart);

		const fieldMatch = afterMetadata.match(/^@metadata\.([a-zA-Z0-9_.]*)/);
		if (!fieldMatch) {
			return { field: null, startPosition: 0, hasOperator: false };
		}

		const field = fieldMatch[1];
		const fieldEnd = fieldMatch[0];
		const afterField = afterMetadata.substring(fieldEnd.length);
		const hasOperator = /\s*[:<>=]+\s*/.test(afterField);
		const needsOperator = !hasOperator && afterField.startsWith(' ');

		if (hasOperator) {
			const operatorMatch = afterField.match(/\s*[:<>=]+\s*/);
			if (operatorMatch) {
				const valueStart = metadataStart + fieldEnd.length + operatorMatch[0].length;
				const valueText = value.substring(valueStart, position);
				return {
					field,
					startPosition: valueStart,
					hasOperator: true,
					valuePrefix: valueText.trim()
				};
			}
		}

		return {
			field,
			startPosition: metadataStart + fieldEnd.length,
			hasOperator,
			needsOperator
		};
	}

	async function fetchMetadataFields(): Promise<MetadataFieldSuggestion[]> {
		if (metadataFieldsCache) {
			return metadataFieldsCache;
		}

		try {
			const response = await fetchApi('/assets/suggestions/metadata/fields');
			const data: MetadataFieldSuggestion[] = await response.json();
			metadataFieldsCache = data;
			return data;
		} catch (error) {
			console.error('Error fetching metadata fields:', error);
			return [];
		}
	}

	async function fetchMetadataValues(
		field: string,
		prefix: string
	): Promise<MetadataValueSuggestion[]> {
		try {
			const field_clean = field.split(/\s+/)[0].trim();
			const params = new URLSearchParams({
				field: field_clean,
				prefix: prefix.trim(),
				limit: '10'
			});
			const response = await fetchApi(`/assets/suggestions/metadata/values?${params}`);
			const data: MetadataValueSuggestion[] = await response.json();
			return data;
		} catch (error) {
			console.error('Error fetching metadata values:', error);
			return [];
		}
	}

	const debouncedFetchMetadataValues = debounce(fetchMetadataValues, 300);

	async function updateSuggestions() {
		if (!input) return;

		const cursorPos = input.selectionStart || 0;
		const fieldInfo = getMetadataFieldAtPosition(value, cursorPos);

		if (fieldInfo.field === null && !value.substring(0, cursorPos).includes('@metadata.')) {
			showDropdown = false;
			return;
		}

		suggestionStartPos = fieldInfo.startPosition;
		selectedIndex = -1;
		suggestions = [];

		if (fieldInfo.needsOperator) {
			suggestions = operators;
			showDropdown = true;
			return;
		}

		if (!fieldInfo.hasOperator) {
			const fields = await fetchMetadataFields();
			let searchPath = fieldInfo.field ? fieldInfo.field.toLowerCase() : '';
			const parts = searchPath.split('.');

			if (parts.length === 1 && parts[0] === '') {
				suggestions = fields
					.filter((f) => f.path_parts.length > 0)
					.map((f) => ({
						type: f.types[0],
						value: f.path_parts[0],
						display: f.path_parts[0],
						count: f.count
					}))
					.filter(
						(suggestion, index, self) =>
							index === self.findIndex((s) => s.display === suggestion.display)
					)
					.sort((a, b) => (b.count || 0) - (a.count || 0));
			} else {
				const searchPathClean = searchPath.endsWith('.') ? searchPath.slice(0, -1) : searchPath;

				const filteredFields = fields.filter((f) => {
					const fieldId = f.field.toLowerCase();
					return fieldId.startsWith(searchPathClean + '.') && fieldId !== searchPathClean;
				});

				if (filteredFields.length === 0 && searchPath.endsWith('.')) {
					const parentField = fields.find((f) => f.field.toLowerCase() === searchPathClean);
					if (!parentField || parentField.type !== 'object') {
						suggestions = operators;
					}
				} else {
					suggestions = filteredFields
						.map((f) => {
							const remainingPath = f.field.substring(searchPathClean.length + 1);
							const nextPart = remainingPath.split('.')[0];
							const isObject =
								f.types[0] === 'object' ||
								fields.some((field) =>
									field.field.toLowerCase().startsWith(f.field.toLowerCase() + '.')
								);
							return {
								type: isObject ? 'object' : 'value',
								value: searchPathClean + '.' + nextPart,
								display: nextPart,
								count: f.count
							};
						})
						.filter((s) => s.display)
						.filter(
							(suggestion, index, self) =>
								index === self.findIndex((s) => s.display === suggestion.display)
						)
						.sort((a, b) => (b.count || 0) - (a.count || 0));

					if (!searchPath.endsWith('.') && parts[parts.length - 1]) {
						const lastPart = parts[parts.length - 1].toLowerCase();
						suggestions = suggestions.filter((suggestion) =>
							suggestion.display.toLowerCase().startsWith(lastPart)
						);
					}
				}
			}

			if (searchPath.endsWith('.')) {
				showDropdown = suggestions.length > 0;
				return;
			}
		} else if (fieldInfo.hasOperator) {
			const prefix = fieldInfo.valuePrefix || '';
			const values = await debouncedFetchMetadataValues(fieldInfo.field!, prefix);
			if (values) {
				suggestions = values
					.filter((v) => v.value !== null)
					.map((v) => ({
						type: 'value',
						value: v.value,
						display: v.value
					}));
			}
		}

		showDropdown = suggestions.length > 0;
	}

	function adjustTextareaHeight() {
		if (!input) return;
		input.style.height = 'auto';
		input.style.height = `${input.scrollHeight}px`;
	}

	function handleInput() {
		adjustTextareaHeight();
		updateSuggestions();

		if (!isIncompleteMetadataQuery(value)) {
			onQueryChange(value);
		}
	}

	function isIncompleteMetadataQuery(query: string): boolean {
		if (!query.includes('@metadata.')) return false;

		const lastMetadataIndex = query.lastIndexOf('@metadata.');
		const restOfQuery = query.slice(lastMetadataIndex);

		if (restOfQuery.match(/@metadata\.[a-zA-Z0-9_.]+\s*[:<>=]+\s*[^:\s]+/)) {
			return false;
		}

		if (restOfQuery.startsWith('@metadata.')) {
			return (
				!restOfQuery.includes(':') ||
				/:\s*$/.test(restOfQuery) ||
				restOfQuery === '@metadata.' ||
				restOfQuery.split(':')[1].trim() === ''
			);
		}

		return false;
	}

	function isValidQuery(query: string): boolean {
		if (!query) return true;

		if (query.includes('@metadata.')) {
			const parts = query.split(' ');
			for (const part of parts) {
				if (part.startsWith('@metadata.')) {
					if (!part.includes(':') || part.endsWith(':') || part === '@metadata.') {
						return false;
					}

					const [, value] = part.split(':');
					if (!value || value.trim() === '') {
						return false;
					}
				}
			}
		}

		return true;
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();

			if (showDropdown && selectedIndex >= 0 && suggestions[selectedIndex]) {
				applySuggestion(suggestions[selectedIndex]);
				return;
			}

			if (value.trim()) {
				onSubmit();
				showDropdown = false;
			}
			return;
		}

		if (!showDropdown || suggestions.length === 0) return;

		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				selectedIndex = (selectedIndex + 1) % suggestions.length;
				scrollSelectedIntoView();
				break;
			case 'ArrowUp':
				event.preventDefault();
				selectedIndex = selectedIndex <= 0 ? suggestions.length - 1 : selectedIndex - 1;
				scrollSelectedIntoView();
				break;
			case 'Escape':
				event.preventDefault();
				showDropdown = false;
				break;
			case 'Tab':
				event.preventDefault();
				if (selectedIndex >= 0) {
					applySuggestion(suggestions[selectedIndex]);
				}
				break;
		}
	}

	function scrollSelectedIntoView() {
		if (!dropdownContainer) return;

		const selectedElement = dropdownContainer.children[selectedIndex] as HTMLElement;
		if (selectedElement) {
			selectedElement.scrollIntoView({
				block: 'nearest',
				behavior: 'smooth'
			});
		}
	}

	function handleSubmit() {
		showDropdown = false;
		onSubmit();
	}

	function applySuggestion(suggestion: { type: string; value: string; display: string }) {
		if (!input) return;

		const cursorPos = input.selectionStart || 0;
		const fieldInfo = getMetadataFieldAtPosition(value, cursorPos);

		if (suggestion.type === 'operator') {
			const beforeCursor = value.substring(0, fieldInfo.startPosition);
			const afterCursor = value.substring(cursorPos);
			value = `${beforeCursor} ${suggestion.value} ${afterCursor}`;
			const newCursorPos = beforeCursor.length + suggestion.value.length + 2;
			requestAnimationFrame(() => {
				if (input) {
					input.setSelectionRange(newCursorPos, newCursorPos);
					input.focus();
				}
			});
			return;
		}

		if (!fieldInfo.hasOperator) {
			const metadataPrefix = '@metadata.';
			const lastMetadataIndex = value.lastIndexOf(metadataPrefix);
			const beforeMetadata = value.substring(0, lastMetadataIndex);
			const afterMetadata = value.substring(lastMetadataIndex + metadataPrefix.length);

			const currentField = afterMetadata.split(/[\s:]/)[0];

			value =
				beforeMetadata +
				metadataPrefix +
				suggestion.value +
				(suggestion.type === 'object' ? '.' : '') +
				value.substring(cursorPos);

			const newCursorPos =
				beforeMetadata.length +
				metadataPrefix.length +
				suggestion.value.length +
				(suggestion.type === 'object' ? 1 : 0);
			requestAnimationFrame(() => {
				if (input) {
					input.setSelectionRange(newCursorPos, newCursorPos);
					input.focus();
				}
			});
		} else if (fieldInfo.hasOperator && suggestion.type === 'value') {
			const beforeValue = value.substring(0, fieldInfo.startPosition);
			const afterValue = value.substring(cursorPos);
			const newValue = ` "${suggestion.value}"`;
			value = beforeValue + newValue + afterValue;
			const newCursorPos = beforeValue.length + newValue.length;
			requestAnimationFrame(() => {
				if (input) {
					input.setSelectionRange(newCursorPos, newCursorPos);
					input.focus();
				}
			});
		}

		if (!isIncompleteMetadataQuery(value)) {
			onQueryChange(value);
		}
		showDropdown = false;
	}

	onMount(() => {
		fetchMetadataFields();

		clickOutsideHandler = (event: MouseEvent) => {
			if (
				dropdownContainer &&
				!dropdownContainer.contains(event.target as Node) &&
				input &&
				!input.contains(event.target as Node)
			) {
				showDropdown = false;
			}
		};
		document.addEventListener('click', clickOutsideHandler);
	});

	onDestroy(() => {
		if (clickOutsideHandler) {
			document.removeEventListener('click', clickOutsideHandler);
		}
	});
</script>

<div class="relative w-full">
	<div class="relative">
		<div
			class="absolute inset-0 pointer-events-none px-3 py-2 font-mono break-words whitespace-pre-wrap"
			aria-hidden="true"
		>
			{#each getHighlightedText(value) as part}
				<span class={part.class}>{part.text}</span>
			{/each}
		</div>
		<textarea
			bind:this={input}
			bind:value
			on:input={handleInput}
			on:keydown={handleKeydown}
			{placeholder}
			rows="1"
			class="w-full px-3 py-2 bg-transparent border border-gray-200 dark:border-gray-700 dark:border-gray-700 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent font-mono text-transparent caret-gray-900 break-words whitespace-pre-wrap resize-none overflow-hidden"
			autocomplete="off"
			spellcheck="false"
		/>
	</div>

	{#if showDropdown && suggestions.length > 0}
		<div
			bind:this={dropdownContainer}
			class="fixed z-50 bg-earthy-brown-50 dark:bg-gray-900 dark:bg-gray-900 rounded-lg border border-earthy-brown-100 shadow-lg dark:shadow-lg-white overflow-y-auto max-h-[280px]"
			style="left: {calculatePixelOffset(suggestionStartPos).left}px; top: {calculatePixelOffset(
				suggestionStartPos
			).top}px; min-width: 200px; width: auto; max-width: 400px;"
		>
			{#each suggestions as suggestion, i}
				<button
					class="w-full px-3 py-2 text-left hover:bg-earthy-brown-100 text-sm text-gray-900 first:rounded-t-lg last:rounded-b-lg break-words whitespace-normal {i ===
					selectedIndex
						? 'bg-earthy-brown-200'
						: ''}"
					on:click={() => applySuggestion(suggestion)}
					on:mouseenter={() => (selectedIndex = i)}
				>
					<span class="font-mono">{suggestion.display}</span>
				</button>
			{/each}
		</div>
	{/if}

	{#if isLoading}
		<div class="absolute right-3 top-2.5">
			<div
				class="animate-spin h-5 w-5 border-2 border-gray-300 dark:border-gray-600 dark:border-gray-600 border-t-blue-500 rounded-full"
			/>
		</div>
	{/if}
</div>
