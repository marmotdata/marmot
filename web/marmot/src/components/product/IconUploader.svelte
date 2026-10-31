<script lang="ts">
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { toast } from '@marmotdata/elements/toast.js';
	import type { ProductImageMeta } from '$lib/dataproducts/types';
	import ProductIcon from './ProductIcon.svelte';

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

	const OUTPUT_SIZE = 256;
	const KEY_STEP = 8;

	let trigger = $state<HTMLButtonElement | null>(null);
	let fileInput = $state<HTMLInputElement | null>(null);
	let canvas = $state<HTMLCanvasElement | null>(null);
	let menuOpen = $state(false);
	let dialogOpen = $state(false);
	let uploading = $state(false);
	let error = $state('');

	let imageSrc = $state<string | null>(null);
	let image = $state<HTMLImageElement | null>(null);
	let crop = $state({ x: 0, y: 0, size: 100 });
	let offset = $state({ x: 0, y: 0 });
	let dragging = false;
	let dragStart = { x: 0, y: 0 };
	let cropStart = { x: 0, y: 0 };

	function validate(file: File): Promise<string | null> {
		return new Promise((resolve) => {
			if (!['image/jpeg', 'image/png', 'image/gif', 'image/webp'].includes(file.type)) {
				resolve(m.products_icon_invalid_type());
				return;
			}
			if (file.size > 5 * 1024 * 1024) {
				resolve(m.products_icon_too_large());
				return;
			}
			const img = new Image();
			img.onload = () => {
				URL.revokeObjectURL(img.src);
				resolve(img.width < 32 || img.height < 32 ? m.products_icon_too_small() : null);
			};
			img.onerror = () => {
				URL.revokeObjectURL(img.src);
				resolve(m.products_icon_invalid_file());
			};
			img.src = URL.createObjectURL(file);
		});
	}

	async function handleFile(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		const problem = await validate(file);
		if (problem) {
			input.value = '';
			toast({ tone: 'danger', message: problem });
			return;
		}
		const reader = new FileReader();
		reader.onload = (e) => {
			imageSrc = e.target?.result as string;
			error = '';
			dialogOpen = true;
		};
		reader.readAsDataURL(file);
	}

	function handleImageLoad(event: Event) {
		const img = event.target as HTMLImageElement;
		image = img;
		const imgRect = img.getBoundingClientRect();
		const box = img.parentElement?.getBoundingClientRect();
		if (box) offset = { x: imgRect.left - box.left, y: imgRect.top - box.top };
		const sizePx = Math.min(img.clientWidth, img.clientHeight, 200);
		crop = { x: (img.clientWidth - sizePx) / 2, y: (img.clientHeight - sizePx) / 2, size: sizePx };
	}

	function moveTo(x: number, y: number) {
		if (!image) return;
		crop = {
			...crop,
			x: Math.max(0, Math.min(x, image.clientWidth - crop.size)),
			y: Math.max(0, Math.min(y, image.clientHeight - crop.size))
		};
	}

	function handlePointerDown(event: PointerEvent) {
		event.preventDefault();
		dragging = true;
		dragStart = { x: event.clientX, y: event.clientY };
		cropStart = { x: crop.x, y: crop.y };
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
	}

	function handlePointerMove(event: PointerEvent) {
		if (!dragging) return;
		moveTo(cropStart.x + event.clientX - dragStart.x, cropStart.y + event.clientY - dragStart.y);
	}

	// Arrow keys move the selection too, so cropping works without a pointer.
	function handleCropKey(event: KeyboardEvent) {
		const moves: Record<string, [number, number]> = {
			ArrowLeft: [-KEY_STEP, 0],
			ArrowRight: [KEY_STEP, 0],
			ArrowUp: [0, -KEY_STEP],
			ArrowDown: [0, KEY_STEP]
		};
		const move = moves[event.key];
		if (!move) return;
		event.preventDefault();
		moveTo(crop.x + move[0], crop.y + move[1]);
	}

	function authHeaders(): Record<string, string> {
		const token = auth.getToken();
		return token ? { Authorization: `Bearer ${token}` } : {};
	}

	async function upload() {
		if (!image || !canvas) return;
		uploading = true;
		error = '';
		try {
			const scaleX = image.naturalWidth / image.clientWidth;
			const scaleY = image.naturalHeight / image.clientHeight;
			canvas.width = OUTPUT_SIZE;
			canvas.height = OUTPUT_SIZE;
			canvas
				.getContext('2d')!
				.drawImage(
					image,
					crop.x * scaleX,
					crop.y * scaleY,
					crop.size * scaleX,
					crop.size * scaleY,
					0,
					0,
					OUTPUT_SIZE,
					OUTPUT_SIZE
				);
			const target = canvas;
			const blob = await new Promise<Blob>((resolve, reject) => {
				target.toBlob(
					(b) => (b ? resolve(b) : reject(new Error(m.products_icon_blob_error()))),
					'image/png',
					0.9
				);
			});
			const form = new FormData();
			form.append('file', blob, 'icon.png');
			const response = await fetch(`/api/v1/products/images/${productId}/icon`, {
				method: 'POST',
				body: form,
				headers: authHeaders()
			});
			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				throw new Error(data.error || m.products_icon_upload_error());
			}
			const meta: ProductImageMeta = await response.json();
			// The URL stays the same after a replace, so a timestamp makes the browser fetch the new image.
			onIconChange?.(`${meta.url}?t=${Date.now()}`);
			closeDialog();
		} catch (err) {
			error = err instanceof Error ? err.message : m.products_icon_upload_error();
		} finally {
			uploading = false;
		}
	}

	async function removeIcon() {
		if (!currentIconUrl) return;
		uploading = true;
		try {
			const response = await fetch(`/api/v1/products/images/${productId}/icon`, {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json', ...authHeaders() }
			});
			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				throw new Error(data.error || m.products_icon_delete_error());
			}
			onIconChange?.(null);
		} catch (err) {
			toast({
				tone: 'danger',
				message: err instanceof Error ? err.message : m.products_icon_delete_error()
			});
		} finally {
			uploading = false;
		}
	}

	function closeDialog() {
		dialogOpen = false;
		imageSrc = null;
		image = null;
		error = '';
		if (fileInput) fileInput.value = '';
	}

	function handleTrigger() {
		if (currentIconUrl) menuOpen = true;
		else fileInput?.click();
	}

	function handleMenu(event: CustomEvent<{ value: string }>) {
		menuOpen = false;
		if (event.detail.value === 'upload') fileInput?.click();
		else if (event.detail.value === 'remove') removeIcon();
	}
