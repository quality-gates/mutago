export const title = "The bundled executor keeps the source unchanged";

export function state(v) {
  const source = v.source === "original" ? "original source" : "mutant in source";
  if (v.engine === "returned") return `Exec returned; ${source}`;
  if (v.shell === "killed") return `Script killed; ${source}`;
  if (v.shell === "exited") return `Script exited; ${source}`;
  const phase = {
    preparing: "Script preparing",
    testing: "Mutant test running",
    testDone: "Mutant test finished",
  }[v.shell];
  const pending = v.contextState === "cancelled" ? "; cancellation pending" : "";
  return `${phase}; ${source}${pending}`;
}

export function step(action) {
  return {
    StartTest: "the script starts the test with the mutant overlay",
    CancelContext: "the context is cancelled or times out",
    KillScript: "Go kills the custom script",
    FinishTest: "the test command finishes",
    ExitScript: "the script exits",
    ReturnFromExec: "the engine returns from the custom exec",
    Done: "execution is complete",
  }[action] ?? action;
}
