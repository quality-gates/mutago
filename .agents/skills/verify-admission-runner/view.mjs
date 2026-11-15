export const title = 'Candidate admission';
export const state = v => `${v.last}; ${v.seen.length} edits admitted`;
export const step = action => ({Reject:'Reject an out-of-scope candidate', Accept:'Admit an unseen edit', Duplicate:'Reject a duplicate'})[action] || action;
