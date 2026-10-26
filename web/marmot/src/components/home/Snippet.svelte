<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';

	let { code, label }: { code: string; label?: string } = $props();

	let copied = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copy() {
		try {
			await navigator.clipboard.writeText(code);
			copied = true;
			clearTimeout(timer);
			timer = setTimeout(() => (copied = false), 1600);
		} catch {
			// Clipboard access can be refused; the text stays selectable
		}
	}
</script>

<div class="overflow-hidden rounded-lg border border-gray-800 bg-[#171513]">
	{#if label}
		<div
			class="flex items-center justify-between border-b border-white/5 px-3 py-1.5 text-[11px] text-gray-400"
		>
			<span class="font-mono">{label}</span>
		</div>
	{/if}
	<div class="group relative">
		<pre
			class="overflow-x-auto px-3 py-2.5 font-mono text-[12.5px] leading-relaxed text-gray-100">{code}</pre>
		<button
			type="button"
			onclick={copy}
			class="absolute right-2 top-2 inline-flex items-center gap-1 rounded-md bg-white/10 px-2 py-1 text-[11px] font-medium text-gray-200 opacity-0 transition hover:bg-white/20 focus:opacity-100 group-hover:opacity-100"
			aria-label={m.home_copy()}
		>
			<IconifyIcon
				icon={copied ? 'material-symbols:check' : 'material-symbols:content-copy-outline'}
				class="h-3.5 w-3.5"
			/>
			{copied ? m.home_copied() : m.home_copy()}
		</button>
	</div>
</div>
