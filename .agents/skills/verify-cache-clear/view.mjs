export const title = 'Package cache clear and in-flight loads';
export const state = v => `${v.cache === 'empty' ? 'Cache empty' : 'Package cached'}; ${v.loading ? (v.fresh ? 'Load current' : 'Load predates clear') : 'No load in flight'}`;
export const step = a => ({StartLoad: 'Start a package load', Publish: 'Finish loading', Clear: 'Clear the cache'})[a] || a;
