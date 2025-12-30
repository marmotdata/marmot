<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Editor, textblockTypeInputRule } from '@tiptap/core';
	import StarterKit from '@tiptap/starter-kit';
	import Placeholder from '@tiptap/extension-placeholder';
	import Link from '@tiptap/extension-link';
	import Typography from '@tiptap/extension-typography';
	import Image from '@tiptap/extension-image';
	import TextAlign from '@tiptap/extension-text-align';
	import { Table } from '@tiptap/extension-table';
	import { TableRow } from '@tiptap/extension-table-row';
	import { TableCell } from '@tiptap/extension-table-cell';
	import { TableHeader } from '@tiptap/extension-table-header';
	import { CodeBlockLowlight } from '@tiptap/extension-code-block-lowlight';
	import { common, createLowlight } from 'lowlight';
	import { marked } from 'marked';
	import TurndownService from 'turndown';
	import { gfm } from '@truto/turndown-plugin-gfm';
	import Icon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';

	// Regex to match ```language at start of line
	const backtickInputRegex = /^```([a-z]+)?[\s\n]$/;

	// Extended CodeBlockLowlight with markdown input rule for ```language
	const CustomCodeBlock = CodeBlockLowlight.extend({
		addInputRules() {
			return [
				textblockTypeInputRule({
					find: backtickInputRegex,
					type: this.type,
					getAttributes: (match) => ({
						language: match[1] || 'plaintext'
					})
				})
			];
		}
	});

	interface Props {
		value?: string;
		placeholder?: string;
		disabled?: boolean;
		pageId?: string | null;
		onImageUpload?: (imageUrl: string) => void;
	}

	let {
		value = $bindable(''),
		placeholder = 'Start typing...',
		disabled = false,
		pageId = null,
		onImageUpload = undefined
	}: Props = $props();

	let editor: Editor | null = null;
	let element: HTMLElement;
	let turndownService: TurndownService;
	let isUpdating = false;
	let isUploading = false;
	let uploadError = '';
	let fileInput: HTMLInputElement;

	// Separate counters for different types of updates to minimize re-renders
	let selectionVersion = $state(0); // Only for toolbar state updates
	let lastExternalValue = ''; // Track external value changes

	// Create lowlight instance with common languages
	const lowlight = createLowlight(common);

	onMount(() => {
		turndownService = new TurndownService({
			headingStyle: 'atx',
			codeBlockStyle: 'fenced',
			emDelimiter: '*',
			strongDelimiter: '**'
		});

		// Add GFM (tables, strikethrough) support
		turndownService.use(gfm);

		// Add image rule to turndown
		turndownService.addRule('image', {
			filter: 'img',
			replacement: function (content, node) {
				const img = node as HTMLImageElement;
				const alt = img.alt || '';
				const src = img.src || '';
				const title = img.title ? ` "${img.title}"` : '';
				return `![${alt}](${src}${title})`;
			}
		});

		// Preserve text alignment by keeping aligned elements as HTML
		turndownService.addRule('alignedText', {
			filter: function (node) {
				if (!['P', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6'].includes(node.nodeName)) {
					return false;
				}
				const style = (node as HTMLElement).getAttribute('style') || '';
				return style.includes('text-align: center') || style.includes('text-align: right');
			},
			replacement: function (content, node) {
				const el = node as HTMLElement;
				const tag = el.nodeName.toLowerCase();
				const style = el.getAttribute('style') || '';
				return `\n<${tag} style="${style}">${content}</${tag}>\n`;
			}
		});

		// Custom rule for code blocks with language
		turndownService.addRule('codeBlock', {
			filter: function (node) {
				return node.nodeName === 'PRE' && node.firstChild?.nodeName === 'CODE';
			},
			replacement: function (content, node) {
				const codeNode = node.firstChild as HTMLElement;
				const className = codeNode?.className || '';
				const languageMatch = className.match(/language-(\w+)/);
				const language = languageMatch ? languageMatch[1] : '';
				const code = codeNode?.textContent || '';
				return `\n\`\`\`${language}\n${code}\n\`\`\`\n`;
			}
		});

		const initialHtml = value ? (marked(value) as string) : '';

		editor = new Editor({
			element: element,
			extensions: [
				StarterKit.configure({
					heading: {
						levels: [1, 2, 3]
					},
					// Disable default code block in favor of lowlight version
					codeBlock: false
				}),
				Placeholder.configure({
					placeholder: placeholder
				}),
				Link.configure({
					openOnClick: false,
					HTMLAttributes: {
						class: 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700 underline'
					}
				}),
				Typography,
				TextAlign.configure({
					types: ['heading', 'paragraph']
				}),
				Image.configure({
					inline: false,
					allowBase64: true,
					HTMLAttributes: {
						class: 'max-w-full h-auto rounded-lg my-4'
					}
				}),
				// Code block with syntax highlighting and markdown input rules
				CustomCodeBlock.configure({
					lowlight,
					defaultLanguage: 'plaintext',
					HTMLAttributes: {
						class: 'code-block'
					}
				}),
				// Table extensions
				Table.configure({
					resizable: true,
					HTMLAttributes: {
						class: 'doc-table'
					}
				}),
				TableRow,
				TableHeader,
				TableCell
			],
			content: initialHtml,
			editable: !disabled,
			onUpdate: ({ editor }) => {
				if (!isUpdating) {
					const html = editor.getHTML();
					const markdown = turndownService.turndown(html);
					value = markdown;
					lastExternalValue = markdown;
				}
			},
			onSelectionUpdate: () => {
				// Only update toolbar state on selection changes
				selectionVersion++;
			},
			editorProps: {
				attributes: {
					class:
						'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[200px] px-3 py-2'
				},
				handlePaste: (view, event) => {
					const items = event.clipboardData?.items;
					if (!items) return false;

					for (const item of items) {
						if (item.type.startsWith('image/')) {
							event.preventDefault();
							const file = item.getAsFile();
							if (file) {
								handleImageFile(file);
							}
							return true;
						}
					}
					return false;
				},
				handleDrop: (view, event) => {
					const files = event.dataTransfer?.files;
					if (!files || files.length === 0) return false;

					for (const file of files) {
						if (file.type.startsWith('image/')) {
							event.preventDefault();
							handleImageFile(file);
							return true;
						}
					}
					return false;
				}
			}
		});
	});

	onDestroy(() => {
		if (editor) {
			editor.destroy();
		}
	});

	$effect(() => {
		if (editor && !editor.isDestroyed) {
			editor.setEditable(!disabled);
		}
	});

	// Only sync content when value changes externally (not from our own edits)
	$effect(() => {
		if (
			editor &&
			!editor.isDestroyed &&
			value !== undefined &&
			value !== lastExternalValue &&
			!isUpdating
		) {
			isUpdating = true;
			lastExternalValue = value;
			const html = value ? (marked(value) as string) : '';
			editor.commands.setContent(html);
			isUpdating = false;
		}
	});

	async function handleImageFile(file: File) {
		if (!pageId) {
			// If no pageId, insert as base64 (temporary)
			const reader = new FileReader();
			reader.onload = (e) => {
				const dataUrl = e.target?.result as string;
				editor?.chain().focus().setImage({ src: dataUrl }).run();
			};
			reader.readAsDataURL(file);
			return;
		}

		// Validate file size (5MB max)
		if (file.size > 5 * 1024 * 1024) {
			uploadError = 'Image exceeds maximum size (5MB)';
			setTimeout(() => (uploadError = ''), 3000);
			return;
		}

		// Validate file type
		const validTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
		if (!validTypes.includes(file.type)) {
			uploadError = 'Invalid image type. Allowed: JPEG, PNG, GIF, WebP';
			setTimeout(() => (uploadError = ''), 3000);
			return;
		}

		isUploading = true;
		uploadError = '';

		try {
			// Convert to base64
			const base64 = await fileToBase64(file);

			// Upload to server
			const response = await fetchApi(`/docs/pages/${pageId}/images`, {
				method: 'POST',
				body: JSON.stringify({
					filename: file.name,
					content_type: file.type,
					data: base64
				})
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to upload image');
			}

			const imageMeta = await response.json();

			// Insert image with server URL
			editor?.chain().focus().setImage({ src: imageMeta.url, alt: file.name }).run();

			if (onImageUpload) {
				onImageUpload(imageMeta.url);
			}
		} catch (err) {
			uploadError = err instanceof Error ? err.message : 'Failed to upload image';
			setTimeout(() => (uploadError = ''), 3000);
		} finally {
			isUploading = false;
		}
	}

	function fileToBase64(file: File): Promise<string> {
		return new Promise((resolve, reject) => {
			const reader = new FileReader();
			reader.onload = () => {
				const result = reader.result as string;
				// Remove data URL prefix if present
				const base64 = result.includes(',') ? result.split(',')[1] : result;
				resolve(base64);
			};
			reader.onerror = reject;
			reader.readAsDataURL(file);
		});
	}

	function openFileDialog() {
		fileInput?.click();
	}

	function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		if (file) {
			handleImageFile(file);
		}
		input.value = '';
	}

	function toggleBold() {
		editor?.chain().focus().toggleBold().run();
	}

	function toggleItalic() {
		editor?.chain().focus().toggleItalic().run();
	}

	function toggleCode() {
		editor?.chain().focus().toggleCode().run();
	}

	function toggleHeading(level: 1 | 2 | 3) {
		editor?.chain().focus().toggleHeading({ level }).run();
	}

	function toggleBulletList() {
		editor?.chain().focus().toggleBulletList().run();
	}

	function toggleOrderedList() {
		editor?.chain().focus().toggleOrderedList().run();
	}

	function toggleBlockquote() {
		editor?.chain().focus().toggleBlockquote().run();
	}

	function toggleCodeBlock() {
		editor?.chain().focus().toggleCodeBlock().run();
	}

	function setTextAlign(alignment: 'left' | 'center' | 'right') {
		editor?.chain().focus().setTextAlign(alignment).run();
	}

	function setLink() {
		const url = window.prompt('Enter URL:');
		if (url) {
			editor?.chain().focus().setLink({ href: url }).run();
		}
	}

	function unsetLink() {
		editor?.chain().focus().unsetLink().run();
	}

	// Table functions
	function insertTable() {
		editor?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
	}

	function addColumnBefore() {
		editor?.chain().focus().addColumnBefore().run();
	}

	function addColumnAfter() {
		editor?.chain().focus().addColumnAfter().run();
	}

	function deleteColumn() {
		editor?.chain().focus().deleteColumn().run();
	}

	function addRowBefore() {
		editor?.chain().focus().addRowBefore().run();
	}

	function addRowAfter() {
		editor?.chain().focus().addRowAfter().run();
	}

	function deleteRow() {
		editor?.chain().focus().deleteRow().run();
	}

	function deleteTable() {
		editor?.chain().focus().deleteTable().run();
	}

	// Reactive toolbar state - derived from selection changes only
	// Check for tableCell since cursor is inside cells, not the table itself
	let isInTable = $derived(
		(() => {
			void selectionVersion;
			return (editor?.isActive('tableCell') || editor?.isActive('tableHeader')) ?? false;
		})()
	);

	let isBoldActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('bold') ?? false;
		})()
	);

	let isItalicActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('italic') ?? false;
		})()
	);

	let isCodeActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('code') ?? false;
		})()
	);

	let isHeading1Active = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('heading', { level: 1 }) ?? false;
		})()
	);

	let isHeading2Active = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('heading', { level: 2 }) ?? false;
		})()
	);

	let isHeading3Active = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('heading', { level: 3 }) ?? false;
		})()
	);

	let isBulletListActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('bulletList') ?? false;
		})()
	);

	let isOrderedListActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('orderedList') ?? false;
		})()
	);

	let isBlockquoteActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('blockquote') ?? false;
		})()
	);

	let isCodeBlockActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('codeBlock') ?? false;
		})()
	);

	let isLinkActive = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive('link') ?? false;
		})()
	);

	let isAlignLeft = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive({ textAlign: 'left' }) ?? true;
		})()
	);

	let isAlignCenter = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive({ textAlign: 'center' }) ?? false;
		})()
	);

	let isAlignRight = $derived(
		(() => {
			void selectionVersion;
			return editor?.isActive({ textAlign: 'right' }) ?? false;
		})()
	);
