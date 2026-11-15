export const title = 'Result finalization';
export const state = v => v.stage === 'done' ? `Finished: ${v.outcome}` : v.stage;
export const step = action => ({Select:'Choose initial action', Finish:'Complete finalization', Done:'Remain finished'})[action] || action;
