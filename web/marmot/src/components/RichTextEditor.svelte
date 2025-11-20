<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Editor } from '@tiptap/core';
	import StarterKit from '@tiptap/starter-kit';
	import Placeholder from '@tiptap/extension-placeholder';
	import Link from '@tiptap/extension-link';
	import Typography from '@tiptap/extension-typography';
	import { marked } from 'marked';
	import TurndownService from 'turndown';
	import Icon from '@iconify/svelte';

	export let value: string = '';
	export let placeholder: string = 'Start typing...';
	export let disabled: boolean = false;

	let editor: Editor | null = null;
	let element: HTMLElement;
	let turndownService: TurndownService;
	let isUpdating = false;

	onMount(() => {
		turndownService = new TurndownService({
			headingStyle: 'atx',
			codeBlockStyle: 'fenced',
			emDelimiter: '*',
			strongDelimiter: '**'
		});

		const initialHtml = value ? (marked(value) as string) : '';

		editor = new Editor({
			element: element,
			extensions: [
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
			],
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
					class: 'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[100px] px-3 py-2'
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
				const html = value ? (marked(value) as string) : '';
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
			disabled={disabled}
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
				disabled={disabled}
				class="p-1.5 rounded bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-700 disabled:opacity-50 disabled:cursor-not-allowed"
				title="Remove Link"
			>
				<Icon icon="material-symbols:link-off" class="h-4 w-4" />
			</button>
		{:else}
			<button
				type="button"
				on:click={setLink}
				disabled={disabled}
				class="p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
				title="Add Link"
			>
				<Icon icon="material-symbols:link" class="h-4 w-4" />
			</button>
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
</style>
