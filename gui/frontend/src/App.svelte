<script lang="ts">
  // Home Sync's main window: what's in sync, what isn't, and buttons to fix it. Every
  // action goes through ActionDialog, which shows what will happen before it happens.
  import { onMount } from "svelte";
  import { ModeWatcher } from "mode-watcher";
  import { Toaster } from "$lib/components/ui/sonner";
  import { toast } from "svelte-sonner";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import * as Card from "$lib/components/ui/card";
  import * as Alert from "$lib/components/ui/alert";
  import { Switch } from "$lib/components/ui/switch";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import * as Tooltip from "$lib/components/ui/tooltip";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import Folder from "@lucide/svelte/icons/folder";
  import File from "@lucide/svelte/icons/file";
  import RefreshCw from "@lucide/svelte/icons/refresh-cw";
  import Cloud from "@lucide/svelte/icons/cloud";
  import CircleAlert from "@lucide/svelte/icons/circle-alert";
  import FolderPlus from "@lucide/svelte/icons/folder-plus";
  import FilePlus from "@lucide/svelte/icons/file-plus";
  import Ellipsis from "@lucide/svelte/icons/ellipsis";
  import Wrench from "@lucide/svelte/icons/wrench";
  import { SyncService } from "../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui";
  import {
    Action,
    Status,
    type Overview,
    type Plan,
  } from "../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui/models";
  import { statusInfo, toneBadge, toneText } from "$lib/status";
  import { actionInfo, primaryAction } from "$lib/actions";
  import ActionDialog from "$lib/ActionDialog.svelte";
  import SetupWizard from "$lib/SetupWizard.svelte";

  // $state makes a variable reactive: the page updates whenever it changes.
  let overview = $state<Overview | null>(null);
  let error = $state("");
  let loading = $state(true);
  let showAdvanced = $state(false);
  let selected = $state<string[]>([]);

  // The action dialog
  let dialogOpen = $state(false);
  let plan = $state<Plan | null>(null);
  let note = $state("");

  // $derived recomputes from the state it reads.
  let entries = $derived((overview?.entries ?? []).filter((e) => showAdvanced || !e.hidden));
  let hiddenCount = $derived((overview?.entries ?? []).filter((e) => e.hidden).length);
  let needsAttention = $derived(entries.filter((e) => e.status !== Status.StatusSynced).length);
  let allSelected = $derived(entries.length > 0 && entries.every((e) => selected.includes(e.item)));

  const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

  // SyncService.* are the Go methods; Wails runs them and returns the result.
  async function load() {
    loading = true;
    error = "";
    try {
      overview = await SyncService.Overview();
      // Forget selections of items that are gone or now hidden
      const shown = new Set(entries.map((e) => e.item));
      selected = selected.filter((i) => shown.has(i));
    } catch (e) {
      error = message(e);
    } finally {
      loading = false;
    }
  }

  // Opens the dialog with the preview of an action; nothing happens until confirmed.
  async function start(action: Action, items: string[], extraNote = "") {
    try {
      const p = await SyncService.Preview(action, items);
      if (p.changes === 0) {
        toast.info(`${actionInfo[action].label}: nothing to do.`, {
          description: items.length === 1 ? items[0] : `${items.length} items`,
        });
        return;
      }
      plan = p;
      note = extraNote;
      dialogOpen = true;
    } catch (e) {
      toast.error("Couldn't check that", { description: message(e) });
    }
  }

  function toggle(item: string, on: boolean) {
    selected = on ? [...selected, item] : selected.filter((i) => i !== item);
  }

  function toggleAll(on: boolean) {
    selected = on ? entries.map((e) => e.item) : [];
  }

  // After an action: reload, and clear the selection.
  async function done() {
    selected = [];
    await load();
  }

  // "Add folder…" / "Add file…": the system's dialog, then the item is added to the list
  // and, if it needs something (move to the cloud, connect), offered right away.
  async function add(folder: boolean) {
    try {
      const path = folder
        ? await SyncService.ChooseFolder(showAdvanced)
        : await SyncService.ChooseFile(true);
      if (!path) return; // cancelled
      const added = await SyncService.AddEntry(path);
      if (added.entry.hidden) showAdvanced = true;
      await load();
      const status = added.entry.status;
      if (status === Status.StatusLocalOnly || status === Status.StatusReady || status === Status.StatusConflict) {
        await start(Action.ActionFix, [added.item], `Added ${added.item} to the list.`);
      } else {
        toast.success(`Added ${added.item} to the list.`, { description: statusInfo[status].text });
      }
    } catch (e) {
      toast.error("Couldn't add that", { description: message(e) });
    }
  }

  onMount(load);
