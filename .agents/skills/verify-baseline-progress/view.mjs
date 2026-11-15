export const title = 'Baseline and terminal progress';
export const state = v => `${v.present ? 'Baseline loaded' : 'Baseline absent'}; progress ${v.line}`;
export const step = action => ({Capture:'Capture baseline', Missing:'Remove baseline', Compare:'Compare survivors', Draw:'Draw progress', Clear:'Clear progress'})[action] || action;
