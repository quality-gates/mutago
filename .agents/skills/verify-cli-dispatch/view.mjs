export const title = 'Interrupt cancellation';
export const state = v => v.terminated ? 'Default signal terminates' : v.cancelled ? 'Run cancelled' : v.stopped ? 'Signal interception stopped' : v.first ? 'First signal received' : 'Signal interception active';
export const step = action => ({First:'Receive first signal', Stop:'Stop interception', Cancel:'Cancel run', Second:'Receive second signal', Close:'Close', Done:'End'})[action] || action;
