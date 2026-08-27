<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	// Power & environment live on the location drill-down, not a separate global page.
	onMount(() => {
		const loc = $page.url.searchParams.get('loc');
		goto(loc ? `/datacenter?loc=${loc}#power-env` : '/datacenter#power-env', { replaceState: true });
	});
</script>

<p class="muted">Redirecting…</p>
