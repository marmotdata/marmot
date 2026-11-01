<script lang="ts">
	import DataProductForm from '$components/product/DataProductForm.svelte';
	import { m } from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.products_new_page_title()}</title>
</svelte:head>

<DataProductForm />
