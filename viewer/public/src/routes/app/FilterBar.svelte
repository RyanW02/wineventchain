<div class="filters">
    <div>
        <Button on:click={toggleBuilder} padding={false}>
            <div class="filter-button">
                <i class="fas fa-filter"></i>
                Add Filter
            </div>
        </Button>
    </div>
    <div bind:this={builder} class="builder" use:clickOutside on:clickOutside={closeBuilder}>
        <FilterBuilder on:add={addFilter}/>
    </div>

    <div class="filter-container">
        {#each filters as filter}
            <FilterBadge property={filter.property} operator={filter.operator} value={filter.value}
                         on:remove={() => removeFilter(filter)}/>
        {/each}
    </div>

    {#if refreshButton}
        <span class="refresh" on:click={doRefresh}>
            <i bind:this={refreshIcon} class="fa-solid fa-arrows-rotate spin-on-click" class:clicked={doSpin}></i>
        </span>
    {/if}
</div>

<script>
    import Button from "$lib/components/Button.svelte";
    import FilterBuilder from "./FilterBuilder.svelte";
    import FilterBadge from "./FilterBadge.svelte";
    import {clickOutside} from "$lib/directives/clickoutside.js";
    import {createEventDispatcher} from "svelte";

    export let refreshButton = true;
    export let filters;

    const dispatch = createEventDispatcher();

    let doSpin = false;
    let refreshIcon;
    let builder;

    function toggleBuilder() {
        if (builder.style.visibility === "visible") {
            closeBuilder();
        } else {
            builder.style.visibility = "visible";
        }
    }

    function closeBuilder() {
        builder.style.visibility = "hidden";
    }

    function addFilter(e) {
        closeBuilder();

        const filter = e.detail;

        const conflicting = filters.find(f => f.property === filter.property && f.operator === filter.operator);
        if (conflicting) {
            filters = filters.filter(f => f !== conflicting);
        }

        filters.push(e.detail);
        filters = filters; // Force svelte update

        dispatch('add');
    }

    function removeFilter(filter) {
        filters = filters.filter(f => f !== filter);
        dispatch('remove');
    }

    async function doRefresh() {
        if (doSpin) {
            return;
        }

        doSpin = true;
        setTimeout(() => {
            doSpin = false;
        }, 1000);

        dispatch('refresh');
    }
</script>

<style>
    section {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .filters {
        display: flex;
        flex-direction: row;
        gap: 0.5rem;

        border: 1px solid var(--text-gray);
        border-radius: 3px;

        padding: 5px 0.5rem;
    }

    .filter-container {
        display: flex;
        flex-direction: row;
        gap: 0.25rem;
        flex-wrap: wrap;
        flex: 1;
        min-width: 0;
    }

    .filter-button {
        display: flex;
        flex-direction: row;
        gap: 0.5rem;
        align-items: center;
        white-space: nowrap;
    }

    span.refresh {
        color: var(--primary);
        cursor: pointer;
    }

    .builder {
        visibility: hidden;
        position: absolute;
        margin-top: 28px;
    }

    /* Refresh symbol spin animation */
    @keyframes spin {
        to {
            transform: rotate(360deg);
            -webkit-transform: rotate(360deg);
        }
    }

    .refresh > i {
        display: inline-block;
        cursor: pointer;
        animation-iteration-count: infinite;
    }

    .refresh > i.clicked {
        animation: spin 1s ease-in-out;
    }
</style>