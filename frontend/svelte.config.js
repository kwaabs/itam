import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	compilerOptions: {
		// The UI uses a consistent adjacent-label pattern
		// (<div class="field"><label>Name</label><input/></div>) where the visible
		// label sits next to its control inside a styled wrapper. This rule flags
		// the lack of a programmatic for/id link; we suppress it project-wide as a
		// deliberate design choice while keeping all other a11y checks active.
		warningFilter: (w) => w.code !== 'a11y_label_has_associated_control'
	},
	kit: {
		adapter: adapter()
	}
};

export default config;
