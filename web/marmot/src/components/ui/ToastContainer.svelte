<script lang="ts">
	import { toasts, type Toast } from '$lib/stores/toast';

	// The app's toasts as mm-toast elements in one mm-toaster. The store keeps owning each toast's lifetime, so the elements get no timer of their own and closing one removes it from the store.
	const TONES: Record<Toast['variant'], 'success' | 'danger' | 'info' | 'warning'> = {
		success: 'success',
		error: 'danger',
		info: 'info',
		warning: 'warning'
	};
</script>

<mm-toaster>
	{#each $toasts as toast (toast.id)}
		<mm-toast
			tone={TONES[toast.variant]}
			message={toast.message}
			duration={0}
			onmm-dismiss={() => toasts.remove(toast.id)}
		></mm-toast>
	{/each}
</mm-toaster>
