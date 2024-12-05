<section>
    <div>
        <span class="property">{normalise(property)}</span>
        <span>
            {#if operator === "eq"}
                =
            {:else if operator === "after"}
                &gt;
            {:else if operator === "before"}
                &lt;
            {/if}
        </span>
        <span class="value">
            {#if property === "timestamp"}
                {new Date(value).toLocaleDateString()} {new Date(value).toLocaleTimeString()}
            {:else}
                {value}
            {/if}
        </span>
    </div>

    <span class="remove-button" on:click={remove}>×</span>
</section>

<script>
    import {createEventDispatcher} from "svelte";

    export let property, operator, value;

    const dispatch = createEventDispatcher();

    function normalise(s) {
        s = s.replaceAll('_', ' ');
        return s;
    }

    function remove() {
        dispatch("remove");
    }
</script>

<style>
    section {
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
        gap: 0.2rem;

        padding: 0 0.2rem;

        background-color: var(--primary);
        color: var(--background);
        box-shadow: 0 4px 4px rgb(0 0 0 / 25%);

        user-select: none;

        min-width: 0;
    }

    section > div {
        min-width: 0;
        overflow: hidden;
    }

    span.property {
        text-transform: capitalize;
        white-space: nowrap;
    }

    span.value {
        overflow: hidden;
        text-overflow: ellipsis;
        word-break: break-all;
    }

    span.remove-button {
        font-size: 1rem;
        cursor: pointer;
    }
</style>
