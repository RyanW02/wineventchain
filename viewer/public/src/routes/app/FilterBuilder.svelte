<form on:submit|preventDefault={add}>
    <div>
        <select bind:value={property}>
            <option value="none" selected disabled>Select a field...</option>
            <option value="event_id">Event ID</option>
            <option value="tx_hash">Transaction Hash</option>
            <option value="principal">Principal</option>
            <option value="event_type_id">Event Type ID</option>
            <option value="timestamp">Timestamp</option>
            <option value="provider_name">Provider Name</option>
            <option value="provider_guid">Provider GUID</option>
            <option value="correlation">Correlation</option>
            <option value="channel">Channel</option>
        </select>

        {#if property === "timestamp"}
            <select bind:value={timestampOperator}>
                <option value="after" selected>After</option>
                <option value="before">Before</option>
            </select>

            <input bind:value={timestampValue} type="datetime-local" max={new Date().toISOString().split(".")[0]}
                   on:click={(e) => e.target.showPicker()}/>
        {:else}
            <span>equals</span>
            <Input bind:value={value}/>
        {/if}
    </div>
    <div>
        <Button bind:this={button} shadow={false} type="submit"
                disabled={property === "none" || (property !== "timestamp" && !value) || (property === "timestamp" && !timestampValue)}>
            Add
        </Button>
    </div>
</form>

<script>
    import Input from "$lib/components/Input.svelte";
    import Button from "../../lib/components/Button.svelte";
    import {createEventDispatcher} from "svelte";

    const dispatch = createEventDispatcher();

    let button;

    let property = "none";
    let timestampOperator, timestampValue;
    let value;

    function add() {
        if (button.disabled) return;

        let filter;
        if (property === "timestamp") {
            filter = {
                property: "timestamp",
                operator: timestampOperator,
                value: timestampValue
            };
        } else {
            filter = {
                property,
                operator: "eq",
                value: value.trim()
            };
        }

        dispatch("add", filter);

        property = "none";
        timestampOperator = undefined;
        timestampValue = undefined;
        value = undefined;
    }
</script>

<style>
    form {
        display: grid;
        flex-direction: column;
        gap: 3px;

        padding: 3px 6px;

        background-color: var(--background);
        border-radius: 2px;
        box-shadow: 0 4px 4px rgb(0 0 0 / 25%);
    }

    form > div {
        display: flex;
        flex-direction: row;
        gap: 10px;
        align-items: center;
    }

    select {
        font: inherit;
        font-size: 14px;
        outline: none;
    }

    input[type="datetime-local"] {
        border: var(--text-gray) solid 1px;
        border-radius: 2px;
        font: inherit;
        font-size: 14px;
        outline: none;
        cursor: pointer;
    }
</style>