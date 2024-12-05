{#if event}
    <section>
        <section id="metadata">
            <Card header="Metadata">
                <table>
                    <tbody>
                    <tr>
                        <td>Event ID</td>
                        <td>{event.metadata.event_id}</td>
                    </tr>
                    <tr>
                        <td>Transaction Hash</td>
                        <td>{event.tx_hash}</td>
                    </tr>
                    <tr>
                        <td>Received Time</td>
                        <td>{event.metadata.received_time.toLocaleDateString()} {event.metadata.received_time.toLocaleTimeString()}</td>
                    </tr>
                    <tr>
                        <td>User</td>
                        <td><a class="principal"
                               href="/app?principal={event.metadata.principal}">{event.metadata.principal}</a></td>
                    </tr>
                    </tbody>
                </table>
            </Card>
        </section>

        <section id="system">
            <Card header="Event">
                {@const ev = event.event.system}
                <table>
                    <tbody>
                    <tr>
                        <td>Provider</td>
                        <td>
                            {#if ev.provider.guid !== null}
                                <a href="/app?provider_guid={stripBraces(ev.provider.guid)}">
                                    {ev.provider.name}
                                </a>
                            {:else if ev.provider.name !== null}
                                <a href="/app?provider_name={ev.provider.name}">
                                    {ev.provider.name}
                                </a>
                            {:else}
                                <a>{ev.provider.name}</a>
                            {/if}

                            {#if ev.provider.event_source_name}
                                ({ev.provider.event_source_name})
                            {/if}
                        </td>
                    </tr>
                    <tr>
                        <td>Event Type</td>
                        {#if EventMessages[ev.event_id]}
                            <td>
                                {EventMessages[ev.event_id]}
                                (<a href="/app?event_type_id={ev.event_id}">{ev.event_id}</a>)
                            </td>
                        {:else}
                            <td>
                                <a href="/app?event_type_id={ev.event_id}">{ev.event_id}</a>
                            </td>
                        {/if}
                    </tr>
                    {#if ev.time_created && ev.time_created.system_time}
                        <tr>
                            <td>Local Time Created</td>
                            <td>{ev.time_created.system_time.toLocaleDateString()} {ev.time_created.system_time.toLocaleTimeString()}</td>
                        </tr>
                    {/if}
                    <tr>
                        <td>Local Event Record ID</td>
                        <td>{ev.event_record_id}</td>
                    </tr>
                    {#if ev.correlation && ev.correlation.activity_id}
                        <tr>
                            <td>Correlation</td>
                            <td>
                                <a href="/app?correlation={stripBraces(ev.correlation.activity_id)}">
                                    {stripBraces(ev.correlation.activity_id)}
                                </a>
                            </td>
                        </tr>
                    {/if}
                    {#if ev.execution}
                        <tr>
                            <td>Execution</td>
                        </tr>
                        <tr>
                            <td class="sub-key">Process ID</td>
                            <td>{ev.execution.process_id}</td>
                        </tr>
                        <tr>
                            <td class="sub-key">Thread ID</td>
                            <td>{ev.execution.thread_id}</td>
                        </tr>
                    {/if}
                    <tr>
                        <td>Channel</td>
                        <td>
                            <a href="/app?channel={ev.channel}">{ev.channel}</a>
                        </td>
                    </tr>
                    <tr>
                        <td>Hostname</td>
                        <td>{ev.computer}</td>
                    </tr>
                    </tbody>
                </table>
            </Card>
        </section>

        <section id="data">
            <Card header="Event Data">
                <table>
                    <tbody>
                    {#each event.event.event_data as data_point}
                        <tr class="event-data">
                            <td>{data_point.name || "Unnamed Property"}</td>
                            <td>{data_point.value || "No Data"}</td>
                        </tr>
                    {/each}
                    </tbody>
                </table>
            </Card>
        </section>
    </section>
{/if}

<script>
    import {page} from '$app/stores';
    import {onMount} from "svelte";
    import {error} from '@sveltejs/kit';

    import http from '$lib/http';
    import Card from "$lib/components/Card.svelte";
    import EventMessages from "$lib/event_messages.js";
    import {isLoggedIn} from "$lib/auth.js";

    let id = $page.params.id;
    let event;

    onMount(async () => {
        if (!isLoggedIn()) {
            return;
        }

        const res = await http.get(`/events/by-id/${id}`);
        if (res.status !== 200) {
            if (res.status === 404) {
                error(404, {
                    message: 'Event not found'
                });
                return;
            } else {
                error(res.status, {
                    message: res.data.error || 'An error occurred fetching the event'
                });
                return;
            }
        }

        event = res.data;
        event.metadata.received_time = new Date(event.metadata.received_time);

        if (event.event.system.time_created && event.event.system.time_created.system_time) {
            event.event.system.time_created.system_time = new Date(event.event.system.time_created.system_time);
        }
    });

    function stripBraces(str) {
        if (str.startsWith('{') && str.endsWith('}')) {
            return str.slice(1, -1);
        }
    }
</script>

<style>
    tr > td:first-of-type:not(.sub-key) {
        font-weight: bold;
        padding-left: 0;
        width: 200px;
        min-width: 200px;
    }

    a[href] {
        color: var(--primary);
    }

    td:first-child {
        white-space: nowrap;
    }

    td:nth-child(2) {
        overflow: hidden;
        text-overflow: ellipsis;
        word-break: break-word;
    }

    td.sub-key {
        padding-left: 1em;
    }

    tr.event-data > td:first-child {
        vertical-align: top;
    }

    tr.event-data > td:last-child {
        white-space: unset;
    }
</style>