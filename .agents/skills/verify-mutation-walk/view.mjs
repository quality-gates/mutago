export const title = 'Mutation walk restoration';
const names = {ready:'Ready for mutation', applied:'Mutation applied', unwinding:'Callback finished', resetNotice:'Reset notification', advance:'Restored', failed:'Callback failed', panicked:'Callback panicked', cancelled:'Cancelled', done:'Complete'};
export const state = v => names[v.phase];
export const step = (action, from, to) => ({Apply:'Apply mutation', Callback:'Callback returns or unwinds', Reset:'Restore syntax', AckReset:'Acknowledge reset', Advance:'Advance or stop', Done:'Remain finished'})[action] || action;
