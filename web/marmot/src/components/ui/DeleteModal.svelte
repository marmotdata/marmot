<script lang="ts">
	import { m } from '$lib/paraglide/messages';

	// Kept for pages not yet moved to mm-confirm-dialog directly: the confirmation every destructive action goes through, optionally asking for the resource name to be typed (the element's mono field) before the crimson button unlocks.
	interface Props {
		show?: boolean;
		title: string;
		message: string;
		confirmText?: string;
		resourceName?: string;
		requireConfirmation?: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let {
		show = $bindable(false),
		title,
		message,
		confirmText = m.common_delete(),
		resourceName = '',
		requireConfirmation = false,
		onConfirm,
		onCancel
	}: Props = $props();

	// The caller closes the dialog once the deletion has gone through, so the element must not close itself on confirm.
	function handleConfirm(e: Event) {
		e.preventDefault();
		onConfirm();
	}

	function handleClose() {
		if (show) onCancel();
	}
</script>

<mm-confirm-dialog
	open={show}
	heading={title}
	body={message}
	confirm-label={confirmText}
	confirm-text={requireConfirmation ? resourceName : ''}
	tone="danger"
	onmm-confirm={handleConfirm}
	onmm-close={handleClose}
></mm-confirm-dialog>
