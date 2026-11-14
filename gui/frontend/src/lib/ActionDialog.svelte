<script lang="ts">
  // The dialog every action goes through: it shows the preview (what will happen to each
  // item), runs the action only after confirming, and then shows what happened.
  import * as Dialog from "$lib/components/ui/dialog";
  import { Button } from "$lib/components/ui/button";
  import ArrowRight from "@lucide/svelte/icons/arrow-right";
  import Minus from "@lucide/svelte/icons/minus";
  import Check from "@lucide/svelte/icons/check";
  import X from "@lucide/svelte/icons/x";
  import LoaderCircle from "@lucide/svelte/icons/loader-circle";
  import { SyncService } from "../../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui";
  import type { Plan, Result } from "../../bindings/github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui/models";
  import { actionInfo, describeStep } from "$lib/actions";

  // $props are the component's inputs; $bindable lets the parent bind:open to it.
  let {
    open = $bindable(false),
    plan,
    note = "",
    ondone,
  }: {
    open?: boolean;
    plan: Plan | null;
    note?: string; // an extra first line, e.g. "Added Projects to the list."
    ondone: () => void; // called after the action ran (to reload the list)
  } = $props();

  let running = $state(false);
  let result = $state<Result | null>(null);
  let error = $state("");

  let info = $derived(plan ? actionInfo[plan.action] : null);

  // A new plan starts a fresh dialog
  $effect(() => {
    if (plan) {
      result = null;
      error = "";
      running = false;
    }
  });

  async function confirm() {
    if (!plan) return;
    running = true;
    error = "";
    try {
      const items = (plan.steps ?? []).filter((s) => s.change).map((s) => s.item);
      result = await SyncService.Apply(plan.action, items);
      ondone();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      running = false;
    }
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="sm:max-w-xl" showCloseButton={!running}>
    {#if plan && info}
      <Dialog.Header>
        <Dialog.Title>
          {#if result}
            {result.failed === 0 && !result.error ? "Done" : "Done, with problems"}
          {:else}
            {info.title(plan.changes)}
          {/if}
        </Dialog.Title>
        <Dialog.Description>
          {#if note}<span class="text-foreground block">{note}</span>{/if}
          {#if !result}
            {info.intro}
            {#if (plan.steps ?? []).length > plan.changes}
              {(plan.steps ?? []).length - plan.changes} of the chosen items need nothing and stay as they are.
            {/if}
          {/if}
        </Dialog.Description>
      </Dialog.Header>

      <ul class="flex max-h-80 flex-col gap-2 overflow-y-auto py-2">
        {#each (result ? result.steps : plan.steps) ?? [] as step (step.item)}
          <li class="flex items-start gap-3 text-sm {step.change || result ? '' : 'opacity-60'}">
            <span class="mt-0.5">
              {#if result}
                {#if step.change}
                  <Check class="text-success size-4" />
                {:else}
                  <X class="text-destructive size-4" />
                {/if}
              {:else if step.change}
                <ArrowRight class="text-primary size-4" />
              {:else}
                <Minus class="text-muted-foreground size-4" />
              {/if}
            </span>
            <span class="min-w-0">
              <span class="font-medium">{step.item}</span>
              <span class="text-muted-foreground block">{describeStep(step, !!result)}</span>
            </span>
          </li>
        {/each}
      </ul>

      {#if error || result?.error}
        <p class="text-destructive text-sm">{error || result?.error}</p>
      {/if}

      <Dialog.Footer>
        {#if result}
          <Button onclick={() => (open = false)}>Close</Button>
        {:else}
          <Button variant="outline" onclick={() => (open = false)} disabled={running}>Cancel</Button>
          <Button
            variant={info.destructive ? "destructive" : "default"}
            onclick={confirm}
            disabled={running || plan.changes === 0}
          >
            {#if running}<LoaderCircle class="animate-spin" />{/if}
            {info.confirm}
          </Button>
        {/if}
      </Dialog.Footer>
    {/if}
  </Dialog.Content>
</Dialog.Root>
