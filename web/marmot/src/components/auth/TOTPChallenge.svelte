<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		verify,
		loading = false,
		inverse = false
	}: {
		verify: (code: string) => Promise<void>;
		loading?: boolean;
		inverse?: boolean;
	} = $props();

	let digits = $state<string[]>(Array(6).fill(''));
	let recoveryMode = $state(false);
	let recoveryCode = $state('');
	let submitting = $state(false);
	let inputs: HTMLInputElement[] = [];
	let recoveryInput = $state<HTMLInputElement>();

	onMount(() => inputs[0]?.focus());

	async function submit(code: string) {
		if (loading || submitting) return;
		submitting = true;
		try {
			await verify(code);
		} finally {
			submitting = false;
			digits = Array(6).fill('');
			recoveryCode = '';
			if (recoveryMode) recoveryInput?.focus();
			else inputs[0]?.focus();
		}
	}

	function fill(index: number, raw: string) {
		const value = raw.replace(/\D/g, '');
		const start = value.length === 6 ? 0 : index;
		const next = [...digits];
		const added = value.slice(0, 6 - start).split('');
		if (added.length) added.forEach((digit, offset) => (next[start + offset] = digit));
		else next[index] = '';
		digits = next;
		if (next.every(Boolean)) void submit(next.join(''));
		else if (added.length) inputs[Math.min(start + added.length, 5)]?.focus();
	}

	function paste(event: ClipboardEvent) {
		const value = event.clipboardData?.getData('text').replace(/[\s-]/g, '') ?? '';
		if (/^\d{6}$/.test(value)) {
			event.preventDefault();
			digits = value.split('');
			void submit(value);
		} else if (/^[a-f\d]{32}$/i.test(value)) {
			event.preventDefault();
			recoveryMode = true;
			recoveryCode = value;
			void submit(value);
		}
	}

	function keydown(index: number, event: KeyboardEvent) {
		if (event.key === 'Backspace' && !digits[index] && index > 0) {
			event.preventDefault();
			digits = digits.map((digit, position) => (position === index - 1 ? '' : digit));
			inputs[index - 1]?.focus();
		} else if (event.key === 'ArrowLeft') inputs[Math.max(index - 1, 0)]?.focus();
		else if (event.key === 'ArrowRight') inputs[Math.min(index + 1, 5)]?.focus();
	}

	function toggleMode() {
		recoveryMode = !recoveryMode;
		setTimeout(() => (recoveryMode ? recoveryInput : inputs[0])?.focus(), 0);
	}
</script>

<div class="space-y-4 text-center">
	{#if recoveryMode}
		<form
			class="space-y-4"
			onsubmit={(event) => {
				event.preventDefault();
				if (recoveryCode.trim()) void submit(recoveryCode.trim());
			}}
		>
			<label for="totp-recovery" class="block text-sm font-medium">{m.totp_code()}</label>
			<input
				id="totp-recovery"
				bind:this={recoveryInput}
				bind:value={recoveryCode}
				autocomplete="one-time-code"
				maxlength="128"
				required
				class="w-full rounded-lg border px-4 py-3 text-center font-mono {inverse
					? 'border-white/20 bg-white/5 text-white'
					: 'border-gray-300 bg-white text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white'}"
				onpaste={(event) => {
					const value = event.clipboardData?.getData('text').trim() ?? '';
					if (/^[a-f\d]{32}$/i.test(value)) {
						event.preventDefault();
						recoveryCode = value;
						void submit(value);
					}
				}}
			/>
			<Button type="submit" {loading} text={m.totp_verify()} variant="filled" />
		</form>
	{:else}
		<div role="group" aria-label={m.totp_code()} class="flex justify-center gap-2">
			{#each [0, 1, 2, 3, 4, 5] as index (index)}
				<input
					bind:this={inputs[index]}
					value={digits[index]}
					aria-label={m.totp_digit_label({ number: String(index + 1) })}
					type="text"
					inputmode="numeric"
					autocomplete={index === 0 ? 'one-time-code' : 'off'}
					maxlength="6"
					disabled={loading || submitting}
					class="h-14 w-10 rounded-lg border text-center font-mono text-2xl sm:w-12 {inverse
						? 'border-white/20 bg-white/5 text-white focus:border-white/60'
						: 'border-gray-300 bg-white text-gray-900 focus:border-earthy-terracotta-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white'}"
					oninput={(event) => fill(index, event.currentTarget.value)}
					onpaste={paste}
					onkeydown={(event) => keydown(index, event)}
					onfocus={(event) => event.currentTarget.select()}
				/>
			{/each}
		</div>
	{/if}
	{#if loading || submitting}<p role="status" class="text-sm">{m.totp_verifying()}</p>{/if}
	<button
		type="button"
		class="text-sm underline underline-offset-4 {inverse
			? 'text-white/70 hover:text-white'
			: 'text-earthy-terracotta-700 hover:text-earthy-terracotta-800 dark:text-earthy-terracotta-300'}"
		onclick={toggleMode}
	>
		{recoveryMode ? m.totp_use_authenticator() : m.totp_use_recovery()}
	</button>
</div>
