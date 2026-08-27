export { matchers } from './matchers.js';

export const nodes = [
	() => import('./nodes/0'),
	() => import('./nodes/1'),
	() => import('./nodes/2'),
	() => import('./nodes/3'),
	() => import('./nodes/4'),
	() => import('./nodes/5'),
	() => import('./nodes/6'),
	() => import('./nodes/7'),
	() => import('./nodes/8'),
	() => import('./nodes/9'),
	() => import('./nodes/10'),
	() => import('./nodes/11'),
	() => import('./nodes/12'),
	() => import('./nodes/13'),
	() => import('./nodes/14'),
	() => import('./nodes/15'),
	() => import('./nodes/16'),
	() => import('./nodes/17'),
	() => import('./nodes/18'),
	() => import('./nodes/19'),
	() => import('./nodes/20'),
	() => import('./nodes/21'),
	() => import('./nodes/22'),
	() => import('./nodes/23'),
	() => import('./nodes/24'),
	() => import('./nodes/25'),
	() => import('./nodes/26'),
	() => import('./nodes/27'),
	() => import('./nodes/28'),
	() => import('./nodes/29'),
	() => import('./nodes/30'),
	() => import('./nodes/31'),
	() => import('./nodes/32'),
	() => import('./nodes/33')
];

export const server_loads = [];

export const dictionary = {
		"/": [2],
		"/admin": [3],
		"/assets": [4],
		"/assets/new": [5],
		"/assets/[id]": [6],
		"/audit": [7],
		"/capacity": [8],
		"/costs": [9],
		"/datacenter": [10],
		"/datacenter/power": [11],
		"/datacenter/[id]": [12],
		"/iam": [13],
		"/import": [14],
		"/integrations": [15],
		"/ipam": [16],
		"/locations": [17],
		"/login": [18],
		"/netconfig": [19],
		"/network": [20],
		"/notifications": [21],
		"/org-units": [22],
		"/people": [23],
		"/peripherals": [24],
		"/procurement": [25],
		"/procurement/[id]": [26],
		"/reports": [27],
		"/software": [28],
		"/stock": [29],
		"/storage": [30],
		"/stores": [31],
		"/vendors": [32],
		"/virtualization": [33]
	};

export const hooks = {
	handleError: (({ error }) => { console.error(error) }),
	
	reroute: (() => {}),
	transport: {}
};

export const decoders = Object.fromEntries(Object.entries(hooks.transport).map(([k, v]) => [k, v.decode]));
export const encoders = Object.fromEntries(Object.entries(hooks.transport).map(([k, v]) => [k, v.encode]));

export const hash = false;

export const decode = (type, value) => decoders[type](value);

export { default as root } from '../root.js';