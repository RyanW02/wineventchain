<nav>
    <div>
        <div class="logo-wrapper">
            <i class="fa-sharp fa-solid fa-bolt logo"></i>
            <span>WinEventChain</span>
        </div>

        <a href="/app">Home</a>
    </div>

    <div>
        <a href="/sign-out">
            Sign Out
            <i class="fa-sharp fa-solid fa-right-from-bracket"></i>
        </a>
    </div>
</nav>

<style>
    nav, nav > div {
        display: flex;
        align-items: center;
        gap: 1rem;

        font-size: 1rem;
        color: var(--text-gray);
    }

    nav > div:first-child > div:first-child {
        font-size: 1.25rem;
        color: var(--text);
    }

    nav {
        justify-content: space-between;

        border-bottom: var(--primary) solid 1px;
    }

    nav > * > * {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .logo {
        color: var(--primary);
    }
</style>