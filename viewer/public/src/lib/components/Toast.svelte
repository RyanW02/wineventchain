<div class="wrapper" class:info={success} class:danger={!success} transition:slide={{delay: 250, duration: 300, axis: 'y'}}>
    <div class="content">
        <slot/>
    </div>
    <div>
        <div class="close-button" on:click={dispatchClose}>&#xD7</div>
    </div>
</div>

<script>
    import { slide } from 'svelte/transition';
    import {createEventDispatcher} from 'svelte';

    export let success = true;

    const dispatch = createEventDispatcher();

    function dispatchClose() {
        dispatch('close');
    }
</script>

<style>
    .wrapper {
        display: flex;
        flex-direction: row;
        justify-content: space-between;

        background-color: var(--background);
        box-shadow: 0 4px 4px rgb(0 0 0 / 25%);

        border-style: solid;
        border-width: 0 0 0 7px;
        border-radius: 5px;

        font-size: 14px;

        word-break: break-word;
    }

    .wrapper.info {
        border-color: var(--accent);
    }

    .wrapper.danger {
        border-color: var(--danger);
    }

    .content {
        padding: 3px 0 3px 5px;
    }

    .close-button {
        display: flex;
        color: var(--text-gray);
        font-size: 20px;
        cursor: pointer;

        margin: -5px 2px 0 2px;
    }
</style>