</script>

{#if disabled}
	<ProductIcon iconUrl={currentIconUrl} {size} />
{:else}
	<button
		bind:this={trigger}
		type="button"
		class="trigger"
		aria-label={currentIconUrl ? m.products_icon_change_title() : m.products_icon_upload_title()}
		aria-haspopup={currentIconUrl ? 'menu' : undefined}
		aria-expanded={currentIconUrl ? menuOpen : undefined}
		disabled={uploading}
		onclick={handleTrigger}
	>
		<ProductIcon iconUrl={currentIconUrl} {size} />
		<span class="badge" aria-hidden="true"><mm-icon name="edit" size={12}></mm-icon></span>
	</button>
	<input
		bind:this={fileInput}
		type="file"
		accept="image/jpeg,image/png,image/gif,image/webp"
		class="mm-visually-hidden"
		tabindex="-1"
		aria-hidden="true"
		onchange={handleFile}
	/>
	{#if currentIconUrl && trigger}
		<mm-menu
			anchor={trigger}
			open={menuOpen}
			label={m.products_icon_change_title()}
			onmm-select={handleMenu}
			onmm-close={() => (menuOpen = false)}
		>
			<mm-menu-item value="upload" icon="upload">{m.products_icon_upload_new()}</mm-menu-item>
			<mm-menu-separator></mm-menu-separator>
			<mm-menu-item value="remove" icon="delete" danger
				>{m.products_icon_remove_title()}</mm-menu-item
			>
		</mm-menu>
	{/if}
{/if}

<mm-dialog
	open={dialogOpen}
	heading={m.products_icon_crop_heading()}
	description={m.products_icon_crop_hint()}
	onmm-close={closeDialog}
>
	{#if imageSrc}
		<div class="stage">
			<img
				src={imageSrc}
				alt={m.products_icon_preview_alt()}
				class="source"
				onload={handleImageLoad}
				draggable="false"
			/>
			{#if image}
				<div
					class="shade"
					style:left={`${offset.x}px`}
					style:top={`${offset.y}px`}
					style:width={`${image.clientWidth}px`}
					style:height={`${image.clientHeight}px`}
					style:clip-path={`polygon(0% 0%, 0% 100%, ${crop.x}px 100%, ${crop.x}px ${crop.y}px, ${crop.x + crop.size}px ${crop.y}px, ${crop.x + crop.size}px ${crop.y + crop.size}px, ${crop.x}px ${crop.y + crop.size}px, ${crop.x}px 100%, 100% 100%, 100% 0%)`}
				></div>
				<div
					class="selection"
					style:left={`${offset.x + crop.x}px`}
					style:top={`${offset.y + crop.y}px`}
					style:width={`${crop.size}px`}
					style:height={`${crop.size}px`}
					onpointerdown={handlePointerDown}
					onpointermove={handlePointerMove}
					onpointerup={() => (dragging = false)}
					onkeydown={handleCropKey}
					role="slider"
					aria-label={m.products_icon_crop_selection_aria()}
					aria-valuenow={Math.round(crop.x)}
					aria-valuetext={`${Math.round(crop.x)}, ${Math.round(crop.y)}`}
					tabindex="0"
				></div>
			{/if}
		</div>
		<canvas bind:this={canvas} hidden></canvas>
		{#if error}
			<mm-callout tone="danger" compact heading={error}></mm-callout>
		{/if}
	{/if}
	<mm-button slot="footer" onclick={closeDialog} disabled={uploading}>{m.common_cancel()}</mm-button
	>
	<mm-button slot="footer" variant="primary" icon="check" loading={uploading} onclick={upload}>
		{m.products_icon_save()}
	</mm-button>
</mm-dialog>

<style>
	.trigger {
		position: relative;
		display: inline-flex;
		padding: 0;
		border: 0;
		background: none;
		border-radius: var(--radius-md);
		cursor: pointer;
	}
	.trigger:hover :global(.tile) {
		border-color: var(--mm-control-border);
	}
	.badge {
		position: absolute;
		inset-block-end: calc(var(--space-1) * -1);
		inset-inline-end: calc(var(--space-1) * -1);
		display: grid;
		place-items: center;
		width: 20px;
		height: 20px;
		border: 1px solid var(--mm-control-border);
		border-radius: var(--radius-sm);
		background: var(--mm-surface);
		color: var(--mm-text-2);
	}
	.stage {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		max-height: 400px;
		overflow: hidden;
		border-radius: var(--radius-sm);
		background: var(--mm-sunken);
		user-select: none;
	}
	.source {
		max-width: 100%;
		max-height: 400px;
		object-fit: contain;
		pointer-events: none;
	}
	.shade {
		position: absolute;
		pointer-events: none;
		background: color-mix(in srgb, var(--mm-code-bg) 55%, transparent);
	}
	.selection {
		position: absolute;
		border: 2px solid var(--mm-paper);
		outline-offset: 2px;
		cursor: move;
		touch-action: none;
	}
</style>
