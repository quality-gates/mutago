export const title = 'Index and path-cache lifecycle';
export const state = v => `Collected ${v.seen.length} nodes; safety ${v.safetyBuilt ? 'built' : 'empty'}; resolved ${v.cached.length} paths`;
export const step = action => ({Ensure:'Query range', BuildSafety:'Build safety index', ClearSafety:'Clear safety index', ClearIndexes:'Clear AST indexes', Resolve:'Resolve coverage path'})[action] || action;
