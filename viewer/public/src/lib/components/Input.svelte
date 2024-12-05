<div>
    {#if label}
        <label>{label}</label>
    {/if}
    <input on:input on:change bind:value {placeholder} {disabled} readonly={readOnly}>
</div>

<script>
    export let label;
    export let placeholder;
    export let readOnly = false;
    export let disabled = false;

    export let value;
</script>

<style>
    div {
        display: flex;
        flex-direction: column;
    }

    input {
        border-color: #455d7a;

        background-color: var(--background);
        border: var(--text) solid 1px;
        color: var(--text-gray);
        font-size: 0.9rem;

        border-radius: 1px;

        padding: 1px 3px;
    }

    input:focus {
        outline: none;
    }
</style>