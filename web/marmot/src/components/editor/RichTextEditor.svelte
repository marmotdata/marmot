<script lang="ts">
	import { onMount, onDestroy, mount, unmount } from 'svelte';
	import { Editor } from '@tiptap/core';
	import StarterKit from '@tiptap/starter-kit';
	import Placeholder from '@tiptap/extension-placeholder';
	import Link from '@tiptap/extension-link';
	import Typography from '@tiptap/extension-typography';
	import Mention from '@tiptap/extension-mention';
	import { marked } from 'marked';
	import TurndownService from 'turndown';
	import Icon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import MentionList from './MentionList.svelte';

	export let value: string = '';
	export let placeholder: string = 'Start typing...';
	export let disabled: boolean = false;
	export let enableMentions: boolean = false;

	let editor: Editor | null = null;
	let element: HTMLElement;
	let turndownService: TurndownService;
	let isUpdating = false;

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

	interface MentionItem {
		type: string;
		id: string;
		name: string;
		username?: string;
		profile_picture?: string;
	}

	function createMentionSuggestion() {
		let searchCache: Map<string, Array<MentionItem>> = new Map();

		const searchOwners = async (query: string) => {
			if (searchCache.has(query)) {
				return searchCache.get(query)!;
			}
			try {
				const response = await fetchApi(`/owners/search?q=${encodeURIComponent(query)}&limit=8`);
				if (response.ok) {
					const data = await response.json();
					const owners = data.owners || [];
					searchCache.set(query, owners);
					return owners;
				}
			} catch (err) {
				console.error('Failed to search owners:', err);
			}
			return [];
		};

		return {
			items: async ({ query }: { query: string }) => {
				if (!query || query.length < 1) return [];
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
						popup.style.position = 'fixed';
						popup.style.zIndex = '9999';
						document.body.appendChild(popup);

						const rect = props.clientRect?.();
						if (rect) {
							popup.style.left = `${rect.left}px`;
							popup.style.top = `${rect.bottom + 4}px`;
						}

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
						searchCache.clear();
					}
				};
			}
		};
	}

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

		// Custom rule for mentions - serialize to markdown link format
		// Format: [@Label](mention:type:id)
		turndownService.addRule('mention', {
			filter: (node) => {
				return node.nodeName === 'SPAN' && node.getAttribute('data-type') === 'mention';
			},
			replacement: (_content, node) => {
				const el = node as HTMLElement;
				const label = el.getAttribute('data-label') || el.getAttribute('data-id') || '';
				const mentionType = el.getAttribute('data-mention-type') || 'user';
				const id = el.getAttribute('data-id') || '';
				// Use markdown link format: [@Label](mention:type:id)
				return `[@${label}](mention:${mentionType}:${id})`;
			}
		});

		const initialHtml = value ? (marked(preprocessMentions(value)) as string) : '';

		const extensions: any[] = [
			StarterKit.configure({
				heading: {
					levels: [1, 2, 3]
				}
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
			Typography
		];

		if (enableMentions) {
			extensions.push(
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
			);
		}

		editor = new Editor({
			element: element,
			extensions,
			content: initialHtml,
			editable: !disabled,
			onUpdate: ({ editor }) => {
				if (!isUpdating) {
					const html = editor.getHTML();
					const markdown = turndownService.turndown(html);
					value = markdown;
				}
			},
			editorProps: {
				attributes: {
					class:
						'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[100px] px-3 py-2'
				}
			}
		});
	});

	onDestroy(() => {
		if (editor) {
			editor.destroy();
		}
	});

	$: if (editor && !editor.isDestroyed) {
		editor.setEditable(!disabled);
	}

	$: {
		if (editor && !editor.isDestroyed && value !== undefined) {
			const currentMarkdown = turndownService?.turndown(editor.getHTML()) || '';
			if (currentMarkdown !== value && !isUpdating) {
				isUpdating = true;
				const html = value ? (marked(preprocessMentions(value)) as string) : '';
				editor.commands.setContent(html);
				isUpdating = false;
			}
		}
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

	function setLink() {
		const url = window.prompt('Enter URL:');
		if (url) {
			editor?.chain().focus().setLink({ href: url }).run();
		}
	}

	function unsetLink() {
		editor?.chain().focus().unsetLink().run();
	}

	$: isActive = (name: string, attrs?: any) => editor?.isActive(name, attrs) ?? false;
</script>

<div class="border border-gray-300 dark:border-gray-600 rounded-md {disabled ? 'opacity-50' : ''}">
	<!-- Toolbar -->
	<div
		class="flex flex-wrap gap-1 p-2 border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/50"
	>
		<button
			type="button"
			on:click={toggleBold}
			{disabled}
			class="p-1.5 rounded {isActive('bold')
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
			class="p-1.5 rounded {isActive('italic')
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
			class="p-1.5 rounded {isActive('code')
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
			class="p-1.5 rounded {isActive('heading', { level: 1 })
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
			class="p-1.5 rounded {isActive('heading', { level: 2 })
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
			class="p-1.5 rounded {isActive('heading', { level: 3 })
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed text-sm font-bold"
			title="Heading 3"
		>
			H3
		</button>

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		<button
			type="button"
			on:click={toggleBulletList}
			{disabled}
			class="p-1.5 rounded {isActive('bulletList')
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
			class="p-1.5 rounded {isActive('orderedList')
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
			class="p-1.5 rounded {isActive('blockquote')
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
			class="p-1.5 rounded {isActive('codeBlock')
				? 'bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
				: 'hover:bg-gray-200 dark:hover:bg-gray-700'} disabled:opacity-50 disabled:cursor-not-allowed"
			title="Code Block"
		>
			<Icon icon="material-symbols:code-blocks" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>

		{#if isActive('link')}
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

		{#if enableMentions}
			<div class="w-px h-6 bg-gray-300 dark:bg-gray-600 mx-1"></div>
			<span class="px-2 py-1 text-xs text-gray-500 dark:text-gray-400">Type @ to mention</span>
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
		@apply bg-gray-100 dark:bg-gray-800 p-4 rounded-lg my-2 overflow-x-auto;
	}

	:global(.ProseMirror pre code) {
		@apply bg-transparent p-0;
	}

	:global(.ProseMirror strong) {
		@apply font-bold;
	}

	:global(.ProseMirror em) {
		@apply italic;
	}

	/* Mention styles - shared */
	:global(.ProseMirror .mention) {
		@apply px-1 py-0.5 rounded font-medium;
	}

	/* User mentions - terracotta/orange */
	:global(.ProseMirror .mention-user) {
		@apply bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-400;
	}

	/* Team mentions - blue */
	:global(.ProseMirror .mention-team) {
		@apply bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400;
	}
</style>
