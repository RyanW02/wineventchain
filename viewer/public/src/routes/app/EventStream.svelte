<section>
    {#if ws && ws.readyState === 1}
        <FilterBar bind:filters refreshButton={false} on:add={updateFilters} on:remove={updateFilters} />
        <div class="events">
            {#each events as event}
                <div>
                    <EventPreview {event}/>
                </div>
            {/each}
        </div>
    {:else}
        <div class="centre">
            <p class="connecting-message">Connecting to event stream...</p>
            <Spinner />
        </div>
    {/if}
</section>

<script>
    import Spinner from '$lib/components/Spinner.svelte';
    import {onMount} from "svelte";
    import {addToast} from "$lib/stores.js";
    import EventPreview from "./EventPreview.svelte";
    import FilterBar from "./FilterBar.svelte";
    import http from "$lib/http.js";

    let ws;
    let events = [];
    let filters = [];

    function connect() {
        if (window.localStorage.getItem('server_url') === null) {
            return;
        }

        const url = new URL(window.localStorage.getItem('server_url'));
        if (url.protocol === 'https:') {
            url.protocol = 'wss:';
        } else {
            url.protocol = 'ws:';
        }

        url.pathname = '/events/stream';

        ws = new WebSocket(url.toString());

        ws.onopen = () => {
            console.log("Websocket connected");
            events = [];

            ws.send(JSON.stringify({
                type: 'auth',
                data: {
                    token: window.localStorage.getItem('token')
                }
            }));

            updateFilters();
        };

        ws.onclose = () => {
            console.log("Websocket closed; trying reconnect in 5 seconds...");

            setTimeout(() => {
                connect();
            }, 5000);
        };

        ws.onerror = (e) => {
            console.error('error', e);
        };

        ws.onmessage = (e) => {
            try {
                const message = JSON.parse(e.data);

                if (message.type === 'event') {
                    events = [message.data, ...events];

                    if (events.length > 50) {
                        events = events.slice(0, 50);
                    }
                } else if (message.type === 'error') {
                    addToast(false, message.data);
                } else {
                    console.error('Unknown message type', message.type);
                    addToast(false, 'Unknown message type from event stream');
                }
            } catch (e) {
                console.error('Failed to parse message', e);
                addToast(false, 'Failed to parse message from event stream');
            }
        };
    }

    async function updateFilters() {
        ws.send(JSON.stringify({
            type: 'subscribe',
            data: filters
        }));

        const previous = await fetchPrevious();
        events = previous;
    }

    async function fetchPrevious() {
        try {
            let path = `/events`;
            const res = await http.post(path, {filters});
            if (res.status !== 200) {
                addToast(false, res.data.error || "Failed to fetch events from server");
                return;
            }

            return res.data;
        } catch (e) {
            console.error(e);
            addToast(false, "Failed to search events");
            return [];
        }
    }

    onMount(() => {
        connect();
    });
</script>

<style>
    section {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .centre {
        display: flex;
        flex-direction: column;
        align-items: center;
    }

    .events {
        display: flex;
        flex-direction: column;
    }

    .events > div:not(:last-child) {
        border-bottom: 1px solid var(--text-gray);
    }
</style>
