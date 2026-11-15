export const title = 'Cache and annotation publication';
export const state = v => v.cache === 'empty' ? 'Package absent' : 'Package cached';
export const step = a => ({Publish:'Publish a loaded package', Clear:'Clear the cache', Edit:'Change collected exclusions', Store:'Publish owner exclusions', Release:'Release owner exclusions'})[a] || a;
