<div class="wrapper">
    <Navbar />
    <slot />
</div>

<script>
    import {onMount} from 'svelte';
    import {isLoggedIn} from '$lib/auth.js';
    import Navbar from "./Navbar.svelte";
    import { goto } from '$app/navigation';

    onMount(() => {
        if (!isLoggedIn()) {
            goto('/sign-in');
        }
    });
</script>

<style>
    .wrapper {
        display: flex;
        flex-direction: column;

        width: 100%;
    }
</style>