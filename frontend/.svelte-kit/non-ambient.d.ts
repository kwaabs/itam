
// this file is generated — do not edit it


declare module "svelte/elements" {
	export interface HTMLAttributes<T> {
		'data-sveltekit-keepfocus'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-noscroll'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-preload-code'?:
			| true
			| ''
			| 'eager'
			| 'viewport'
			| 'hover'
			| 'tap'
			| 'off'
			| undefined
			| null;
		'data-sveltekit-preload-data'?: true | '' | 'hover' | 'tap' | 'off' | undefined | null;
		'data-sveltekit-reload'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-replacestate'?: true | '' | 'off' | undefined | null;
	}
}

export {};


declare module "$app/types" {
	type MatcherParam<M> = M extends (param : string) => param is (infer U extends string) ? U : string;

	export interface AppTypes {
		RouteId(): "/" | "/admin" | "/assets" | "/assets/new" | "/assets/[id]" | "/audit" | "/capacity" | "/costs" | "/datacenter" | "/datacenter/power" | "/datacenter/[id]" | "/iam" | "/import" | "/integrations" | "/ipam" | "/locations" | "/login" | "/netconfig" | "/network" | "/notifications" | "/org-units" | "/people" | "/peripherals" | "/procurement" | "/procurement/[id]" | "/reports" | "/software" | "/stock" | "/storage" | "/stores" | "/vendors" | "/virtualization";
		RouteParams(): {
			"/assets/[id]": { id: string };
			"/datacenter/[id]": { id: string };
			"/procurement/[id]": { id: string }
		};
		LayoutParams(): {
			"/": { id?: string | undefined };
			"/admin": Record<string, never>;
			"/assets": { id?: string | undefined };
			"/assets/new": Record<string, never>;
			"/assets/[id]": { id: string };
			"/audit": Record<string, never>;
			"/capacity": Record<string, never>;
			"/costs": Record<string, never>;
			"/datacenter": { id?: string | undefined };
			"/datacenter/power": Record<string, never>;
			"/datacenter/[id]": { id: string };
			"/iam": Record<string, never>;
			"/import": Record<string, never>;
			"/integrations": Record<string, never>;
			"/ipam": Record<string, never>;
			"/locations": Record<string, never>;
			"/login": Record<string, never>;
			"/netconfig": Record<string, never>;
			"/network": Record<string, never>;
			"/notifications": Record<string, never>;
			"/org-units": Record<string, never>;
			"/people": Record<string, never>;
			"/peripherals": Record<string, never>;
			"/procurement": { id?: string | undefined };
			"/procurement/[id]": { id: string };
			"/reports": Record<string, never>;
			"/software": Record<string, never>;
			"/stock": Record<string, never>;
			"/storage": Record<string, never>;
			"/stores": Record<string, never>;
			"/vendors": Record<string, never>;
			"/virtualization": Record<string, never>
		};
		Pathname(): "/" | "/admin" | "/assets" | "/assets/new" | `/assets/${string}` & {} | "/audit" | "/capacity" | "/costs" | "/datacenter" | "/datacenter/power" | `/datacenter/${string}` & {} | "/iam" | "/import" | "/integrations" | "/ipam" | "/locations" | "/login" | "/netconfig" | "/network" | "/notifications" | "/org-units" | "/people" | "/peripherals" | "/procurement" | `/procurement/${string}` & {} | "/reports" | "/software" | "/stock" | "/storage" | "/stores" | "/vendors" | "/virtualization";
		ResolvedPathname(): `${"" | `/${string}`}${ReturnType<AppTypes['Pathname']>}`;
		Asset(): string & {};
	}
}