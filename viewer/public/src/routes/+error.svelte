<section>
    <div class="error">
        <h1 id="code">{$page.status}</h1>
        <span id="message">{$page.error.message}</span>
    </div>
    <div class="button-wrapper">
        <Button on:click={() => goto('/')}>Home</Button>
    </div>
</section>

<script>
    import { page } from '$app/stores';
    import Button from "../lib/components/Button.svelte";
    import {goto} from "$app/navigation";
</script>

<style>
    section {
        display: grid;
        margin-top: 20vh;
        gap: 10vh;
    }

    .button-wrapper {
        display: flex;
        align-self: center;
        justify-self: center
    }

    .error {
        display: flex;
        flex-direction: column;
        align-items: center;
    }

    .error > h1 {
        font-weight: normal;
        font-size: 7rem;
        margin: 0;
    }

    .error > span {
        margin-top: -20px;
    }
</style>