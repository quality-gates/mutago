export const title = "Custom exec must restore the source";

export function state(v) {
  if (v.engine === "returned") {
    return v.source === "original"
      ? "Exec returned with the original source"
      : "Exec returned with the mutant in the source";
  }
  if (v.shell === "killed") return v.source === "original" ? "Script killed after restoring" : "Script killed; mutant remains";
  if (v.shell === "exited") return v.source === "original" ? "Script exited after restoring" : "Script exited without restoring";
  if (v.shell === "restored") return v.source === "original" ? "Script restored the original" : "Script continued without restoring";
  const phase = v.shell === "readyToRestore" ? "Test finished; cleanup pending" : "Mutant installed; test running";
  return v.contextState === "cancelled" ? `${phase}; cancellation pending` : phase;
}

export function step(action, from, to) {
  return {
    CancelContext: "the context is cancelled or times out",
    KillScript: "Go kills the custom script",
    FinishTest: "the test command finishes",
    RestoreSource: to.source === "original" ? "the script restores the original" : "the script continues without restoring",
    ExitScript: "the script exits",
    ReturnFromExec: "the engine returns from the custom exec",
    Done: "execution is complete",
  }[action] ?? action;
}
