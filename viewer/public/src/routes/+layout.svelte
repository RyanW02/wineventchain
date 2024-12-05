<svelte:head>
    <title>WinEventChain Viewer</title>

    <link rel="stylesheet" href="/style.css"/>

    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css" />
</svelte:head>

<main>
    <slot/>

    <div class="toasts">
        {#each $toasts as toast}
            <Toast success={toast.success} on:close={() => removeToast(toast)}>{toast.content}</Toast>
        {/each}
    </div>
</main>

<script>
    import Toast from "$lib/components/Toast.svelte";
    import {toasts, removeToast} from '$lib/stores.js';
</script>

<style>
    @import url('https://fonts.googleapis.com/css2?family=Kanit:wght@700;800&family=Nanum+Gothic:wght@400;700;800&family=Poppins:ital,wght@0,100;0,200;0,300;0,400;0,500;0,600;0,700;0,800;0,900;1,100;1,200;1,300;1,400;1,500;1,600;1,700;1,800;1,900&display=swap');

    :global(body) {
        margin: 5px;
        background-color: var(--background);
    }

    main {
        display: flex;
        justify-content: center;

        font-family: "Poppins", sans-serif;
        font-size: var(--base-font-size);
        color: var(--text);
    }

    .toasts {
        display: flex;
        flex-direction: column;
        gap: 1vh;
        position: absolute;

        top: 10px;
        right: 10px;

        width: 20%;
        min-width: 200px;
    }
</style>