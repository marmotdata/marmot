<script lang="ts">
	export let show = false;
	export let title: string;
	export let message: string;
	export let confirmText = 'Delete';
	export let resourceName = '';
	export let requireConfirmation = false;
	export let onConfirm: () => void;
	export let onCancel: () => void;

	let inputText = '';
	$: isConfirmEnabled = !requireConfirmation || (resourceName && inputText === resourceName);

	$: if (!show) {
		inputText = '';
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && isConfirmEnabled) {
			event.preventDefault();
			onConfirm();
		} else if (event.key === 'Escape') {
			event.preventDefault();
			onCancel();
		}
	}
</script>

{#if show}
	<div class="fixed z-50 inset-0 overflow-y-auto">
		<div class="flex items-center justify-center min-h-screen p-4">
			<div
				class="fixed inset-0 bg-gray-50 dark:bg-gray-800 dark:bg-gray-9000 bg-opacity-75 transition-opacity"
				on:click={onCancel}
			/>

			<div
				role="dialog"
				aria-modal="true"
				aria-labelledby="modal-title"
				class="relative bg-earthy-brown-50 dark:bg-gray-900 dark:bg-gray-900 rounded-lg px-4 pt-5 pb-4 text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:max-w-lg sm:w-full sm:p-6"
			>
				<div class="sm:flex sm:items-start">
					<div class="mt-3 text-center sm:mt-0 sm:text-left w-full">
						<h3
							id="modal-title"
							class="text-lg leading-6 font-medium text-gray-900 dark:text-gray-100 dark:text-gray-100 dark:text-gray-100 dark:text-gray-200"
						>
							{title}
						</h3>
						<div class="mt-2">
							<p
								class="text-sm text-gray-500 dark:text-gray-500 dark:text-gray-500 dark:text-gray-500 dark:text-gray-500 dark:text-gray-500"
							>
								{message}
							</p>
						</div>
						{#if requireConfirmation}
							<div class="mt-4">
								<label
									for="confirm-text"
									class="block text-sm font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 dark:text-gray-300"
								>
									Type "{resourceName}" to confirm deletion
								</label>
								<input
									type="text"
									id="confirm-text"
									bind:value={inputText}
									class="mt-1 block w-full px-3 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 border border-gray-300 dark:border-gray-600 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:focus:border-earthy-terracotta-500 sm:text-sm"
									on:keydown={handleKeydown}
								/>
							</div>
						{/if}
					</div>
				</div>
				<div class="mt-5 sm:mt-4 sm:flex sm:flex-row-reverse">
					<button
						type="button"
						class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-red-800 text-base font-medium text-white hover:bg-red-900 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-700 disabled:opacity-50 disabled:cursor-not-allowed sm:ml-3 sm:w-auto sm:text-sm"
						disabled={!isConfirmEnabled}
						on:click={onConfirm}
					>
						{confirmText}
					</button>
					<button
						type="button"
						class="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 dark:border-gray-600 dark:border-gray-600 shadow-sm px-4 py-2 bg-white dark:bg-gray-800 dark:bg-gray-800 text-base font-medium text-gray-700 dark:text-gray-300 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 dark:bg-gray-800 dark:bg-gray-900 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-600 dark:focus:ring-earthy-terracotta-600 sm:mt-0 sm:w-auto sm:text-sm"
						on:click={onCancel}
					>
						Cancel
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
