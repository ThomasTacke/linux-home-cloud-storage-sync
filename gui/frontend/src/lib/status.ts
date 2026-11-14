// How Home Sync describes each state, in plain words. The states come from the Go side
// (SyncService.Overview); the words are only here.
import type { Component } from "svelte";
import CircleCheck from "@lucide/svelte/icons/circle-check";
import CloudDownload from "@lucide/svelte/icons/cloud-download";
import CloudUpload from "@lucide/svelte/icons/cloud-upload";
import Copy from "@lucide/svelte/icons/copy";
import Unlink from "@lucide/svelte/icons/unlink";
import CircleDashed from "@lucide/svelte/icons/circle-dashed";
import CircleAlert from "@lucide/svelte/icons/circle-alert";
import { Status } from "../../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui/models";

export interface StatusInfo {
  label: string; // short, for the badge
  text: string; // one sentence under the name
  icon: Component;
  tone: "success" | "warning" | "danger" | "muted";
}

export const statusInfo: Record<Status, StatusInfo> = {
  [Status.$zero]: { label: "Unknown", text: "", icon: CircleDashed, tone: "muted" },
  [Status.StatusSynced]: {
    label: "In sync",
    text: "Stored in your cloud and used from there.",
    icon: CircleCheck,
    tone: "success",
  },
  [Status.StatusReady]: {
    label: "Ready to connect",
    text: "Your cloud has it, this computer doesn't use it yet.",
    icon: CloudDownload,
    tone: "warning",
  },
  [Status.StatusLocalOnly]: {
    label: "Only on this computer",
    text: "Not in your cloud yet. Move it there to keep it in sync.",
    icon: CloudUpload,
    tone: "warning",
  },
  [Status.StatusConflict]: {
    label: "Two versions",
    text: "This computer and your cloud each have their own copy.",
    icon: Copy,
    tone: "warning",
  },
  [Status.StatusBroken]: {
    label: "Needs repair",
    text: "The connection points to the wrong place.",
    icon: Unlink,
    tone: "danger",
  },
  [Status.StatusMissing]: {
    label: "Not found",
    text: "Neither on this computer nor in your cloud.",
    icon: CircleDashed,
    tone: "muted",
  },
  [Status.StatusOther]: {
    label: "Unusual",
    text: "Home Sync leaves this alone.",
    icon: CircleAlert,
    tone: "danger",
  },
};

// Tailwind classes per tone: text color, and a tinted background for the badge.
export const toneText: Record<StatusInfo["tone"], string> = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-destructive",
  muted: "text-muted-foreground",
};

export const toneBadge: Record<StatusInfo["tone"], string> = {
  success: "bg-success/15 text-success border-success/30",
  warning: "bg-warning/15 text-warning border-warning/30",
  danger: "bg-destructive/15 text-destructive border-destructive/30",
  muted: "bg-muted text-muted-foreground border-border",
};
