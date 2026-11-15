export const title = 'Per-test coverage collection';
export const state = v => v.result === 'collecting' ? (v.pending.length ? 'Collecting profiles' : 'All profiles received') : v.result === 'complete' ? 'Complete profile returned' : 'Incomplete profile rejected';
export const step = action => ({ReceiveGood: 'a profile arrives', ReceiveBad: 'a profile is lost', Finish: 'the collection finishes', Done: 'finish'})[action] || action;
