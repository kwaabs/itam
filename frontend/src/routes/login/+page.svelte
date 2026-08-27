<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { login, azureLogin, azureEnabled } from '$lib/auth';

	let email = $state('admin@itam.local');
	let password = $state('admin12345');
	let error = $state('');
	let busy = $state(false);
	let ssoEnabled = $state(false);

	onMount(async () => {
		ssoEnabled = await azureEnabled();
		const params = new URLSearchParams(window.location.search);
		const ssoErr = params.get('sso_error');
		if (ssoErr) {
			error = `Microsoft sign-in failed: ${ssoErr}`;
			history.replaceState(null, '', window.location.pathname);
		}
	});

	async function submit(e: Event) {
		e.preventDefault();
		error = '';
		busy = true;
		try {
			await login(email, password);
			goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Something went wrong';
		} finally {
			busy = false;
		}
	}
</script>

<div class="login-shell">
	<div class="card login-panel">
		<div class="brand">IT<span>AM</span></div>
		<p class="login-tagline">IT Asset Management</p>

		<form onsubmit={submit}>
			<div class="field">
				<label for="email">Email</label>
				<input id="email" type="email" bind:value={email} required autocomplete="username" />
			</div>
			<div class="field">
				<label for="password">Password</label>
				<input id="password" type="password" bind:value={password} required autocomplete="current-password" />
			</div>
			{#if error}<p class="error">{error}</p>{/if}
			<button class="btn" type="submit" style="width:100%" disabled={busy}>
				{busy ? 'Please wait…' : 'Sign in'}
			</button>
		</form>

		{#if ssoEnabled}
			<div class="login-divider">or</div>
			<button class="btn microsoft" style="width:100%" onclick={azureLogin}>
				<span class="ms-logo" aria-hidden="true"><i></i><i></i><i></i><i></i></span>
				Sign in with Microsoft
			</button>
		{/if}
	</div>
</div>