</script>

<input
	type="file"
	accept="image/jpeg,image/png,image/gif,image/webp"
	class="hidden"
	bind:this={fileInput}
	on:change={handleFileSelect}
/>

<div class="border border-gray-300 dark:border-gray-600 rounded-md {disabled ? 'opacity-50' : ''}">
	<!-- Toolbar -->
	<div
		class="flex flex-wrap items-center gap-1 p-2 border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/50"
	>
		<button
			type="button"
			on:click={toggleBold}
			{disabled}
			class="p-1.5 rounded {isBoldActive
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Bold (Ctrl+B)"
		>
			<Icon icon="material-symbols:format-bold" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleItalic}
			{disabled}
			class="p-1.5 rounded {isItalicActive
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Italic (Ctrl+I)"
		>
			<Icon icon="material-symbols:format-italic" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleCode}
			{disabled}
			class="p-1.5 rounded {isCodeActive
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Inline Code"
		>
			<Icon icon="material-symbols:code" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		<button
			type="button"
			on:click={() => toggleHeading(1)}
			{disabled}
			class="p-1.5 rounded {isHeading1Active
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed text-sm font-bold"
			title="Heading 1"
		>
			H1
		</button>

		<button
			type="button"
			on:click={() => toggleHeading(2)}
			{disabled}
			class="p-1.5 rounded {isHeading2Active
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed text-sm font-bold"
			title="Heading 2"
		>
			H2
		</button>

		<button
			type="button"
			on:click={() => toggleHeading(3)}
			{disabled}
			class="p-1.5 rounded {isHeading3Active
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed text-sm font-bold"
			title="Heading 3"
		>
			H3
		</button>

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		<button
			type="button"
			on:click={() => setTextAlign('left')}
			{disabled}
			class="p-1.5 rounded {isAlignLeft
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Align Left"
		>
			<Icon icon="material-symbols:format-align-left" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={() => setTextAlign('center')}
			{disabled}
			class="p-1.5 rounded {isAlignCenter
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Align Center"
		>
			<Icon icon="material-symbols:format-align-center" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={() => setTextAlign('right')}
			{disabled}
			class="p-1.5 rounded {isAlignRight
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Align Right"
		>
			<Icon icon="material-symbols:format-align-right" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		<button
			type="button"
			on:click={toggleBulletList}
			{disabled}
			class="p-1.5 rounded {isBulletListActive
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Bullet List"
		>
			<Icon icon="material-symbols:format-list-bulleted" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleOrderedList}
			{disabled}
			class="p-1.5 rounded {isOrderedListActive
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Numbered List"
		>
			<Icon icon="material-symbols:format-list-numbered" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		<button
			type="button"
			on:click={toggleBlockquote}
			{disabled}
			class="p-1.5 rounded {isBlockquoteActive
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Quote"
		>
			<Icon icon="material-symbols:format-quote" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleCodeBlock}
			{disabled}
			class="p-1.5 rounded {isCodeBlockActive
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Code Block"
		>
			<Icon icon="material-symbols:code-blocks" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		<!-- Table controls -->
		{#if isInTable}
			<div class="flex items-center gap-0.5">
				<button
					type="button"
					on:click={addColumnBefore}
					{disabled}
					class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
					title="Add column before"
				>
					<Icon icon="mdi:table-column-plus-before" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={addColumnAfter}
					{disabled}
					class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
					title="Add column after"
				>
					<Icon icon="mdi:table-column-plus-after" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={deleteColumn}
					{disabled}
					class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
					title="Delete column"
				>
					<Icon icon="mdi:table-column-remove" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={addRowBefore}
					{disabled}
					class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
					title="Add row above"
				>
					<Icon icon="mdi:table-row-plus-before" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={addRowAfter}
					{disabled}
					class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
					title="Add row below"
				>
					<Icon icon="mdi:table-row-plus-after" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={deleteRow}
					{disabled}
					class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
					title="Delete row"
				>
					<Icon icon="mdi:table-row-remove" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={deleteTable}
					{disabled}
					class="p-1.5 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-red-600 dark:text-red-400 disabled:opacity-50 disabled:cursor-not-allowed"
					title="Delete table"
				>
					<Icon icon="mdi:table-remove" class="h-4 w-4" />
				</button>
			</div>
		{:else}
			<button
				type="button"
				on:click={insertTable}
				{disabled}
				class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
				title="Insert Table"
			>
				<Icon icon="material-symbols:table" class="h-4 w-4" />
			</button>
		{/if}

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		{#if isLinkActive}
			<button
				type="button"
				on:click={unsetLink}
				{disabled}
				class="p-1.5 rounded bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700 disabled:opacity-50 disabled:cursor-not-allowed"
				title="Remove Link"
			>
				<Icon icon="material-symbols:link-off" class="h-4 w-4" />
			</button>
		{:else}
			<button
				type="button"
				on:click={setLink}
				{disabled}
				class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
				title="Add Link"
			>
				<Icon icon="material-symbols:link" class="h-4 w-4" />
			</button>
		{/if}

		<button
			type="button"
			on:click={openFileDialog}
			disabled={disabled || isUploading}
			class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
			title="Insert Image"
		>
			{#if isUploading}
				<Icon icon="mdi:loading" class="h-4 w-4 animate-spin" />
			{:else}
				<Icon icon="material-symbols:image" class="h-4 w-4" />
			{/if}
		</button>

		{#if uploadError}
			<span class="text-xs text-red-500 ml-2">{uploadError}</span>
		{/if}
	</div>

	<!-- Editor -->
	<div
		bind:this={element}
		class="bg-white dark:bg-gray-700 rounded-b-md text-gray-900 dark:text-gray-100"
	></div>
</div>

<style>
	:global(.ProseMirror) {
		outline: none;
	}

	:global(.ProseMirror p.is-editor-empty:first-child::before) {
		content: attr(data-placeholder);
		float: left;
		color: rgb(156 163 175);
		pointer-events: none;
		height: 0;
	}

	:global(.ProseMirror h1) {
		@apply text-2xl font-bold mt-4 mb-2;
	}

	:global(.ProseMirror h2) {
		@apply text-xl font-bold mt-3 mb-2;
	}

	:global(.ProseMirror h3) {
		@apply text-lg font-bold mt-2 mb-1;
	}

	:global(.ProseMirror ul) {
		@apply list-disc list-inside my-2;
	}

	:global(.ProseMirror ol) {
		@apply list-decimal list-inside my-2;
	}

	:global(.ProseMirror blockquote) {
		@apply border-l-4 border-gray-300 dark:border-gray-600 pl-4 italic my-2;
	}

	:global(.ProseMirror code) {
		@apply bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded text-sm font-mono;
	}

	:global(.ProseMirror pre) {
		@apply bg-gray-50 dark:bg-gray-800 p-6 rounded-lg my-3 overflow-x-auto;
		margin: 0;
	}

	:global(.ProseMirror pre code) {
		@apply bg-transparent p-0;
		font-family:
			ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New',
			monospace;
		font-size: 0.875rem;
		display: block;
		white-space: pre;
		width: max-content;
		min-width: 100%;
	}

	:global(.ProseMirror strong) {
		@apply font-bold;
	}

	:global(.ProseMirror em) {
		@apply italic;
	}

	:global(.ProseMirror img) {
		@apply max-w-full h-auto rounded-lg my-4 cursor-pointer;
	}

	:global(.ProseMirror img:hover) {
		@apply ring-2 ring-earthy-terracotta-500;
	}

	:global(.ProseMirror img.ProseMirror-selectednode) {
		@apply ring-2 ring-earthy-terracotta-500;
	}

	/* Table styles */
	:global(.ProseMirror table) {
		@apply w-full my-4 text-sm border border-gray-300 dark:border-gray-600 rounded-lg overflow-hidden;
		border-collapse: collapse;
	}

	:global(.ProseMirror th) {
		@apply bg-gray-100 dark:bg-gray-800 font-semibold text-left px-3 py-2.5 text-gray-900 dark:text-gray-100;
		border: 1px solid;
		@apply border-gray-300 dark:border-gray-600;
		border-right-width: 2px;
		@apply border-r-gray-400 dark:border-r-gray-500;
	}

	:global(.ProseMirror th:last-child) {
		border-right-width: 1px;
		@apply border-r-gray-300 dark:border-r-gray-600;
	}

	:global(.ProseMirror td) {
		@apply px-3 py-2.5 text-gray-700 dark:text-gray-300;
		border: 1px solid;
		@apply border-gray-300 dark:border-gray-600;
		border-right-width: 2px;
		@apply border-r-gray-400 dark:border-r-gray-500;
	}

	:global(.ProseMirror td:last-child) {
		border-right-width: 1px;
		@apply border-r-gray-300 dark:border-r-gray-600;
	}

	:global(.ProseMirror tr:nth-child(even) td) {
		@apply bg-gray-50 dark:bg-gray-800/30;
	}

	:global(.ProseMirror tr:hover td) {
		@apply bg-blue-50 dark:bg-blue-900/20;
	}

	:global(.ProseMirror .selectedCell) {
		@apply bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30;
		box-shadow: inset 0 0 0 2px rgb(var(--color-earthy-terracotta-500) / 0.5);
	}

	/* Column resize handle - more visible */
	:global(.ProseMirror .column-resize-handle) {
		@apply absolute top-0 bottom-0 right-0 pointer-events-none;
		width: 4px;
		background: #c9601c;
	}

	:global(.ProseMirror.resize-cursor) {
		cursor: col-resize;
	}

	/* Cells need position relative for resize handle */
	:global(.ProseMirror th),
	:global(.ProseMirror td) {
		position: relative;
	}

	/* Light theme - Earthy colors matching CodeBlock/Docusaurus */
	:global(.ProseMirror pre code) {
		color: #1f2937;
	}

	:global(.ProseMirror .hljs-comment),
	:global(.ProseMirror .hljs-quote) {
		color: #4a674a;
		font-style: italic;
	}

	:global(.ProseMirror .hljs-keyword),
	:global(.ProseMirror .hljs-selector-tag),
	:global(.ProseMirror .hljs-addition) {
		color: #8d3718;
	}

	:global(.ProseMirror .hljs-number),
	:global(.ProseMirror .hljs-string),
	:global(.ProseMirror .hljs-meta .hljs-meta-string),
	:global(.ProseMirror .hljs-literal),
	:global(.ProseMirror .hljs-doctag),
	:global(.ProseMirror .hljs-regexp) {
		color: #35593b;
	}

	:global(.ProseMirror .hljs-title),
	:global(.ProseMirror .hljs-section),
	:global(.ProseMirror .hljs-name),
	:global(.ProseMirror .hljs-selector-id),
	:global(.ProseMirror .hljs-selector-class) {
		color: #b34822;
	}

	:global(.ProseMirror .hljs-attribute),
	:global(.ProseMirror .hljs-attr),
	:global(.ProseMirror .hljs-variable),
	:global(.ProseMirror .hljs-template-variable),
	:global(.ProseMirror .hljs-class .hljs-title),
	:global(.ProseMirror .hljs-type) {
		color: #7b5935;
	}

	:global(.ProseMirror .hljs-symbol),
	:global(.ProseMirror .hljs-bullet),
	:global(.ProseMirror .hljs-subst),
	:global(.ProseMirror .hljs-meta),
	:global(.ProseMirror .hljs-meta .hljs-keyword),
	:global(.ProseMirror .hljs-selector-attr),
	:global(.ProseMirror .hljs-selector-pseudo),
	:global(.ProseMirror .hljs-link) {
		color: #7b5935;
	}

	:global(.ProseMirror .hljs-built_in),
	:global(.ProseMirror .hljs-deletion) {
		color: #b34822;
	}

	:global(.ProseMirror .hljs-punctuation),
	:global(.ProseMirror .hljs-operator) {
		color: #4a674a;
	}

	:global(.ProseMirror .hljs-emphasis) {
		font-style: italic;
	}

	:global(.ProseMirror .hljs-strong) {
		font-weight: bold;
	}

	/* Dark theme - Brighter earthy tones matching CodeBlock/Docusaurus */
	:global(.dark .ProseMirror pre code) {
		color: #f3f4f6;
	}

	:global(.dark .ProseMirror .hljs-comment),
	:global(.dark .ProseMirror .hljs-quote) {
		color: #a8c5a8;
		font-style: italic;
	}

	:global(.dark .ProseMirror .hljs-keyword),
	:global(.dark .ProseMirror .hljs-selector-tag),
	:global(.dark .ProseMirror .hljs-addition) {
		color: #ffa77d;
	}

	:global(.dark .ProseMirror .hljs-number),
	:global(.dark .ProseMirror .hljs-string),
	:global(.dark .ProseMirror .hljs-meta .hljs-meta-string),
	:global(.dark .ProseMirror .hljs-literal),
	:global(.dark .ProseMirror .hljs-doctag),
	:global(.dark .ProseMirror .hljs-regexp) {
		color: #b9d9b9;
	}

	:global(.dark .ProseMirror .hljs-title),
	:global(.dark .ProseMirror .hljs-section),
	:global(.dark .ProseMirror .hljs-name),
	:global(.dark .ProseMirror .hljs-selector-id),
	:global(.dark .ProseMirror .hljs-selector-class) {
		color: #ffb899;
	}

	:global(.dark .ProseMirror .hljs-attribute),
	:global(.dark .ProseMirror .hljs-attr),
	:global(.dark .ProseMirror .hljs-variable),
	:global(.dark .ProseMirror .hljs-template-variable),
	:global(.dark .ProseMirror .hljs-class .hljs-title),
	:global(.dark .ProseMirror .hljs-type) {
		color: #f0d97e;
	}

	:global(.dark .ProseMirror .hljs-symbol),
	:global(.dark .ProseMirror .hljs-bullet),
	:global(.dark .ProseMirror .hljs-subst),
	:global(.dark .ProseMirror .hljs-meta),
	:global(.dark .ProseMirror .hljs-meta .hljs-keyword),
	:global(.dark .ProseMirror .hljs-selector-attr),
	:global(.dark .ProseMirror .hljs-selector-pseudo),
	:global(.dark .ProseMirror .hljs-link) {
		color: #f0d97e;
	}

	:global(.dark .ProseMirror .hljs-built_in),
	:global(.dark .ProseMirror .hljs-deletion) {
		color: #ffb899;
	}

	:global(.dark .ProseMirror .hljs-punctuation),
	:global(.dark .ProseMirror .hljs-operator) {
		color: #d1e5d1;
	}
</style>
