<script>
	import Search from '../components/Search.svelte';
	import AssetOverview from '../components/AssetOverview.svelte';
</script>

<div class="flex flex-col items-center w-full max-w-2xl mx-auto">
	<div class="mb-8">
		<img src="/images/marmot.svg" alt="Your Logo" class="h-48 w-auto" />
	</div>

	<Search />
	<AssetOverview />
</div>
