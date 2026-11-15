export const title = 'Run shutdown';
export const state = v => v.removed ? 'Temporary directory removed' : v.summaries ? 'Summary published' : v.progressDone ? 'Progress stopped' : v.workersDone ? 'Workers stopped' : v.closing ? 'Draining workers' : 'Running';
export const step = action => ({Start:'Start job', Skip:'Skip cancelled job', Complete:'Complete job', Close:'Close queue', WaitWorkers:'Workers stop', StopProgress:'Progress stops', Summarize:'Publish summary', Remove:'Remove temporary directory'})[action] || action;
