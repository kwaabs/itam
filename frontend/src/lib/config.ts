import { env } from '$env/dynamic/public';

export const API_URL = env.PUBLIC_API_URL ?? 'http://localhost:5607';
export const GOTRUE_URL = env.PUBLIC_GOTRUE_URL ?? 'http://localhost:5606';
