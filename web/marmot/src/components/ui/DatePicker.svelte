<script lang="ts">
	import Icon from '@iconify/svelte';

	export let value: string = '';
	export let min: string | null = null;
	export let id: string | undefined = undefined;
	export let placeholder: string = 'Select date';

	let open = false;
	let container: HTMLDivElement;

	const weekdays = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su'];
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
		class="flex items-center gap-2 px-3 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 dark:focus:ring-earthy-terracotta-500 focus:border-transparent {selectedLabel
			? 'text-gray-900 dark:text-gray-100'
			: 'text-gray-400 dark:text-gray-500'}"
	>
		<span class="w-4 h-4 text-gray-500 dark:text-gray-400">
			<Icon icon="material-symbols:calendar-month-outline" />
		</span>
		{selectedLabel || placeholder}
	</button>

	{#if open}
		<div
			class="absolute left-0 top-full mt-2 z-20 w-64 p-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg"
			role="dialog"
			aria-label="Choose date"
		>
			<div class="flex items-center justify-between mb-2">
				<button
					type="button"
					onclick={previousMonth}
					aria-label="Previous month"
					class="p-1 rounded-md text-gray-500 dark:text-gray-400 hover:bg-earthy-brown-100 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-gray-100"
				>
					<span class="block w-4 h-4"><Icon icon="material-symbols:chevron-left" /></span>
				</button>
				<span class="text-sm font-medium text-gray-900 dark:text-gray-100">{monthLabel}</span>
				<button
					type="button"
					onclick={nextMonth}
					aria-label="Next month"
					class="p-1 rounded-md text-gray-500 dark:text-gray-400 hover:bg-earthy-brown-100 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-gray-100"
				>
					<span class="block w-4 h-4"><Icon icon="material-symbols:chevron-right" /></span>
				</button>
			</div>

			<div class="grid grid-cols-7 gap-y-1 text-center">
				{#each weekdays as weekday (weekday)}
					<span class="text-xs font-medium text-gray-500 dark:text-gray-400 py-1">{weekday}</span>
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
						class="w-8 h-8 mx-auto flex items-center justify-center rounded-full text-sm transition-colors
							{isSelected(day)
							? 'bg-earthy-terracotta-600 text-white font-medium'
							: isToday(day)
								? 'text-earthy-terracotta-600 dark:text-earthy-terracotta-500 font-semibold hover:bg-earthy-brown-100 dark:hover:bg-gray-700'
								: 'text-gray-700 dark:text-gray-300 hover:bg-earthy-brown-100 dark:hover:bg-gray-700'}
							{isDisabled(day)
							? 'opacity-40 cursor-not-allowed hover:bg-transparent dark:hover:bg-transparent'
							: ''}"
					>
						{day}
					</button>
				{/each}
			</div>
		</div>
	{/if}
</div>