</script>

<ModeWatcher />
<Toaster richColors position="bottom-center" />
<ActionDialog bind:open={dialogOpen} {plan} {note} ondone={done} />

<Tooltip.Provider>
  <main class="mx-auto flex max-w-3xl flex-col gap-6 p-6 pb-24">
    <header class="flex items-start justify-between gap-4">
      <div class="flex items-center gap-3">
        <img src="/icon.svg" alt="" class="size-10" />
        <div>
          <h1 class="text-2xl font-semibold tracking-tight">Home Sync</h1>
          {#if overview?.cloudDir}
            <p class="text-muted-foreground flex items-center gap-1.5 text-sm">
              <Cloud class="size-4" />
              {overview.cloudDir}
            </p>
          {/if}
        </div>
      </div>
      <div class="flex gap-2">
        {#if overview && !overview.needsSetup}
          <Button variant="outline" onclick={() => add(true)}>
            <FolderPlus />
            Add folder…
          </Button>
          {#if showAdvanced}
            <Button variant="outline" onclick={() => add(false)}>
              <FilePlus />
              Add file…
            </Button>
          {/if}
        {/if}
        <Button variant="outline" onclick={load} disabled={loading}>
          <RefreshCw class={loading ? "animate-spin" : ""} />
          Check again
        </Button>
      </div>
    </header>

    {#if error}
      <Alert.Root variant="destructive">
        <CircleAlert />
        <Alert.Title>Home Sync can't read its settings</Alert.Title>
        <Alert.Description>{error}</Alert.Description>
      </Alert.Root>
    {:else if overview?.needsSetup}
      <SetupWizard ondone={load} />
    {:else if loading && !overview}
      <div class="flex flex-col gap-3">
        {#each { length: 4 } as _}
          <Skeleton class="h-16 w-full rounded-xl" />
        {/each}
      </div>
    {:else if overview}
      <section class="flex items-center justify-between gap-4">
        <label class="text-muted-foreground flex items-center gap-3 text-sm">
          <Checkbox checked={allSelected} onCheckedChange={(v) => toggleAll(v === true)} aria-label="Select all" />
          {#if needsAttention === 0}
            Everything is in sync.
          {:else}
            {needsAttention} of {entries.length} need your attention.
          {/if}
        </label>
        {#if hiddenCount > 0}
          <label class="flex items-center gap-2 text-sm">
            <Switch bind:checked={showAdvanced} />
            Show hidden settings ({hiddenCount})
          </label>
        {/if}
      </section>

      <ul class="flex flex-col gap-3">
        {#each entries as entry (entry.item)}
          {@const info = statusInfo[entry.status]}
          {@const primary = primaryAction[entry.status]}
          <li>
            <Card.Root class="py-4 {selected.includes(entry.item) ? 'ring-primary/60 ring-2' : ''}">
              <Card.Content class="flex items-center gap-4">
                <Checkbox
                  checked={selected.includes(entry.item)}
                  onCheckedChange={(v) => toggle(entry.item, v === true)}
                  aria-label="Select {entry.item}"
                />
                <div class="bg-muted text-muted-foreground rounded-lg p-2">
                  {#if entry.isDir}
                    <Folder class="size-5" />
                  {:else}
                    <File class="size-5" />
                  {/if}
                </div>
                <div class="min-w-0 flex-1">
                  <p class="truncate font-medium">{entry.item}</p>
                  <p class="text-muted-foreground line-clamp-2 text-sm" title={entry.pointsTo ? `points to ${entry.pointsTo}` : undefined}>
                    {info.text}
                    {#if entry.pointsTo}(points to {entry.pointsTo}){/if}
                    {#if entry.detail}{entry.detail}{/if}
                  </p>
                </div>
                <Tooltip.Root>
                  <Tooltip.Trigger>
                    <Badge variant="outline" class={toneBadge[info.tone]}>
                      <info.icon class={toneText[info.tone]} />
                      {info.label}
                    </Badge>
                  </Tooltip.Trigger>
                  <Tooltip.Content>
                    <p>On this computer: {entry.home}</p>
                    <p>In your cloud: {entry.cloud}</p>
                  </Tooltip.Content>
                </Tooltip.Root>
                {#if primary}
                  <Button size="sm" onclick={() => start(primary, [entry.item])}>
                    {actionInfo[primary].label}
                  </Button>
                {/if}
                <DropdownMenu.Root>
                  <DropdownMenu.Trigger>
                    {#snippet child({ props })}
                      <Button {...props} variant="ghost" size="icon-sm" aria-label="More for {entry.item}">
                        <Ellipsis />
                      </Button>
                    {/snippet}
                  </DropdownMenu.Trigger>
                  <DropdownMenu.Content align="end" class="min-w-52">
                    {#if entry.status === Status.StatusSynced}
                      <DropdownMenu.Item onSelect={() => start(Action.ActionStop, [entry.item])}>
                        {actionInfo[Action.ActionStop].label}
                      </DropdownMenu.Item>
                    {/if}
                    {#if entry.status === Status.StatusBroken || entry.status === Status.StatusConflict}
                      <DropdownMenu.Item onSelect={() => start(Action.ActionUseCloud, [entry.item])}>
                        {actionInfo[Action.ActionUseCloud].label}
                      </DropdownMenu.Item>
                    {/if}
                    <DropdownMenu.Item variant="destructive" onSelect={() => start(Action.ActionRemove, [entry.item])}>
                      {actionInfo[Action.ActionRemove].label}
                    </DropdownMenu.Item>
                  </DropdownMenu.Content>
                </DropdownMenu.Root>
              </Card.Content>
            </Card.Root>
          </li>
        {:else}
          <li class="text-muted-foreground text-sm">Nothing to show.</li>
        {/each}
      </ul>
    {/if}
  </main>

  {#if selected.length > 0}
    <!-- The selection bar: actions for all selected items at once -->
    <div class="bg-card fixed inset-x-0 bottom-4 mx-auto flex w-fit items-center gap-3 rounded-xl border p-2 pl-4 shadow-lg">
      <span class="text-sm">{selected.length} selected</span>
      <Button onclick={() => start(Action.ActionFix, selected)}>
        <Wrench />
        Fix
      </Button>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger>
          {#snippet child({ props })}
            <Button {...props} variant="outline">More…</Button>
          {/snippet}
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="end" class="min-w-52">
          {#each [Action.ActionConnect, Action.ActionMove, Action.ActionUseCloud, Action.ActionStop] as action}
            <DropdownMenu.Item onSelect={() => start(action, selected)}>{actionInfo[action].label}</DropdownMenu.Item>
          {/each}
          <DropdownMenu.Separator />
          <DropdownMenu.Item variant="destructive" onSelect={() => start(Action.ActionRemove, selected)}>
            {actionInfo[Action.ActionRemove].label}
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Root>
      <Button variant="ghost" onclick={() => (selected = [])}>Clear</Button>
    </div>
  {/if}
</Tooltip.Provider>
