<section>
    <div class="data">
        <table>
            <tr>
                <td>Event ID</td>
                <td>{shortenEventId(event.metadata.event_id)}</td>
            </tr>
            <tr>
                <td>Principal</td>
                <td>{event.metadata.principal}</td>
            </tr>
            <tr>
                <td>Provider</td>
                <td>{event.event.system.provider.name}</td>
            </tr>
            <tr>
                <td>Channel</td>
                <td>{event.event.system.channel}</td>
            </tr>
            <tr>
                <td>Event Type</td>
                {#if EventMessages[event.event.system.event_id]}
                    <td>{EventMessages[event.event.system.event_id]} ({event.event.system.event_id})</td>
                {:else}
                    <td>{event.event.system.event_id}</td>
                {/if}
            </tr>
            <tr>
                <td>Timestamp</td>
                <td>
                    {new Date(event.metadata.received_time).toLocaleDateString()}
                    {new Date(event.metadata.received_time).toLocaleTimeString()}
                </td>
            </tr>
        </table>
    </div>
    <div>
        <a href="/app/event/{event.metadata.event_id}">
            <Button>View</Button>
        </a>
    </div>
</section>

<script>
    import Button from "$lib/components/Button.svelte";
    import EventMessages from "$lib/event_messages.js";

    export let event;

    function shortenEventId(eventId) {
        if (eventId.length <= 8) {
            return eventId;
        }

        return eventId.substring(0, 4) + "..." + eventId.substring(eventId.length - 4, eventId.length);
    }
</script>

<style>
    section {
        display: flex;
        flex-direction: row;
        justify-content: space-between;
    }

    section > div:first-child {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    section > div:last-child {
        margin-left: 1rem;
    }

    section > div {
        display: flex;
        flex-direction: column;
        justify-content: center;
    }

    td:first-child::after {
        content: ":";
    }

    td:first-of-type {
        font-weight: bold;
        white-space: nowrap;
        vertical-align: top;
    }

    td:nth-of-type(2) {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: pre-wrap;
    }
</style>