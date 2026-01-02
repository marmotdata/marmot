<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import IconifyIcon from '@iconify/svelte';
	import AuthenticatedImage from '$components/ui/AuthenticatedImage.svelte';
	import type { ProductImageMeta } from '$lib/dataproducts/types';

	interface Props {
		productId: string;
		currentIconUrl?: string | null;
		onIconChange?: (iconUrl: string | null) => void;
		disabled?: boolean;
		size?: 'sm' | 'md' | 'lg';
	}

	let {
		productId,
		currentIconUrl = null,
		onIconChange,
		disabled = false,
		size = 'lg'
	}: Props = $props();

	let showModal = $state(false);
	let uploading = $state(false);
	let error = $state('');
	let fileInput: HTMLInputElement;

	// Cropper state
	let imageSrc = $state<string | null>(null);
	let imageElement = $state<HTMLImageElement | null>(null);
	let cropArea = $state({ x: 0, y: 0, size: 100 });
	let isDragging = $state(false);
	let dragStartPos = $state({ x: 0, y: 0 });
	let cropStartPos = $state({ x: 0, y: 0 });
	let imageOffset = $state({ x: 0, y: 0 });
	let canvasRef: HTMLCanvasElement;

	// Size classes
	const sizeClasses = {
		sm: 'w-10 h-10',
		md: 'w-12 h-12',
		lg: 'w-14 h-14'
	};

	const iconSizeClasses = {
		sm: 'w-5 h-5',
		md: 'w-6 h-6',
		lg: 'w-7 h-7'
	};

	function validateImage(file: File): Promise<boolean> {
		return new Promise((resolve) => {
			// Check file type
			if (!['image/jpeg', 'image/png', 'image/gif', 'image/webp'].includes(file.type)) {
				error = 'Please select a valid image file (JPEG, PNG, GIF, or WebP)';
				resolve(false);
				return;
			}

			// Check file size (5MB max)
			if (file.size > 5 * 1024 * 1024) {
				error = 'Image must be less than 5MB';
				resolve(false);
				return;
			}

			// Validate it's actually an image by trying to load it
			const img = new Image();
			img.onload = () => {
				URL.revokeObjectURL(img.src);
				// Check minimum dimensions
				if (img.width < 32 || img.height < 32) {
					error = 'Image must be at least 32x32 pixels';
					resolve(false);
					return;
				}
				resolve(true);
			};
			img.onerror = () => {
				URL.revokeObjectURL(img.src);
				error = 'Invalid image file';
				resolve(false);
			};
			img.src = URL.createObjectURL(file);
		});
	}

	async function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		error = '';

		const isValid = await validateImage(file);
		if (!isValid) {
			input.value = '';
			return;
		}

		const reader = new FileReader();
		reader.onload = (e) => {
			imageSrc = e.target?.result as string;
			showModal = true;
		};
		reader.readAsDataURL(file);
	}

	function handleImageLoad(event: Event) {
		const img = event.target as HTMLImageElement;
		imageElement = img;

		// Get the image's position relative to container
		const imgRect = img.getBoundingClientRect();
		const containerRect = img.parentElement?.getBoundingClientRect();

		if (containerRect) {
			imageOffset = {
				x: imgRect.left - containerRect.left,
				y: imgRect.top - containerRect.top
			};
		}

		// Calculate initial crop area - fit largest square centered on image
		const displayWidth = img.clientWidth;
		const displayHeight = img.clientHeight;
		const cropSize = Math.min(displayWidth, displayHeight, 200);

		cropArea = {
			x: (displayWidth - cropSize) / 2,
			y: (displayHeight - cropSize) / 2,
			size: cropSize
		};
	}

	function handleMouseDown(event: MouseEvent) {
		if (disabled) return;
		event.preventDefault();
		isDragging = true;
		dragStartPos = { x: event.clientX, y: event.clientY };
		cropStartPos = { x: cropArea.x, y: cropArea.y };
	}

	function handleMouseMove(event: MouseEvent) {
		if (!isDragging || !imageElement) return;

		const deltaX = event.clientX - dragStartPos.x;
		const deltaY = event.clientY - dragStartPos.y;

		let newX = cropStartPos.x + deltaX;
		let newY = cropStartPos.y + deltaY;

		// Constrain within image bounds
		const maxX = imageElement.clientWidth - cropArea.size;
		const maxY = imageElement.clientHeight - cropArea.size;

		newX = Math.max(0, Math.min(newX, maxX));
		newY = Math.max(0, Math.min(newY, maxY));

		cropArea = { ...cropArea, x: newX, y: newY };
	}

	function handleMouseUp() {
		isDragging = false;
	}

	async function handleCropAndUpload() {
		if (!imageElement || !canvasRef) return;

		uploading = true;
		error = '';

		try {
			// Calculate the scale between display and natural image size
			const scaleX = imageElement.naturalWidth / imageElement.clientWidth;
			const scaleY = imageElement.naturalHeight / imageElement.clientHeight;

			// Crop coordinates in natural image pixels
			const srcX = cropArea.x * scaleX;
			const srcY = cropArea.y * scaleY;
			const srcWidth = cropArea.size * scaleX;
			const srcHeight = cropArea.size * scaleY;

			// Draw cropped image to canvas (output 256x256 for icons)
			const outputSize = 256;
			canvasRef.width = outputSize;
			canvasRef.height = outputSize;
			const ctx = canvasRef.getContext('2d')!;

			ctx.drawImage(imageElement, srcX, srcY, srcWidth, srcHeight, 0, 0, outputSize, outputSize);

			// Convert canvas to blob
			const blob = await new Promise<Blob>((resolve, reject) => {
				canvasRef.toBlob(
					(b) => {
						if (b) resolve(b);
						else reject(new Error('Failed to create image blob'));
					},
					'image/png',
					0.9
				);
			});

			// Upload replaces any existing icon (backend handles upsert)
			const formData = new FormData();
			formData.append('file', blob, 'icon.png');

			const token = auth.getToken();
			const headers: Record<string, string> = {};
			if (token) {
				headers['Authorization'] = `Bearer ${token}`;
			}

			const response = await fetch(`/api/v1/products/images/${productId}/icon`, {
				method: 'POST',
				body: formData,
				headers
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to upload icon');
			}

			const meta: ProductImageMeta = await response.json();
			// Add cache-busting timestamp to force browser to fetch new image
			const cacheBustUrl = `${meta.url}?t=${Date.now()}`;
			onIconChange?.(cacheBustUrl);
			closeModal();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to upload icon';
		} finally {
			uploading = false;
		}
	}

	async function handleDeleteIcon() {
		if (!currentIconUrl) return;

		uploading = true;
		error = '';

		try {
			const token = auth.getToken();
			const headers: Record<string, string> = {
				'Content-Type': 'application/json'
			};
			if (token) {
				headers['Authorization'] = `Bearer ${token}`;
			}

			const response = await fetch(`/api/v1/products/images/${productId}/icon`, {
				method: 'DELETE',
				headers
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to delete icon');
			}

			onIconChange?.(null);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete icon';
		} finally {
			uploading = false;
		}
	}

	function closeModal() {
		showModal = false;
		imageSrc = null;
		imageElement = null;
		error = '';
		if (fileInput) fileInput.value = '';
	}

	function openFileDialog() {
		fileInput?.click();
	}
</script>

<svelte:window onmousemove={handleMouseMove} onmouseup={handleMouseUp} />

{#snippet defaultIcon()}
	<IconifyIcon
		icon="mdi:package-variant-closed"
		class="{iconSizeClasses[size]} text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
	/>
{/snippet}

<!-- Single icon with hover controls -->
<div class="relative group inline-block">
	<div
		class="{sizeClasses[
			size
		]} rounded-xl bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 flex items-center justify-center overflow-hidden"
	>
		{#if currentIconUrl}
			<AuthenticatedImage
				src={currentIconUrl}
				alt="Product icon"
				class="w-full h-full object-cover"
				fallback={defaultIcon}
			/>
		{:else}
			{@render defaultIcon()}
		{/if}
	</div>

	<!-- Hover overlay with edit/remove buttons -->
	{#if !disabled}
		<div
			class="absolute inset-0 rounded-xl bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-1"
		>
			<input
				type="file"
				accept="image/jpeg,image/png,image/gif,image/webp"
				class="hidden"
				bind:this={fileInput}
				onchange={handleFileSelect}
				{disabled}
			/>
			<button
				type="button"
				onclick={openFileDialog}
				disabled={disabled || uploading}
				class="p-1.5 rounded-lg bg-white/20 hover:bg-white/30 text-white transition-colors disabled:opacity-50"
				title={currentIconUrl ? 'Change icon' : 'Upload icon'}
			>
				<IconifyIcon icon="material-symbols:edit" class="w-4 h-4" />
			</button>
			{#if currentIconUrl}
				<button
					type="button"
					onclick={handleDeleteIcon}
					disabled={disabled || uploading}
					class="p-1.5 rounded-lg bg-white/20 hover:bg-red-500/80 text-white transition-colors disabled:opacity-50"
					title="Remove icon"
				>
					<IconifyIcon icon="material-symbols:delete-outline" class="w-4 h-4" />
				</button>
			{/if}
		</div>
	{/if}

	{#if error && !showModal}
		<div class="absolute top-full left-0 mt-1 z-10">
			<span
				class="text-xs text-red-600 dark:text-red-400 whitespace-nowrap bg-white dark:bg-gray-800 px-2 py-1 rounded shadow-lg"
			>
				{error}
			</span>
		</div>
	{/if}
</div>

<!-- Crop Modal -->
{#if showModal && imageSrc}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="fixed inset-0 bg-black/60 dark:bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4"
		onclick={(e) => e.target === e.currentTarget && closeModal()}
		onkeydown={(e) => e.key === 'Escape' && closeModal()}
		role="dialog"
		tabindex="-1"
	>
		<div
			class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-lg w-full border border-gray-200 dark:border-gray-700"
			onclick={(e) => e.stopPropagation()}
			onkeydown={() => {}}
			role="document"
		>
			<div class="p-4 border-b border-gray-200 dark:border-gray-700">
				<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Crop Icon</h3>
				<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
					Drag the selection to position. Icons will be cropped to a square.
				</p>
			</div>

			<div class="p-4">
				<!-- Image with crop overlay -->
				<div
					class="relative bg-gray-100 dark:bg-gray-900 rounded-lg overflow-hidden flex items-center justify-center select-none"
					style="max-height: 400px;"
				>
					<img
						src={imageSrc}
						alt="Preview"
						class="max-w-full max-h-[400px] object-contain pointer-events-none"
						onload={handleImageLoad}
						draggable="false"
					/>

					{#if imageElement}
						<!-- Dark overlay with transparent crop window -->
						<div
							class="absolute pointer-events-none"
							style="
								left: {imageOffset.x}px;
								top: {imageOffset.y}px;
								width: {imageElement.clientWidth}px;
								height: {imageElement.clientHeight}px;
								background: rgba(0, 0, 0, 0.5);
								clip-path: polygon(
									0% 0%,
									0% 100%,
									{cropArea.x}px 100%,
									{cropArea.x}px {cropArea.y}px,
									{cropArea.x + cropArea.size}px {cropArea.y}px,
									{cropArea.x + cropArea.size}px {cropArea.y + cropArea.size}px,
									{cropArea.x}px {cropArea.y + cropArea.size}px,
									{cropArea.x}px 100%,
									100% 100%,
									100% 0%
								);
							"
						></div>

						<!-- Crop selection box -->
						<div
							class="absolute border-2 border-white shadow-lg cursor-move"
							style="
								left: {imageOffset.x + cropArea.x}px;
								top: {imageOffset.y + cropArea.y}px;
								width: {cropArea.size}px;
								height: {cropArea.size}px;
								box-shadow: 0 0 0 9999px rgba(0, 0, 0, 0);
							"
							onmousedown={handleMouseDown}
							role="slider"
							aria-label="Crop selection"
							aria-valuenow={cropArea.x}
							tabindex="0"
						>
							<!-- Corner indicators -->
							<div class="absolute -top-1 -left-1 w-3 h-3 bg-white rounded-full shadow"></div>
							<div class="absolute -top-1 -right-1 w-3 h-3 bg-white rounded-full shadow"></div>
							<div class="absolute -bottom-1 -left-1 w-3 h-3 bg-white rounded-full shadow"></div>
							<div class="absolute -bottom-1 -right-1 w-3 h-3 bg-white rounded-full shadow"></div>

							<!-- Grid lines for visual aid -->
							<div
								class="absolute inset-0 pointer-events-none"
								style="
									background-image:
										linear-gradient(to right, rgba(255,255,255,0.3) 1px, transparent 1px),
										linear-gradient(to bottom, rgba(255,255,255,0.3) 1px, transparent 1px);
									background-size: 33.33% 33.33%;
								"
							></div>
						</div>
					{/if}
				</div>

				<!-- Hidden canvas for cropping -->
				<canvas bind:this={canvasRef} class="hidden"></canvas>

				{#if error}
					<div
						class="mt-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-sm text-red-800 dark:text-red-200"
					>
						{error}
					</div>
				{/if}
			</div>

			<div class="p-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-2">
				<button
					type="button"
					onclick={closeModal}
					disabled={uploading}
					class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={handleCropAndUpload}
					disabled={uploading}
					class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 rounded-lg disabled:opacity-50"
				>
					{#if uploading}
						<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
						Uploading...
					{:else}
						<IconifyIcon icon="material-symbols:check" class="w-4 h-4" />
						Save Icon
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
