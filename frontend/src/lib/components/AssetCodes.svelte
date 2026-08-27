<script lang="ts">
	import QRCode from 'qrcode';
	import JsBarcode from 'jsbarcode';

	let {
		tag,
		url = '',
		name = '',
		compact = false
	}: { tag: string; url?: string; name?: string; compact?: boolean } = $props();

	let qrSvg = $state('');
	let barSvg = $state('');

	// The QR encodes a deep link (scan -> open asset); the barcode encodes the
	// asset tag (handheld scanners that "type" the tag into a search box).
	const target = $derived(url || tag);

	async function render() {
		if (!tag) return;
		try {
			qrSvg = await QRCode.toString(target, { type: 'svg', margin: 0, width: compact ? 64 : 150 });
		} catch {
			qrSvg = '';
		}
		try {
			const el = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
			JsBarcode(el, tag, {
				format: 'CODE128',
				displayValue: !compact,
				fontSize: 13,
				height: compact ? 28 : 46,
				width: compact ? 1 : 1.6,
				margin: 0
			});
			barSvg = el.outerHTML;
		} catch {
			barSvg = '';
		}
	}

	$effect(() => {
		void target;
		void compact;
		render();
	});

	export function printLabel() {
		const w = window.open('', '_blank', 'width=460,height=340');
		if (!w) return;
		w.document.write(`<!doctype html><html><head><title>Label ${tag}</title><style>
			*{box-sizing:border-box}
			body{font-family:system-ui,-apple-system,sans-serif;margin:0;padding:16px;background:#fff;color:#000}
			.label{width:300px;border:1px solid #000;border-radius:8px;padding:12px;display:flex;gap:12px;align-items:center}
			.qr{flex:0 0 auto}
			.qr svg{display:block;width:96px;height:96px}
			.meta{min-width:0}
			.nm{font-weight:700;font-size:14px;margin-bottom:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:170px}
			.tg{font-size:11px;color:#444;margin-bottom:6px}
			.bar svg{max-width:170px}
			.actions{margin-top:14px}
			button{font:inherit;padding:6px 12px;border:1px solid #888;border-radius:6px;background:#f3f3f3;cursor:pointer}
			@media print{.actions{display:none}body{padding:0}.label{border-color:#000}}
		</style></head><body>
			<div class="label">
				<div class="qr">${qrSvg}</div>
				<div class="meta">
					<div class="nm">${name || tag}</div>
					<div class="tg">${tag}</div>
					<div class="bar">${barSvg}</div>
				</div>
			</div>
			<div class="actions"><button onclick="window.print()">Print</button></div>
		</body></html>`);
		w.document.close();
	}
</script>

{#if compact}
	<div class="codes compact" title={tag}>
		{#if qrSvg}<div class="qr">{@html qrSvg}</div>{/if}
	</div>
{:else}
	<div class="codes">
		<div class="qr">{@html qrSvg}</div>
		<div class="bar">
			{@html barSvg}
			<button class="btn secondary small" onclick={printLabel}>Print label</button>
		</div>
	</div>
{/if}

<style>
	.codes {
		display: flex;
		gap: 18px;
		align-items: center;
	}
	.codes .qr :global(svg) {
		display: block;
		width: 150px;
		height: 150px;
		background: #fff;
		padding: 6px;
		border-radius: 6px;
	}
	.codes .bar {
		display: flex;
		flex-direction: column;
		gap: 8px;
		align-items: flex-start;
	}
	.codes .bar :global(svg) {
		background: #fff;
		padding: 6px 8px;
		border-radius: 6px;
		max-width: 100%;
	}
	.codes.compact .qr :global(svg) {
		display: block;
		width: 56px;
		height: 56px;
		background: #fff;
		padding: 3px;
		border-radius: 4px;
	}
</style>
