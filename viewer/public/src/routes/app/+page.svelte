<section>
    <nav>
        {#each tabs as tab}
            <a on:click={() => showTab(tab)} class:active={tab.active}>{tab.name}</a>
        {/each}
    </nav>
    <div class="content">
        <div bind:this={tabs[0].tab} class:active={tabs[0].active}>
            <Card>
                <EventSearch/>
            </Card>
        </div>
        <div bind:this={tabs[1].tab} class:active={tabs[1].active}>
            <Card>
                <EventStream/>
            </Card>
        </div>
    </div>
</section>

<script>
    import Card from "$lib/components/Card.svelte";
    import EventStream from "./EventStream.svelte";
    import EventSearch from "./EventSearch.svelte";
    import {onMount} from "svelte";

    let tabs = [
        {
            name: "Event Search",
        },
        {
            name: "Event Stream",
        }
    ];

    function showTab(tab) {
        const activeTab = tabs.find(t => t.active === true);
        if (activeTab) {
            activeTab.active = false;
        }

        tab.active = true;
        tabs = tabs; // Force reactivity update
    }

    onMount(() => showTab(tabs[0]));
</script>

<style>
    section {
        display: flex;
        flex-direction: column;
        gap: 1rem;
        padding: 0 1rem;
        margin-top: 1rem;
    }

    .content > div:not(.active) {
        display: none;
    }

    nav > a {
        border-bottom: 1px solid var(--text-gray);
        color: var(--text);
        padding: 5px 10px;

        cursor: pointer;
    }

    nav > a.active {
        border-bottom: 3px solid var(--primary);
        color: var(--primary);
        font-weight: bold;
    }
</style>
