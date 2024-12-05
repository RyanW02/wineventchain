<div class="wrapper">
    {#if header}
        <div class="header">
            {header}
        </div>
    {/if}
    <div class="content">
        <slot/>
    </div>

    {#if $$slots.footer}
        <div class="footer">
            <div>
                <slot name="footer"/>
            </div>
        </div>
    {/if}
</div>

<script>
    export let header;
</script>

<style>
    .wrapper {
        display: inline-flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        background-color: var(--background);
        border-radius: 3px;

        width: 100%;
    }

    .wrapper > div {
        flex-basis: 100%;
    }

    .header {
        text-align: center;
        width: 100%;

        background-color: var(--background);
        border-bottom: 1px solid var(--accent);

        border-radius: 3px 3px 0 0;
        padding: 2px 0;

        font-size: 1.5rem;
    }

    .content {
        display: flex;
        flex-direction: column;
        justify-content: center;
        gap: var(--gap);

        width: 98%;
        margin: 0 10px;
        padding: 5px;
    }

    .footer {
        display: flex;
        flex-direction: column;
        justify-content: center;
        gap: var(--gap);

        width: 100%;

        border-top: 1px solid var(--accent);
    }

    .footer > div {
        display: flex;
        justify-content: center;

        padding: 5px;
    }
</style>