<div class="spinner"/>

<style>
    .spinner {
        display: inline-block;
        width: 50px;
        height: 50px;
        border: 3px solid var(--background);
        border-radius: 50%;
        border-top-color: var(--primary);
        animation: spin 1s infinite linear;
        -webkit-animation: spin 1s infinite linear;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
            -webkit-transform: rotate(360deg);
        }
    }

    @-webkit-keyframes spin {
        to {
            transform: rotate(360deg);
            -webkit-transform: rotate(360deg);
        }
    }
</style>