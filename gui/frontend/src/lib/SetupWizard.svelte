<script lang="ts">
  // First run: where's the cloud folder, which folders go into it, what will happen, set
  // up. Nothing is written until "Set up".
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import * as Card from "$lib/components/ui/card";
  import * as RadioGroup from "$lib/components/ui/radio-group";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import Cloud from "@lucide/svelte/icons/cloud";
  import FolderOpen from "@lucide/svelte/icons/folder-open";
  import Monitor from "@lucide/svelte/icons/monitor";
  import FileText from "@lucide/svelte/icons/file-text";
  import Image from "@lucide/svelte/icons/image";
  import MusicIcon from "@lucide/svelte/icons/music";
  import Film from "@lucide/svelte/icons/film";
  import LayoutTemplate from "@lucide/svelte/icons/layout-template";
  import Folder from "@lucide/svelte/icons/folder";
  import ArrowRight from "@lucide/svelte/icons/arrow-right";
  import Minus from "@lucide/svelte/icons/minus";
  import Check from "@lucide/svelte/icons/check";
  import X from "@lucide/svelte/icons/x";
  import LoaderCircle from "@lucide/svelte/icons/loader-circle";
  import type { Component } from "svelte";
  import { SyncService } from "../../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui";
  import type {
    CloudOption,
    Plan,
    Result,
    SetupInfo,
  } from "../../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui/models";
  import { describeStep } from "$lib/actions";

  let { ondone }: { ondone: () => void } = $props();

  // The subfolder in the cloud folder (cloudSubfolder in setup.go)
  const cloudSubfolder = "LinuxHome";

  // English names and icons for the usual folders (the item is this system's name for it)
  const folderInfo: Record<string, { name: string; icon: Component }> = {
    desktop: { name: "Desktop", icon: Monitor },
    documents: { name: "Documents", icon: FileText },
    pictures: { name: "Pictures", icon: Image },
    music: { name: "Music", icon: MusicIcon },
    videos: { name: "Videos", icon: Film },
    templates: { name: "Templates", icon: LayoutTemplate },
  };

  let step = $state<1 | 2 | 3 | 4>(1);
  let info = $state<SetupInfo | null>(null);
  let clouds = $state<CloudOption[]>([]);
  let cloud = $state(""); // the chosen cloud folder's path
  let chosen = $state<string[]>([]);
  let plan = $state<Plan | null>(null);
  let result = $state<Result | null>(null);
  let busy = $state(false);
  let error = $state("");

  let cloudShow = $derived(clouds.find((c) => c.path === cloud)?.show ?? cloud);

  const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

  onMount(async () => {
    try {
      info = await SyncService.SetupInfo();
      clouds = info.clouds ?? [];
      if (clouds.length > 0) cloud = clouds[0].path;
      // Preselect the folders that exist here
      chosen = (info.folders ?? []).filter((f) => f.exists).map((f) => f.item);
    } catch (e) {
      error = message(e);
    }
  });

  async function chooseCloud() {
    try {
      const path = await SyncService.ChooseCloudFolder();
      if (!path) return;
      if (!clouds.some((c) => c.path === path)) {
        clouds = [...clouds, { name: "Chosen folder", path, show: path }];
      }
      cloud = path;
    } catch (e) {
      error = message(e);
    }
  }

  function toggle(item: string, on: boolean) {
    chosen = on ? [...chosen, item] : chosen.filter((i) => i !== item);
  }

  async function review() {
    busy = true;
    error = "";
    try {
      plan = await SyncService.SetupPreview(cloud, chosen);
      step = 3;
    } catch (e) {
      error = message(e);
    } finally {
      busy = false;
    }
  }

  async function setUp() {
    busy = true;
    error = "";
    try {
      result = await SyncService.SetupApply(cloud, chosen);
      step = 4;
    } catch (e) {
      error = message(e);
    } finally {
      busy = false;
    }
  }
</script>

