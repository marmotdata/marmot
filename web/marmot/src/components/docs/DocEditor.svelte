<script lang="ts">
	import { onMount, onDestroy, mount, unmount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
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
	import Mention from '@tiptap/extension-mention';
	import { common, createLowlight } from 'lowlight';
	import { marked } from 'marked';
	import TurndownService from 'turndown';
	import { gfm } from '@truto/turndown-plugin-gfm';
	import Icon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import { mergeAttributes } from '@tiptap/core';
	import { docImagePath, loadDocImage } from '$lib/docs/images';
	import { m } from '$lib/paraglide/messages';
	import MentionList from '$components/editor/MentionList.svelte';

	// Debounce helper
	function debounce<T extends (...args: Parameters<T>) => ReturnType<T>>(
		fn: T,
		delay: number
	): (...args: Parameters<T>) => void {
		let timeoutId: ReturnType<typeof setTimeout>;
		return (...args: Parameters<T>) => {
			clearTimeout(timeoutId);
			timeoutId = setTimeout(() => fn(...args), delay);
		};
	}

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

	// Image node that loads doc images with the auth token for display while keeping the
	// canonical URL in the document, since <img> tags cannot authenticate themselves
	const DocImage = Image.extend({
		addNodeView() {
			const baseAttributes = this.options.HTMLAttributes;
			return ({ node }) => {
				const img = document.createElement('img');
				let currentSrc: string | null = null;

				const render = (attributes: Record<string, unknown>) => {
					for (const [key, val] of Object.entries(mergeAttributes(baseAttributes, attributes))) {
						if (key === 'src') continue;
						if (val === null || val === undefined) img.removeAttribute(key);
						else img.setAttribute(key, String(val));
					}

					const src = (attributes.src as string | null) ?? null;
					if (src === currentSrc) return;
					currentSrc = src;

					const path = src ? docImagePath(src) : null;
					if (!path) {
						if (src) img.src = src;
						else img.removeAttribute('src');
						return;
					}
					img.removeAttribute('src');
					loadDocImage(path)
						.then((objectUrl) => {
							if (currentSrc === src) img.src = objectUrl;
						})
						.catch(() => {});
				};

				render(node.attrs);

				return {
					dom: img,
					update: (updated) => {
						if (updated.type !== node.type) return false;
						render(updated.attrs);
						return true;
					}
				};
			};
		}
	});

	interface MentionItem {
		type: string;
		id: string;
		name: string;
		username?: string;
		profile_picture?: string;
	}

	function createMentionSuggestion() {
		// Cache for debounced search results
		let searchCache: SvelteMap<string, Array<MentionItem>> = new SvelteMap();

		// Debounced search function - searches both users and teams
		const searchOwners = async (query: string) => {
			if (searchCache.has(query)) {
				return searchCache.get(query)!;
			}
			try {
				const response = await fetchApi(`/owners/search?q=${encodeURIComponent(query)}&limit=10`);
				if (!response.ok) return [];
				const data = await response.json();
				const owners = data.owners || [];
				searchCache.set(query, owners);
				return owners;
			} catch {
				return [];
			}
		};

		return {
			items: async ({ query }: { query: string }) => {
				if (!query || query.length < 1) return [];
				// Return a promise that resolves after debounce
				return new Promise((resolve) => {
					const debouncedSearch = debounce(async (q: string) => {
						const results = await searchOwners(q);
						resolve(results);
					}, 200);
					debouncedSearch(query);
				});
			},
			render: () => {
				let component: ReturnType<typeof mount> | null = null;
				let popup: HTMLElement | null = null;

				const mountComponent = (
					items: Array<MentionItem>,
					command: (item: { id: string; label: string; type: string }) => void
				) => {
					if (!popup) return;

					// Unmount existing component if any
					if (component) {
						unmount(component);
						component = null;
					}

					component = mount(MentionList, {
						target: popup,
						props: { items, command }
					});
				};

				return {
					onStart: (props: {
						items: Array<MentionItem>;
						command: (item: { id: string; label: string; type: string }) => void;
						clientRect: () => DOMRect | null;
					}) => {
						popup = document.createElement('div');
						popup.className = 'mention-popup';

						const rect = props.clientRect?.();
						if (rect) {
							popup.style.position = 'fixed';
							popup.style.left = `${rect.left}px`;
							popup.style.top = `${rect.bottom + 4}px`;
							popup.style.zIndex = '9999';
						}

						document.body.appendChild(popup);
						mountComponent(props.items, props.command);
					},
					onUpdate: (props: {
						items: Array<MentionItem>;
						command: (item: { id: string; label: string; type: string }) => void;
						clientRect: () => DOMRect | null;
					}) => {
						if (popup) {
							const rect = props.clientRect?.();
							if (rect) {
								popup.style.left = `${rect.left}px`;
								popup.style.top = `${rect.bottom + 4}px`;
							}
							// Remount with new props
							mountComponent(props.items, props.command);
						}
					},
					onKeyDown: (props: { event: KeyboardEvent }) => {
						if (props.event.key === 'Escape') {
							if (popup) {
								popup.remove();
								popup = null;
							}
							return true;
						}
						// Call onKeyDown on the component instance
						const comp = component as { onKeyDown?: (e: KeyboardEvent) => boolean } | null;
						if (comp && typeof comp.onKeyDown === 'function') {
							return comp.onKeyDown(props.event);
						}
						return false;
					},
					onExit: () => {
						if (component) {
							unmount(component);
							component = null;
						}
						if (popup) {
							popup.remove();
							popup = null;
						}
						// Clear cache on exit
						searchCache.clear();
					}
				};
			}
		};
	}

	interface Props {
		value?: string;
		placeholder?: string;
		disabled?: boolean;
		pageId?: string | null;
		onImageUpload?: (imageUrl: string) => void;
	}

	let {
		value = $bindable(''),
		placeholder = m.editor_placeholder(),
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

	// Convert @mentions in markdown to proper mention spans before parsing
	// Format: [@Label](mention:type:id)
	function preprocessMentions(markdown: string): string {
		// Match markdown link format: [@Label](mention:type:id)
		return markdown.replace(
			/\[@([^\]]+)\]\(mention:(user|team):([^)]+)\)/g,
			(_match, label, mentionType, id) => {
				const mentionClass =
					mentionType === 'team' ? 'mention mention-team' : 'mention mention-user';
				return `<span data-type="mention" data-id="${id}" data-label="${label}" data-mention-type="${mentionType}" class="${mentionClass}">@${label}</span>`;
			}
		);
	}

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

		// Custom rule for mentions - serialize to markdown link format
		// Format: [@Label](mention:type:id)
		turndownService.addRule('mention', {
			filter: function (node) {
				return node.nodeName === 'SPAN' && node.getAttribute('data-type') === 'mention';
			},
			replacement: function (_content, node) {
				const el = node as HTMLElement;
				const label = el.getAttribute('data-label') || el.getAttribute('data-id') || '';
				const mentionType = el.getAttribute('data-mention-type') || 'user';
				const id = el.getAttribute('data-id') || '';
				// Use markdown link format: [@Label](mention:type:id)
				return `[@${label}](mention:${mentionType}:${id})`;
			}
		});

		const initialHtml = value ? (marked(preprocessMentions(value)) as string) : '';

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
						class: 'mm-link'
					}
				}),
				Typography,
				TextAlign.configure({
					types: ['heading', 'paragraph']
				}),
				DocImage.configure({
					inline: false,
					allowBase64: true,
					HTMLAttributes: {
						class: 'max-w-full h-auto rounded-md my-4'
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
				TableCell,
				Mention.extend({
					addAttributes() {
						return {
							id: {
								default: null,
								parseHTML: (element) => element.getAttribute('data-id'),
								renderHTML: (attributes) => ({
									'data-id': attributes.id
								})
							},
							label: {
								default: null,
								parseHTML: (element) => element.getAttribute('data-label'),
								renderHTML: (attributes) => ({
									'data-label': attributes.label
								})
							},
							type: {
								default: 'user',
								parseHTML: (element) => element.getAttribute('data-mention-type') || 'user',
								renderHTML: (attributes) => ({
									'data-mention-type': attributes.type || 'user'
								})
							}
						};
					},
					renderHTML({ node, HTMLAttributes }) {
						const mentionType = node.attrs.type || 'user';
						const classes =
							mentionType === 'team' ? 'mention mention-team' : 'mention mention-user';
						return [
							'span',
							{
								...HTMLAttributes,
								'data-type': 'mention',
								class: classes
							},
							`@${node.attrs.label || node.attrs.id}`
						];
					}
				}).configure({
					suggestion: createMentionSuggestion()
				})
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
					class: 'max-w-none focus:outline-none min-h-[200px] px-3 py-2'
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
			const html = value ? (marked(preprocessMentions(value)) as string) : '';
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
			uploadError = m.docs_editor_image_too_large();
			setTimeout(() => (uploadError = ''), 3000);
			return;
		}

		// Validate file type
		const validTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
		if (!validTypes.includes(file.type)) {
			uploadError = m.docs_editor_image_invalid_type();
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
				throw new Error(errorData.error || m.docs_editor_image_upload_error());
			}

			const imageMeta = await response.json();

			// Insert image with server URL
			editor?.chain().focus().setImage({ src: imageMeta.url, alt: file.name }).run();

			if (onImageUpload) {
				onImageUpload(imageMeta.url);
			}
		} catch (err) {
			uploadError = err instanceof Error ? err.message : m.docs_editor_image_upload_error();
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
		const url = window.prompt(m.docs_editor_enter_url_prompt());
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
			const e = editor;
			return (e?.isActive('tableCell') || e?.isActive('tableHeader')) ?? false;
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

<div
	class="doc-editor border border-mm-control-border rounded-sm bg-mm-surface {disabled
		? 'opacity-50'
		: ''}"
>
	<!-- Toolbar -->
	<div class="flex flex-wrap items-center gap-1 p-2 border-b border-mm-rule bg-mm-sunken">
		<button
			type="button"
			on:click={toggleBold}
			{disabled}
			class="tb-btn"
			class:is-active={isBoldActive}
			aria-pressed={isBoldActive}
			title={m.docs_editor_bold_title()}
		>
			<Icon icon="material-symbols:format-bold" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleItalic}
			{disabled}
			class="tb-btn"
			class:is-active={isItalicActive}
			aria-pressed={isItalicActive}
			title={m.docs_editor_italic_title()}
		>
			<Icon icon="material-symbols:format-italic" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleCode}
			{disabled}
			class="tb-btn"
			class:is-active={isCodeActive}
			aria-pressed={isCodeActive}
			title={m.docs_editor_inline_code_title()}
		>
			<Icon icon="material-symbols:code" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		<button
			type="button"
			on:click={() => toggleHeading(1)}
			{disabled}
			class="tb-btn tb-btn--text"
			class:is-active={isHeading1Active}
			aria-pressed={isHeading1Active}
			title={m.docs_editor_heading_title({ level: 1 })}
		>
			H1
		</button>

		<button
			type="button"
			on:click={() => toggleHeading(2)}
			{disabled}
			class="tb-btn tb-btn--text"
			class:is-active={isHeading2Active}
			aria-pressed={isHeading2Active}
			title={m.docs_editor_heading_title({ level: 2 })}
		>
			H2
		</button>

		<button
			type="button"
			on:click={() => toggleHeading(3)}
			{disabled}
			class="tb-btn tb-btn--text"
			class:is-active={isHeading3Active}
			aria-pressed={isHeading3Active}
			title={m.docs_editor_heading_title({ level: 3 })}
		>
			H3
		</button>

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		<button
			type="button"
			on:click={() => setTextAlign('left')}
			{disabled}
			class="tb-btn"
			class:is-active={isAlignLeft}
			aria-pressed={isAlignLeft}
			title={m.docs_editor_align_left_title()}
		>
			<Icon icon="material-symbols:format-align-left" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={() => setTextAlign('center')}
			{disabled}
			class="tb-btn"
			class:is-active={isAlignCenter}
			aria-pressed={isAlignCenter}
			title={m.docs_editor_align_center_title()}
		>
			<Icon icon="material-symbols:format-align-center" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={() => setTextAlign('right')}
			{disabled}
			class="tb-btn"
			class:is-active={isAlignRight}
			aria-pressed={isAlignRight}
			title={m.docs_editor_align_right_title()}
		>
			<Icon icon="material-symbols:format-align-right" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		<button
			type="button"
			on:click={toggleBulletList}
			{disabled}
			class="tb-btn"
			class:is-active={isBulletListActive}
			aria-pressed={isBulletListActive}
			title={m.docs_editor_bullet_list_title()}
		>
			<Icon icon="material-symbols:format-list-bulleted" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleOrderedList}
			{disabled}
			class="tb-btn"
			class:is-active={isOrderedListActive}
			aria-pressed={isOrderedListActive}
			title={m.docs_editor_numbered_list_title()}
		>
			<Icon icon="material-symbols:format-list-numbered" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		<button
			type="button"
			on:click={toggleBlockquote}
			{disabled}
			class="tb-btn"
			class:is-active={isBlockquoteActive}
			aria-pressed={isBlockquoteActive}
			title={m.docs_editor_quote_title()}
		>
			<Icon icon="material-symbols:format-quote" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleCodeBlock}
			{disabled}
			class="tb-btn"
			class:is-active={isCodeBlockActive}
			aria-pressed={isCodeBlockActive}
			title={m.docs_editor_code_block_title()}
		>
			<Icon icon="material-symbols:code-blocks" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		<!-- Table controls -->
		{#if isInTable}
			<div class="flex items-center gap-0.5">
				<button
					type="button"
					on:click={addColumnBefore}
					{disabled}
					class="tb-btn"
					title={m.docs_editor_add_column_before_title()}
				>
					<Icon icon="mdi:table-column-plus-before" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={addColumnAfter}
					{disabled}
					class="tb-btn"
					title={m.docs_editor_add_column_after_title()}
				>
					<Icon icon="mdi:table-column-plus-after" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={deleteColumn}
					{disabled}
					class="tb-btn"
					title={m.docs_editor_delete_column_title()}
				>
					<Icon icon="mdi:table-column-remove" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={addRowBefore}
					{disabled}
					class="tb-btn"
					title={m.docs_editor_add_row_above_title()}
				>
					<Icon icon="mdi:table-row-plus-before" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={addRowAfter}
					{disabled}
					class="tb-btn"
					title={m.docs_editor_add_row_below_title()}
				>
					<Icon icon="mdi:table-row-plus-after" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={deleteRow}
					{disabled}
					class="tb-btn"
					title={m.docs_editor_delete_row_title()}
				>
					<Icon icon="mdi:table-row-remove" class="h-4 w-4" />
				</button>
				<button
					type="button"
					on:click={deleteTable}
					{disabled}
					class="tb-btn tb-btn--danger"
					title={m.docs_editor_delete_table_title()}
				>
					<Icon icon="mdi:table-remove" class="h-4 w-4" />
				</button>
			</div>
		{:else}
			<button
				type="button"
				on:click={insertTable}
				{disabled}
				class="tb-btn"
				title={m.docs_editor_insert_table_title()}
			>
				<Icon icon="material-symbols:table" class="h-4 w-4" />
			</button>
		{/if}

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		{#if isLinkActive}
			<button
				type="button"
				on:click={unsetLink}
				{disabled}
				class="tb-btn is-active"
				title={m.docs_editor_remove_link_title()}
			>
				<Icon icon="material-symbols:link-off" class="h-4 w-4" />
			</button>
		{:else}
			<button
				type="button"
				on:click={setLink}
				{disabled}
				class="tb-btn"
				title={m.docs_editor_add_link_title()}
			>
				<Icon icon="material-symbols:link" class="h-4 w-4" />
			</button>
		{/if}

		<button
			type="button"
			on:click={openFileDialog}
			disabled={disabled || isUploading}
			class="tb-btn"
			title={m.docs_editor_insert_image_title()}
		>
			{#if isUploading}
				<Icon icon="mdi:loading" class="h-4 w-4 animate-spin" />
			{:else}
				<Icon icon="material-symbols:image" class="h-4 w-4" />
			{/if}
		</button>

		{#if uploadError}
			<span class="text-caption text-mm-danger ml-2" role="alert">{uploadError}</span>
		{/if}
	</div>

	<!-- Editor -->
	<div bind:this={element} class="bg-mm-surface rounded-b-sm text-mm-text"></div>
</div>

<style>
	.tb-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 28px;
		height: 28px;
		padding: 0 var(--space-1);
		border-radius: var(--radius-sm);
		color: var(--mm-text-2);
		background: transparent;
		transition: background-color var(--mm-dur-press) var(--mm-ease);
	}
	.tb-btn--text {
		font: var(--text-label);
	}
	.tb-btn--danger {
		color: var(--mm-danger);
	}
	.tb-btn:hover:not(:disabled) {
		background: var(--mm-selected);
		color: var(--mm-text);
	}
	.tb-btn--danger:hover:not(:disabled) {
		background: var(--mm-danger-soft);
		color: var(--mm-danger);
	}
	.tb-btn.is-active {
		background: var(--mm-selected);
		color: var(--mm-text);
	}
	.tb-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.tb-btn:focus-visible {
		outline: 2px solid var(--mm-text);
		outline-offset: 2px;
	}

	.doc-editor :global(.ProseMirror) {
		outline: none;
		font: var(--text-body);
		color: var(--mm-text);
	}

	.doc-editor :global(.ProseMirror p.is-editor-empty:first-child::before) {
		content: attr(data-placeholder);
		float: left;
		color: var(--mm-text-3);
		pointer-events: none;
		height: 0;
	}

	.doc-editor :global(.ProseMirror > * + *) {
		margin-block-start: var(--space-3);
	}

	.doc-editor :global(.ProseMirror h1) {
		font: var(--text-heading);
		margin-block: var(--space-6) var(--space-2);
	}

	.doc-editor :global(.ProseMirror h2) {
		font: var(--text-heading-sm);
		margin-block: var(--space-5) var(--space-2);
	}

	.doc-editor :global(.ProseMirror h3) {
		font: var(--text-panel-title);
		margin-block: var(--space-4) var(--space-1);
	}

	.doc-editor :global(.ProseMirror ul) {
		list-style: disc;
		padding-inline-start: var(--space-5);
	}

	.doc-editor :global(.ProseMirror ol) {
		list-style: decimal;
		padding-inline-start: var(--space-5);
	}

	.doc-editor :global(.ProseMirror blockquote) {
		border-inline-start: 1px solid var(--mm-control-border);
		padding-inline-start: var(--space-4);
		color: var(--mm-text-2);
	}

	.doc-editor :global(.ProseMirror a) {
		color: var(--mm-primary);
	}

	.doc-editor :global(.ProseMirror code) {
		background: var(--mm-sunken);
		padding: 1px var(--space-1);
		border-radius: var(--radius-sm);
		font: var(--text-mono);
	}

	.doc-editor :global(.ProseMirror pre) {
		background: var(--mm-code-bg);
		color: var(--mm-code-text);
		padding: var(--space-4) var(--space-5);
		border-radius: var(--radius-md);
		overflow-x: auto;
	}

	.doc-editor :global(.ProseMirror pre code) {
		background: transparent;
		padding: 0;
		color: inherit;
		font: var(--text-mono-editor);
		display: block;
		white-space: pre;
		width: max-content;
		min-width: 100%;
	}

	.doc-editor :global(.ProseMirror strong) {
		font-weight: 600;
	}

	.doc-editor :global(.ProseMirror em) {
		font-style: italic;
	}

	.doc-editor :global(.ProseMirror img) {
		max-width: 100%;
		height: auto;
		border-radius: var(--radius-md);
		border: 1px solid var(--mm-rule);
		cursor: pointer;
	}

	.doc-editor :global(.ProseMirror img:hover),
	.doc-editor :global(.ProseMirror img.ProseMirror-selectednode) {
		outline: 2px solid var(--mm-text);
		outline-offset: 2px;
	}

	.doc-editor :global(.ProseMirror table) {
		width: 100%;
		border-collapse: collapse;
		font: var(--text-dense);
		border: 1px solid var(--mm-rule);
	}

	.doc-editor :global(.ProseMirror th),
	.doc-editor :global(.ProseMirror td) {
		position: relative;
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--mm-rule);
		text-align: start;
	}

	.doc-editor :global(.ProseMirror th) {
		font: var(--text-label);
		color: var(--mm-text-2);
		background: var(--mm-sunken);
	}

	.doc-editor :global(.ProseMirror .selectedCell) {
		background: var(--mm-selected);
	}

	.doc-editor :global(.ProseMirror .column-resize-handle) {
		position: absolute;
		inset-block: 0;
		inset-inline-end: 0;
		width: 4px;
		background: var(--mm-control-border);
		pointer-events: none;
	}

	.doc-editor :global(.ProseMirror.resize-cursor) {
		cursor: col-resize;
	}

	.doc-editor :global(.ProseMirror .hljs-comment),
	.doc-editor :global(.ProseMirror .hljs-quote) {
		color: var(--mm-syn-comment);
		font-style: italic;
	}

	.doc-editor :global(.ProseMirror .hljs-keyword),
	.doc-editor :global(.ProseMirror .hljs-selector-tag),
	.doc-editor :global(.ProseMirror .hljs-addition),
	.doc-editor :global(.ProseMirror .hljs-built_in),
	.doc-editor :global(.ProseMirror .hljs-deletion) {
		color: var(--mm-syn-keyword);
	}

	.doc-editor :global(.ProseMirror .hljs-string),
	.doc-editor :global(.ProseMirror .hljs-meta .hljs-meta-string),
	.doc-editor :global(.ProseMirror .hljs-doctag),
	.doc-editor :global(.ProseMirror .hljs-regexp) {
		color: var(--mm-syn-string);
	}

	.doc-editor :global(.ProseMirror .hljs-number),
	.doc-editor :global(.ProseMirror .hljs-literal),
	.doc-editor :global(.ProseMirror .hljs-attribute),
	.doc-editor :global(.ProseMirror .hljs-attr),
	.doc-editor :global(.ProseMirror .hljs-variable),
	.doc-editor :global(.ProseMirror .hljs-template-variable),
	.doc-editor :global(.ProseMirror .hljs-type),
	.doc-editor :global(.ProseMirror .hljs-symbol),
	.doc-editor :global(.ProseMirror .hljs-bullet),
	.doc-editor :global(.ProseMirror .hljs-link) {
		color: var(--mm-syn-number);
	}

	.doc-editor :global(.ProseMirror .hljs-title),
	.doc-editor :global(.ProseMirror .hljs-section),
	.doc-editor :global(.ProseMirror .hljs-name),
	.doc-editor :global(.ProseMirror .hljs-selector-id),
	.doc-editor :global(.ProseMirror .hljs-selector-class),
	.doc-editor :global(.ProseMirror .hljs-class .hljs-title) {
		color: var(--mm-syn-function);
	}

	.doc-editor :global(.ProseMirror .hljs-emphasis) {
		font-style: italic;
	}

	.doc-editor :global(.ProseMirror .hljs-strong) {
		font-weight: 600;
	}

	/* People and teams share one neutral mention style; the accent stays reserved for links and the primary button. */
	.doc-editor :global(.ProseMirror .mention) {
		padding: 0 var(--space-1);
		border-radius: var(--radius-sm);
		font-weight: 600;
		background: var(--mm-info-soft);
		color: var(--mm-text);
	}
</style>
