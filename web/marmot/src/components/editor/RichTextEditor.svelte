<script lang="ts">
	import { onMount, onDestroy, mount, unmount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { Editor, type Extensions } from '@tiptap/core';
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
	import { m } from '$lib/paraglide/messages';

	export let value: string = '';
	export let placeholder: string = m.editor_placeholder();
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
		let searchCache: Map<string, Array<MentionItem>> = new SvelteMap();

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

		const extensions: Extensions = [
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
					class: 'mm-link'
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
					class: 'max-w-none focus:outline-none min-h-[100px] px-3 py-2'
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
		const url = window.prompt(m.docs_editor_enter_url_prompt());
		if (url) {
			editor?.chain().focus().setLink({ href: url }).run();
		}
	}

	function unsetLink() {
		editor?.chain().focus().unsetLink().run();
	}

	function isActive(name: string, attrs?: Record<string, unknown>): boolean {
		return editor?.isActive(name, attrs) ?? false;
	}
</script>

<div
	class="rte border border-mm-control-border rounded-sm bg-mm-surface {disabled
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
			class:is-active={isActive('bold')}
			aria-pressed={isActive('bold')}
			title={m.docs_editor_bold_title()}
		>
			<Icon icon="material-symbols:format-bold" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleItalic}
			{disabled}
			class="tb-btn"
			class:is-active={isActive('italic')}
			aria-pressed={isActive('italic')}
			title={m.docs_editor_italic_title()}
		>
			<Icon icon="material-symbols:format-italic" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleCode}
			{disabled}
			class="tb-btn"
			class:is-active={isActive('code')}
			aria-pressed={isActive('code')}
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
			class:is-active={isActive('heading', { level: 1 })}
			aria-pressed={isActive('heading', { level: 1 })}
			title={m.docs_editor_heading_title({ level: 1 })}
		>
			H1
		</button>

		<button
			type="button"
			on:click={() => toggleHeading(2)}
			{disabled}
			class="tb-btn tb-btn--text"
			class:is-active={isActive('heading', { level: 2 })}
			aria-pressed={isActive('heading', { level: 2 })}
			title={m.docs_editor_heading_title({ level: 2 })}
		>
			H2
		</button>

		<button
			type="button"
			on:click={() => toggleHeading(3)}
			{disabled}
			class="tb-btn tb-btn--text"
			class:is-active={isActive('heading', { level: 3 })}
			aria-pressed={isActive('heading', { level: 3 })}
			title={m.docs_editor_heading_title({ level: 3 })}
		>
			H3
		</button>

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		<button
			type="button"
			on:click={toggleBulletList}
			{disabled}
			class="tb-btn"
			class:is-active={isActive('bulletList')}
			aria-pressed={isActive('bulletList')}
			title={m.docs_editor_bullet_list_title()}
		>
			<Icon icon="material-symbols:format-list-bulleted" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleOrderedList}
			{disabled}
			class="tb-btn"
			class:is-active={isActive('orderedList')}
			aria-pressed={isActive('orderedList')}
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
			class:is-active={isActive('blockquote')}
			aria-pressed={isActive('blockquote')}
			title={m.docs_editor_quote_title()}
		>
			<Icon icon="material-symbols:format-quote" class="h-4 w-4" />
		</button>

		<button
			type="button"
			on:click={toggleCodeBlock}
			{disabled}
			class="tb-btn"
			class:is-active={isActive('codeBlock')}
			aria-pressed={isActive('codeBlock')}
			title={m.docs_editor_code_block_title()}
		>
			<Icon icon="material-symbols:code-blocks" class="h-4 w-4" />
		</button>

		<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>

		{#if isActive('link')}
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

		{#if enableMentions}
			<div class="w-px h-6 bg-mm-rule mx-1" aria-hidden="true"></div>
			<span class="px-2 py-1 text-caption text-mm-text-2">{m.editor_mention_hint()}</span>
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
	.tb-btn:hover:not(:disabled) {
		background: var(--mm-selected);
		color: var(--mm-text);
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

	.rte :global(.ProseMirror) {
		outline: none;
	}

	.rte :global(.ProseMirror p.is-editor-empty:first-child::before) {
		content: attr(data-placeholder);
		float: left;
		color: var(--mm-text-3);
		pointer-events: none;
		height: 0;
	}

	.rte :global(.ProseMirror h1) {
		font: var(--text-heading);
		margin-block: var(--space-4) var(--space-2);
	}

	.rte :global(.ProseMirror h2) {
		font: var(--text-heading-sm);
		margin-block: var(--space-3) var(--space-2);
	}

	.rte :global(.ProseMirror h3) {
		font: var(--text-panel-title);
		margin-block: var(--space-2) var(--space-1);
	}

	.rte :global(.ProseMirror ul) {
		list-style: disc;
		padding-inline-start: var(--space-5);
		margin-block: var(--space-2);
	}

	.rte :global(.ProseMirror ol) {
		list-style: decimal;
		padding-inline-start: var(--space-5);
		margin-block: var(--space-2);
	}

	.rte :global(.ProseMirror blockquote) {
		border-inline-start: 1px solid var(--mm-control-border);
		padding-inline-start: var(--space-4);
		color: var(--mm-text-2);
		margin-block: var(--space-2);
	}

	.rte :global(.ProseMirror code) {
		background: var(--mm-sunken);
		padding: 1px var(--space-1);
		border-radius: var(--radius-sm);
		font: var(--text-mono);
	}

	.rte :global(.ProseMirror pre) {
		background: var(--mm-code-bg);
		color: var(--mm-code-text);
		padding: var(--space-4);
		border-radius: var(--radius-md);
		margin-block: var(--space-2);
		overflow-x: auto;
	}

	.rte :global(.ProseMirror pre code) {
		background: transparent;
		padding: 0;
		color: inherit;
	}

	.rte :global(.ProseMirror a) {
		color: var(--mm-primary);
	}

	.rte :global(.ProseMirror strong) {
		font-weight: 600;
	}

	.rte :global(.ProseMirror em) {
		font-style: italic;
	}

	/* People and teams share one neutral mention style; the accent stays reserved for links and the primary button. */
	.rte :global(.ProseMirror .mention) {
		padding: 0 var(--space-1);
		border-radius: var(--radius-sm);
		font-weight: 600;
		background: var(--mm-info-soft);
		color: var(--mm-text);
	}
</style>
