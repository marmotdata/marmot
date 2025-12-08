<script lang="ts">
	export let name: string;
	export let profilePicture: string | null | undefined = null;
	export let size: 'xs' | 'sm' | 'md' | 'lg' = 'md';

	// Extract initials from name
	function getInitials(name: string): string {
		const parts = name.trim().split(/\s+/);
		if (parts.length >= 2) {
			return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
		}
		return name.substring(0, 2).toUpperCase();
	}

	// Generate a consistent color based on the name
	function getColorForName(name: string): string {
		const colors = [
			'bg-red-500',
			'bg-orange-500',
			'bg-amber-500',
			'bg-yellow-500',
			'bg-lime-500',
			'bg-green-500',
			'bg-emerald-500',
			'bg-teal-500',
			'bg-cyan-500',
			'bg-sky-500',
			'bg-blue-500',
			'bg-indigo-500',
			'bg-violet-500',
			'bg-purple-500',
			'bg-fuchsia-500',
			'bg-pink-500',
			'bg-rose-500'
		];

		// Simple hash function based on the name
		let hash = 0;
		for (let i = 0; i < name.length; i++) {
			hash = name.charCodeAt(i) + ((hash << 5) - hash);
		}
		const index = Math.abs(hash) % colors.length;
		return colors[index];
	}

	const sizeClasses = {
		xs: 'h-6 w-6 text-xs',
		sm: 'h-8 w-8 text-xs',
		md: 'h-10 w-10 text-sm',
		lg: 'h-12 w-12 text-base'
	};

	$: initials = getInitials(name);
	$: bgColor = getColorForName(name);
	$: sizeClass = sizeClasses[size];
</script>

{#if profilePicture}
	<img
		src={profilePicture}
		alt={name}
		class="rounded-full {sizeClass} object-cover"
	/>
{:else}
	<div
		class="rounded-full {sizeClass} {bgColor} flex items-center justify-center text-white font-semibold"
	>
		{initials}
	</div>
{/if}
