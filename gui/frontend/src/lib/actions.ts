// How Home Sync talks about actions: button labels, dialog texts, and one sentence per
// step, before ("will …") and after ("… done"). The Go side only returns codes.
import { Action, Outcome, Status, type Step } from "../../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui/models";

export interface ActionInfo {
  label: string; // button / menu item
  title: (n: number) => string; // dialog title
  intro: string; // first paragraph in the dialog
  confirm: string; // the confirm button
  destructive?: boolean; // shown in red
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

export const actionInfo: Record<Action, ActionInfo> = {
  [Action.$zero]: { label: "", title: () => "", intro: "", confirm: "" },
  [Action.ActionFix]: {
    label: "Fix",
    title: (n) => `Fix ${plural(n, "item", "items")}?`,
    intro: "Each item gets what it needs to be in sync again.",
    confirm: "Fix",
  },
  [Action.ActionConnect]: {
    label: "Connect",
    title: (n) => `Connect ${plural(n, "item", "items")}?`,
    intro: "This computer will use the copy in your cloud.",
    confirm: "Connect",
  },
  [Action.ActionMove]: {
    label: "Move to cloud",
    title: (n) => `Move ${plural(n, "item", "items")} to your cloud?`,
    intro: "They move into your cloud folder and stay usable here exactly as before.",
    confirm: "Move to cloud",
  },
  [Action.ActionUseCloud]: {
    label: "Use cloud version",
    title: (n) => `Use the cloud version for ${plural(n, "item", "items")}?`,
    intro: "This computer's own copy is kept next to it as a backup, nothing is deleted.",
    confirm: "Use cloud version",
  },
  [Action.ActionStop]: {
    label: "Stop syncing",
    title: (n) => `Stop syncing ${plural(n, "item", "items")}?`,
    intro: "This computer gets its own copy of the current content. The cloud copy stays for your other computers.",
    confirm: "Stop syncing",
    destructive: true,
  },
  [Action.ActionRemove]: {
    label: "Remove from list",
    title: (n) => `Remove ${plural(n, "item", "items")} from the list?`,
    intro: "Home Sync stops looking after them. Nothing on this computer or in your cloud changes.",
    confirm: "Remove",
    destructive: true,
  },
};

// The obvious action for an item in a given state, for its button.
export const primaryAction: Partial<Record<Status, Action>> = {
  [Status.StatusReady]: Action.ActionConnect,
  [Status.StatusLocalOnly]: Action.ActionMove,
  [Status.StatusConflict]: Action.ActionUseCloud,
  [Status.StatusBroken]: Action.ActionFix,
};

// One sentence per step. done: the result after running, otherwise the preview.
export function describeStep(step: Step, done: boolean): string {
  switch (step.outcome) {
    case Outcome.OutcomeConnect:
      return done ? "Connected to your cloud." : "Will use the copy in your cloud.";
    case Outcome.OutcomeMove:
      return done ? "Moved to your cloud." : "Will move to your cloud and stay usable here.";
    case Outcome.OutcomeRepair:
      return done
        ? "Repaired."
        : `Will point to your cloud instead of ${step.pointsTo} (which stays as it is).`;
    case Outcome.OutcomeBackup:
      return done
        ? `Now uses the cloud version. This computer's copy is kept as ${step.backup}.`
        : `Will use the cloud version. This computer's copy is kept as ${step.backup}.`;
    case Outcome.OutcomeDisconnect:
      return done ? "No longer synced." : "Will no longer be synced.";
    case Outcome.OutcomeCopyBack:
      return done ? "Has its own copy now, no longer synced." : "Gets its own copy, no longer synced.";
    case Outcome.OutcomeRemoveEntry:
      return done ? "Removed from the list." : "Will be removed from the list.";
    case Outcome.OutcomeSkipSynced:
      return "Already in sync, nothing to do.";
    case Outcome.OutcomeSkipNoCloud:
      return "Not in your cloud, nothing to do.";
    case Outcome.OutcomeSkipLocal:
      return "Has its own copy on this computer, this action doesn't apply.";
    case Outcome.OutcomeSkipLink:
      return `Points to ${step.pointsTo}, this action doesn't apply.`;
    case Outcome.OutcomeSkipNothing:
      return "Nothing there, nothing to do.";
    case Outcome.OutcomeFailed:
      return `Didn't work: ${step.error}`;
    default:
      return "Left alone.";
  }
}
