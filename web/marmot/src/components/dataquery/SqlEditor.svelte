<script lang="ts">
	import Prism from 'prismjs';
	import 'prismjs/components/prism-sql';
	import { m } from '$lib/paraglide/messages';

	// The gutter, the Prism highlight layer and the textarea that types into it; a caret-level API
	// (editorEl) is exposed so callers such as the source explorer can insert text at the cursor.
	let {
		value = $bindable(''),
		height,
		editorEl = $bindable(null),
		onRun
	}: {
		value?: string;
		height: number;
		editorEl?: HTMLTextAreaElement | null;
		onRun: () => void;
	} = $props();

	let layer = $state<HTMLPreElement | null>(null);
	let lineCount = $derived(Math.max(value.split('\n').length, 1));

	function onKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
			e.preventDefault();
			onRun();
			return;
		}
		if (e.key === 'Tab' && editorEl) {
			e.preventDefault();
			const { selectionStart, selectionEnd } = editorEl;
			value = value.slice(0, selectionStart) + '  ' + value.slice(selectionEnd);
			queueMicrotask(() => editorEl?.setSelectionRange(selectionStart + 2, selectionStart + 2));
		}
	}

	// Prism marks up the text itself, so the layer never renders raw HTML; a trailing space keeps it as tall as the textarea when the statement ends on an empty line
	function highlightSql(node: HTMLElement, text: string) {
		const paint = (code: string) => {
			node.textContent = code + (code.endsWith('\n') ? ' ' : '');
			Prism.highlightElement(node);
		};
		paint(text);
		return { update: paint };
	}
</script>

<div
	class="flex flex-shrink-0 overflow-auto bg-[#171513] font-mono text-sm leading-6"
	style="height: {height}px"
>
	<div class="select-none py-3 pl-4 pr-3 text-right text-gray-600" aria-hidden="true">
		{#each Array(lineCount) as _, i (i)}
			<div>{i + 1}</div>
		{/each}
	</div>
	<!-- The highlighted copy sits under a transparent textarea, so typing, the caret and selection stay native -->
	<div class="sql-editor relative min-w-0 flex-1">
		<pre class="sql-layer" aria-hidden="true" bind:this={layer}><code
				class="language-sql"
				use:highlightSql={value}></code></pre>
		<textarea
			bind:this={editorEl}
			bind:value
			onkeydown={onKeydown}
			onscroll={() => layer && editorEl && (layer.scrollLeft = editorEl.scrollLeft)}
			spellcheck="false"
			wrap="off"
			rows={Math.max(lineCount, 4)}
			placeholder={m.dataquery_console_placeholder()}
			aria-label={m.dataquery_console_editor_label()}
			class="sql-layer sql-input resize-none border-0 bg-transparent caret-earthy-terracotta-400 placeholder:text-gray-600 focus:outline-none focus:ring-0"
		></textarea>
	</div>
</div>

<style>
	.sql-layer {
		margin: 0;
		padding: 0.75rem 1rem;
		font-family: inherit;
		font-size: inherit;
		line-height: inherit;
		white-space: pre;
		overflow: hidden;
		tab-size: 2;
	}

	pre.sql-layer {
		position: absolute;
		inset: 0;
		pointer-events: none;
		color: #f4e4dc;
		background: none;
		border-radius: 0;
	}

	/* A Prism theme loaded elsewhere styles code[class*='language-'], so the layer pins its own type metrics to stay aligned with the textarea */
	.sql-layer code {
		font: inherit;
		line-height: inherit;
		white-space: inherit;
		color: inherit;
		background: none;
		text-shadow: none;
		padding: 0;
	}

	.sql-input {
		position: relative;
		display: block;
		width: 100%;
		min-height: 100%;
		color: transparent;
		overflow-x: auto;
	}

	.sql-input::selection {
		background: rgba(245, 112, 74, 0.35);
	}

	.sql-editor :global(.token.keyword) {
		color: #ffa382;
		font-weight: 600;
	}

	.sql-editor :global(.token.function) {
		color: #ecd292;
	}

	.sql-editor :global(.token.string) {
		color: #b9cdb9;
	}

	.sql-editor :global(.token.number),
	.sql-editor :global(.token.boolean) {
		color: #a2b4c6;
	}

	.sql-editor :global(.token.comment) {
		color: #696969;
		font-style: italic;
	}

	.sql-editor :global(.token.operator),
	.sql-editor :global(.token.punctuation) {
		color: #b0b0b0;
	}
</style>
