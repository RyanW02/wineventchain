<section>
    <i class="fa-solid fa-chevron-left" class:disabled={page <= 1} on:click={previous}></i>
    <span>Page {page}</span>
    <i class="fa-solid fa-chevron-right" class:disabled={!hasMore} on:click={next}></i>
</section>

<script>
    import {createEventDispatcher} from "svelte";

    const dispatch = createEventDispatcher();

    export let page = 1;
    export let hasMore = true;

    function previous() {
        if (page > 1) {
            dispatch("paginate", {page: page - 1});
        }
    }

    function next() {
        if (hasMore) {
            dispatch("paginate", {page: page + 1});
        }
    }
</script>

<style>
    section {
        display: flex;
        flex-direction: row;
        align-items: center;
        gap: 0.2rem;

        user-select: none;
    }

    section > i {
        color: var(--primary);
        cursor: pointer;
    }

    section > i.disabled {
        color: var(--text-gray);
        cursor: unset;
    }
</style>