<Card.Root>
  <Card.Header>
    <p class="text-muted-foreground text-sm">Step {Math.min(step, 3)} of 3</p>
    <Card.Title class="text-xl">
      {#if step === 1}Where's your cloud folder?
      {:else if step === 2}Which folders should be in sync?
      {:else if step === 3}Ready to set up
      {:else}All set{/if}
    </Card.Title>
    <Card.Description>
      {#if step === 1}
        The folder your cloud app (Nextcloud, Dropbox, …) keeps in sync. Home Sync stores your
        folders inside it, in <strong>{cloudSubfolder}</strong>, so they're the same on all your computers.
      {:else if step === 2}
        They move into your cloud and stay exactly where you know them on this computer. You can
        add more folders later.
      {:else if step === 3}
        This is what will happen. Nothing is deleted: where this computer and your cloud both
        have a folder, this computer's copy is kept as a backup next to it.
      {:else if result && result.failed === 0}
        Your folders are in sync now. Do the same on your other computers to get them there too.
      {:else}
        Set up, but some folders didn't work; see below.
      {/if}
    </Card.Description>
  </Card.Header>

  <Card.Content>
    {#if !info && !error}
      <div class="flex flex-col gap-2">
        <Skeleton class="h-12 w-full" />
        <Skeleton class="h-12 w-full" />
      </div>
    {:else if step === 1}
      {#if clouds.length === 0}
        <p class="text-muted-foreground mb-3 text-sm">
          Home Sync couldn't find a cloud folder. Install and set up your cloud app first, or
          choose the folder yourself.
        </p>
      {/if}
      <RadioGroup.Root bind:value={cloud} class="gap-2">
        {#each clouds as c (c.path)}
          <label
            class="hover:bg-muted/50 flex cursor-pointer items-center gap-3 rounded-lg border p-3 {cloud === c.path
              ? 'border-primary'
              : ''}"
          >
            <RadioGroup.Item value={c.path} />
            <Cloud class="text-muted-foreground size-5" />
            <span>
              <span class="block font-medium">{c.name}</span>
              <span class="text-muted-foreground block text-sm">{c.show}</span>
            </span>
          </label>
        {/each}
      </RadioGroup.Root>
      <Button variant="outline" class="mt-3" onclick={chooseCloud}>
        <FolderOpen />
        Choose another folder…
      </Button>
    {:else if step === 2}
      <div class="grid gap-2 sm:grid-cols-2">
        {#each info?.folders ?? [] as f (f.item)}
          {@const fi = folderInfo[f.key] ?? { name: f.item, icon: Folder }}
          <label
            class="hover:bg-muted/50 flex cursor-pointer items-center gap-3 rounded-lg border p-3 {chosen.includes(f.item)
              ? 'border-primary'
              : ''}"
          >
            <Checkbox checked={chosen.includes(f.item)} onCheckedChange={(v) => toggle(f.item, v === true)} />
            <fi.icon class="text-muted-foreground size-5" />
            <span>
              <span class="block font-medium">{f.item}</span>
              {#if f.item !== fi.name}
                <span class="text-muted-foreground block text-sm">{fi.name}</span>
              {:else if !f.exists}
                <span class="text-muted-foreground block text-sm">not on this computer yet</span>
              {/if}
            </span>
          </label>
        {/each}
      </div>
    {:else if (step === 3 && plan) || (step === 4 && result)}
      <p class="text-muted-foreground mb-3 flex items-center gap-1.5 text-sm">
        <Cloud class="size-4" />
        {cloudShow}/{cloudSubfolder}
      </p>
      <ul class="flex flex-col gap-2">
        {#each (step === 4 ? result?.steps : plan?.steps) ?? [] as s (s.item)}
          <li class="flex items-start gap-3 text-sm {s.change || step === 4 ? '' : 'opacity-60'}">
            <span class="mt-0.5">
              {#if step === 4}
                {#if s.change}<Check class="text-success size-4" />{:else}<X class="text-destructive size-4" />{/if}
              {:else if s.change}
                <ArrowRight class="text-primary size-4" />
              {:else}
                <Minus class="text-muted-foreground size-4" />
              {/if}
            </span>
            <span>
              <span class="font-medium">{s.item}</span>
              <span class="text-muted-foreground block">{describeStep(s, step === 4)}</span>
            </span>
          </li>
        {/each}
      </ul>
    {/if}

    {#if error}
      <p class="text-destructive mt-3 text-sm">{error}</p>
    {/if}
  </Card.Content>

  <Card.Footer class="justify-between gap-2">
    {#if step > 1 && step < 4}
      <Button variant="ghost" onclick={() => (step = step === 3 ? 2 : 1)} disabled={busy}>Back</Button>
    {:else}
      <span></span>
    {/if}
    {#if step === 1}
      <Button onclick={() => (step = 2)} disabled={!cloud}>Next</Button>
    {:else if step === 2}
      <Button onclick={review} disabled={chosen.length === 0 || busy}>
        {#if busy}<LoaderCircle class="animate-spin" />{/if}
        Next
      </Button>
    {:else if step === 3}
      <Button onclick={setUp} disabled={busy}>
        {#if busy}<LoaderCircle class="animate-spin" />{/if}
        Set up
      </Button>
    {:else}
      <Button onclick={ondone}>Show my folders</Button>
    {/if}
  </Card.Footer>
</Card.Root>
