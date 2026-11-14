<script lang="ts">
  // Home Sync's main window. Phase 1 only shows: what's in sync, what isn't and why.
  // Actions (move to the cloud, repair, …) come in the next step.
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import * as Card from "$lib/components/ui/card";
  import * as Alert from "$lib/components/ui/alert";
  import { Switch } from "$lib/components/ui/switch";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import * as Tooltip from "$lib/components/ui/tooltip";
  import Folder from "@lucide/svelte/icons/folder";
  import File from "@lucide/svelte/icons/file";
  import RefreshCw from "@lucide/svelte/icons/refresh-cw";
  import Cloud from "@lucide/svelte/icons/cloud";
  import CircleAlert from "@lucide/svelte/icons/circle-alert";
  import { SyncService } from "../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui";
  import { Status, type Overview } from "../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui/models";
  import { statusInfo, toneBadge, toneText } from "$lib/status";

  // $state makes a variable reactive: the page updates whenever it changes.
  let overview = $state<Overview | null>(null);
  let error = $state("");
  let loading = $state(true);
  let showAdvanced = $state(false);

  // $derived recomputes from the state it reads.
  let entries = $derived((overview?.entries ?? []).filter((e) => showAdvanced || !e.hidden));
  let hiddenCount = $derived((overview?.entries ?? []).filter((e) => e.hidden).length);
  let needsAttention = $derived(entries.filter((e) => e.status !== Status.StatusSynced).length);

  // SyncService.Overview is the Go method; Wails runs it and returns the result.
  async function load() {
    loading = true;
    error = "";
    try {
      overview = await SyncService.Overview();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

  onMount(load);
</script>

<Tooltip.Provider>
  <main class="mx-auto flex max-w-3xl flex-col gap-6 p-6">
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
      <Button variant="outline" onclick={load} disabled={loading}>
        <RefreshCw class={loading ? "animate-spin" : ""} />
        Check again
      </Button>
    </header>

    {#if error}
      <Alert.Root variant="destructive">
        <CircleAlert />
        <Alert.Title>Home Sync can't read its settings</Alert.Title>
        <Alert.Description>{error}</Alert.Description>
      </Alert.Root>
    {:else if overview?.needsSetup}
      <Card.Root>
        <Card.Header>
          <Card.Title>Not set up yet</Card.Title>
          <Card.Description>
            There's no configuration at {overview.configPath}. Setting up from here comes in
            the next version; until then, create it with
            <code>linux-home-cloud-storage-sync write-default-config</code>.
          </Card.Description>
        </Card.Header>
      </Card.Root>
    {:else if loading && !overview}
      <div class="flex flex-col gap-3">
        {#each { length: 4 } as _}
          <Skeleton class="h-16 w-full rounded-xl" />
        {/each}
      </div>
    {:else if overview}
      <section class="flex items-center justify-between gap-4">
        <p class="text-muted-foreground text-sm">
          {#if needsAttention === 0}
            Everything is in sync.
          {:else}
            {needsAttention} of {entries.length} need your attention.
          {/if}
        </p>
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
          <li>
            <Card.Root class="py-4">
              <Card.Content class="flex items-center gap-4">
                <div class="bg-muted text-muted-foreground rounded-lg p-2">
                  {#if entry.isDir}
                    <Folder class="size-5" />
                  {:else}
                    <File class="size-5" />
                  {/if}
                </div>
                <div class="min-w-0 flex-1">
                  <p class="truncate font-medium">{entry.item}</p>
                  <p class="text-muted-foreground truncate text-sm">
                    {info.text}
                    {#if entry.pointsTo}
                      (points to {entry.pointsTo})
                    {/if}
                    {#if entry.detail}
                      {entry.detail}
                    {/if}
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
              </Card.Content>
            </Card.Root>
          </li>
        {:else}
          <li class="text-muted-foreground text-sm">Nothing to show.</li>
        {/each}
      </ul>
    {/if}
  </main>
</Tooltip.Provider>
