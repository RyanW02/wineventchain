<script>
    import {onMount} from "svelte";
    import {goto} from "$app/navigation";

    onMount(() => {
        window.localStorage.clear();

        goto('/');
    });
</script>