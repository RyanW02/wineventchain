<section>
    <FilterBar bind:filters on:refresh={doRefresh} on:add={doSearch} on:remove={doSearch} />

    <div class="events">
        {#each searchResults as event}
            <div>
                <EventPreview {event}/>
            </div>
        {/each}
        <nav class="paginator">
            <Paginator page={pageNumber} hasMore={searchResults.length >= pageLimit} on:paginate={loadPage}/>
        </nav>
    </div>
</section>

<script>
    import {page} from "$app/stores";
    import EventPreview from "./EventPreview.svelte";
    import {onMount} from "svelte";
    import http from "$lib/http.js";
    import {addToast} from "$lib/stores.js";
    import Paginator from "./Paginator.svelte";
    import FilterBar from "./FilterBar.svelte";
    import {isLoggedIn} from "$lib/auth.js";

    const pageLimit = 15;

    let filters = [];
    let searchResults = [];
    let isSearching = false;

    let firstEventId = null;
    let pageNumber = 1;

    async function doRefresh() {
        if (pageNumber === 1) {
            firstEventId = null;
        }

        await doSearch();
    }

    async function loadPage(e) {
        if (isSearching) {
            return;
        }

        pageNumber = e.detail.page;
        await doSearch();

        window.scrollTo({top: 0, behavior: 'smooth'});
    }

    async function doSearch() {
        isSearching = true;

        try {
            let path = `/events?page=${pageNumber}`;
            if (firstEventId) {
                path += `&first=${firstEventId}`;
            }

            const res = await http.post(path, {filters});
            if (res.status !== 200) {
                addToast(false, res.data.error || "Failed to search events");
                isSearching = false;
                return;
            }

            searchResults = res.data;
            searchResults = [...searchResults];

            if (pageNumber === 1 && !firstEventId) {
                firstEventId = searchResults[0]?.metadata?.event_id;
            }
        } catch (e) {
            console.error(e);
            addToast(false, "Failed to search events");
            isSearching = false;
            return;
        }

        isSearching = false;
    }

    onMount(async () => {
        if (!isLoggedIn()) {
            return;
        }

        const params = $page.url.searchParams;

        const supportedParams = ["principal", "event_type_id", "provider_name", "provider_guid", "correlation", "channel"];
        for (const param of supportedParams) {
            if (params.has(param)) {
                filters.push({
                    property: param,
                    operator: "eq",
                    value: params.get(param)
                });
            }
        }

        filters = filters;

        await doSearch();
    });
</script>

<style>
    section {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .events {
        display: flex;
        flex-direction: column;
    }

    .events > *:not(div:last-of-type):not(.paginator) {
        border-bottom: 1px solid var(--text-gray);
    }

    .events > .paginator {
        align-self: center;
    }
</style>