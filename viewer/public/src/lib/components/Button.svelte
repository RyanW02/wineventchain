<svelte:options accessors />

<button {type} on:click {disabled} class:shadow class:padding class:square>
    <slot />
</button>

<style>
    button {
        display: flex;
        justify-content: center;
        align-items: center;

        text-align: center;

        background-color: var(--primary);
        border: var(--primary) solid;
        color: var(--background);

        font-size: var(--base-font-size);

        border-width: 2px;
        margin: 0;

        cursor: pointer;
    }

    button:focus, button:active, button:focus-visible {
        outline: none;
    }

    button:disabled {
        background-color: #5b5b5b;
        border: #5b5b5b solid;
        color: var(--background);
        cursor: not-allowed;
    }

    .shadow {
        box-shadow: 0 4px 4px rgb(0 0 0 / 25%);
    }

    .padding {
        padding: 1px 1.5em;
    }

    .square {
        padding: 3px;
    }
</style>

<script>
    export let disabled = false;
    export let type;

    export let shadow = true;
    export let padding = true;
    export let square = false;
</script>
