<script lang="ts">
	import Icon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';

	export let value: string = '';
	export let min: string | null = null;
	export let id: string | undefined = undefined;
	export let placeholder: string = m.ui_datepicker_placeholder();

	let open = false;
	let container: HTMLDivElement;

	const weekdays = [
		m.ui_datepicker_weekday_mo(),
		m.ui_datepicker_weekday_tu(),
		m.ui_datepicker_weekday_we(),
		m.ui_datepicker_weekday_th(),
		m.ui_datepicker_weekday_fr(),
		m.ui_datepicker_weekday_sa(),
		m.ui_datepicker_weekday_su()
	];
	const today = new Date();

	let viewYear = today.getFullYear();
	let viewMonth = today.getMonth();

	function parse(dateValue: string): Date | null {
		const [year, month, day] = dateValue.split('-').map(Number);
		if (!year || !month || !day) return null;
		return new Date(year, month - 1, day);
	}

	function format(date: Date): string {
		const month = String(date.getMonth() + 1).padStart(2, '0');
		const day = String(date.getDate()).padStart(2, '0');
		return `${date.getFullYear()}-${month}-${day}`;
	}

	function toggle() {
		open = !open;
		if (open) {
			const selected = parse(value);
			viewYear = (selected ?? today).getFullYear();
			viewMonth = (selected ?? today).getMonth();
		}
	}

	function previousMonth() {
		if (viewMonth === 0) {
			viewMonth = 11;
			viewYear -= 1;
		} else {
			viewMonth -= 1;
		}
	}

	function nextMonth() {
		if (viewMonth === 11) {
			viewMonth = 0;
			viewYear += 1;
		} else {
			viewMonth += 1;
		}
	}

	function select(day: number) {
		value = format(new Date(viewYear, viewMonth, day));
		open = false;
	}

	function isDisabled(day: number): boolean {
		if (!min) return false;
		const minDate = parse(min);
		return minDate !== null && new Date(viewYear, viewMonth, day) < minDate;
	}

	function isSelected(day: number): boolean {
		const selected = parse(value);
		return (
			selected !== null &&
			selected.getFullYear() === viewYear &&
			selected.getMonth() === viewMonth &&
			selected.getDate() === day
		);
	}

	function isToday(day: number): boolean {
		return (
			today.getFullYear() === viewYear && today.getMonth() === viewMonth && today.getDate() === day
		);
	}

	function handleWindowClick(event: MouseEvent) {
		if (open && container && !container.contains(event.target as Node)) {
			open = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			open = false;
		}
	}

	// Leading blanks so the 1st lands on its weekday (Monday-first)
	$: leadingBlanks = (new Date(viewYear, viewMonth, 1).getDay() + 6) % 7;
	$: daysInMonth = new Date(viewYear, viewMonth + 1, 0).getDate();
	$: monthLabel = new Date(viewYear, viewMonth, 1).toLocaleDateString(undefined, {
		month: 'long',
		year: 'numeric'
	});
	$: selectedLabel = (() => {
		const selected = parse(value);
		return selected
			? selected.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
			: '';
	})();
</script>

<svelte:window onclick={handleWindowClick} onkeydown={handleKeydown} />

<div class="relative" bind:this={container}>
	<button
		{id}
		type="button"
		onclick={toggle}
		aria-haspopup="dialog"
		aria-expanded={open}
		class="flex h-[var(--mm-control)] items-center gap-2 rounded-sm border border-mm-control-border bg-mm-surface px-[10px] text-dense hover:border-mm-text {selectedLabel
			? 'text-mm-text'
			: 'text-mm-text-3'}"
	>
		<span class="w-4 h-4 text-mm-text-2">
			<Icon icon="material-symbols:calendar-month-outline" />
		</span>
		{selectedLabel || placeholder}
	</button>

	{#if open}
		<div
			class="absolute left-0 top-full mt-2 z-20 w-64 p-3 bg-mm-raised border border-mm-rule rounded-md shadow-float"
			role="dialog"
			aria-label={m.ui_datepicker_choose_date_aria()}
		>
			<div class="flex items-center justify-between mb-2">
				<button
					type="button"
					onclick={previousMonth}
					aria-label={m.ui_datepicker_previous_month_aria()}
					class="p-1 rounded-md text-mm-text-2 hover:bg-mm-sunken hover:text-mm-text"
				>
					<span class="block w-4 h-4"><Icon icon="material-symbols:chevron-left" /></span>
				</button>
				<span class="text-dense font-semibold text-mm-text">{monthLabel}</span>
				<button
					type="button"
					onclick={nextMonth}
					aria-label={m.ui_datepicker_next_month_aria()}
					class="p-1 rounded-md text-mm-text-2 hover:bg-mm-sunken hover:text-mm-text"
				>
					<span class="block w-4 h-4"><Icon icon="material-symbols:chevron-right" /></span>
				</button>
			</div>

			<div class="grid grid-cols-7 gap-y-1 text-center">
				{#each weekdays as weekday (weekday)}
					<span class="text-caption font-semibold text-mm-text-2 py-1">{weekday}</span>
				{/each}
				{#each Array(leadingBlanks) as _, i (i)}
					<span></span>
				{/each}
				{#each Array(daysInMonth) as _, i (i)}
					{@const day = i + 1}
					<button
						type="button"
						onclick={() => select(day)}
						disabled={isDisabled(day)}
						class="w-8 h-8 mx-auto flex items-center justify-center rounded-sm text-dense tabular-nums transition-colors
							{isSelected(day)
							? 'bg-mm-text text-mm-on-inverse font-semibold'
							: isToday(day)
								? 'text-mm-text font-semibold underline underline-offset-4 hover:bg-mm-sunken'
								: 'text-mm-text hover:bg-mm-sunken'}
							{isDisabled(day) ? 'opacity-40 cursor-not-allowed hover:bg-transparent' : ''}"
					>
						{day}
					</button>
				{/each}
			</div>
		</div>
	{/if}
</div>
