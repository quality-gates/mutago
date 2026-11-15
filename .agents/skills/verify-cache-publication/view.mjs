export const title = 'Cache and annotation publication';
export const state = v => `${v.cache === 'empty' ? 'Package absent' : 'Package cached'}; ${v.snapshot.length ? 'Exclusions present' : 'No exclusions'}`;
export const step = a => ({Publish:'Publish a loaded package', Clear:'Clear the cache', Edit:'Change collected exclusions', Store:'Publish exclusions'})[a] || a;
