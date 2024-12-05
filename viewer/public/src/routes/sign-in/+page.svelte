<section>
    <Card header="Sign In">
        <Input bind:value={serverAddress} label="Server Address" placeholder={DEFAULT_API_URL}/>

        <Input bind:value={principal} label="Principal Name (Case Sensitive)" placeholder="admin"/>

        <div class="input-col">
            <input bind:this={fileInput} type="file" bind:files hidden/>

            <label>Private Key</label>
            <Button on:click={() => fileInput.click()}>
                {#if files.length > 0}
                    {files[0].name}
                {:else}
                    Choose File
                {/if}
            </Button>
        </div>

        <div slot="footer">
            <Button on:click={signIn}>Sign In</Button>
        </div>
    </Card>
</section>

<script>
    import axios from "axios";
    import {signAsync} from '$lib/vendor/ed25519.js';
    import Card from "$lib/components/Card.svelte";
    import Input from "../../lib/components/Input.svelte";
    import Button from "../../lib/components/Button.svelte";
    import {addToast} from "../../lib/stores.js";
    import {goto} from "$app/navigation";
    import http from "$lib/http.js";

    const DEFAULT_API_URL = __DEFAULT_API_URL__;

    let fileInput;
    let files = [];
    let serverAddress = '';
    let principal = '';

    async function signIn() {
        try {
            new URL(serverAddress || DEFAULT_API_URL);
        } catch (e) {
            addToast(false, 'Invalid server address');
            return;
        }

        if (principal.length === 0) {
            addToast(false, 'Please enter a principal name');
            return;
        }

        if (files.length === 0) {
            addToast(false, 'Please select a private key file');
            return;
        }

        // Decode private key
        const file = files[0];
        const combinedKey = base64Decode(await file.text());

        const privKey = combinedKey.length > 32 ? combinedKey.slice(0, 32) : combinedKey;

        // Fetch challenge
        const origin = new URL(serverAddress || DEFAULT_API_URL).origin;
        const options = {
            baseURL: origin,
            validateStatus: () => true
        };

        let challenge;
        try {
            const res = await axios.post('/auth/challenge', {principal}, options);
            if (res.status !== 200) {
                if (res.data.error) {
                    addToast(false, res.data.error);
                } else {
                    addToast(false, 'Failed to fetch challenge');
                }

                return;
            }

            challenge = res.data.challenge;
        } catch (e) {
            addToast(false, 'Failed to fetch challenge');
            return;
        }

        // Generate signature and convert to base64
        const signature = await signAsync(base64Decode(challenge), privKey);
        const signatureEncoded = btoa(String.fromCodePoint(...signature));

        // Submit challenge
        try {
            const data = {
                principal: principal,
                challenge: challenge,
                response: signatureEncoded
            };

            const res = await axios.post('/auth/challenge-response', data, options);
            if (res.status !== 200) {
                console.log(res.data)
                if (res.data.error) {
                    addToast(false, res.data.error);
                } else {
                    addToast(false, 'Failed to sign in');
                }

                return;
            }

            window.localStorage.setItem('token', res.data.token);
            window.localStorage.setItem('server_url', origin);

            // Update axios object
            http.defaults.baseURL = origin;
            http.defaults.headers = {'Authorization': res.data.token};

            addToast(true, 'Signed in successfully');

            goto('/app');
        } catch (e) {
            addToast(false, 'Failed to sign in');
        }
    }

    function base64Decode(base64String) {
        const decoded = atob(base64String);
        const bytes = new Uint8Array(decoded.length);
        for (let i = 0; i < decoded.length; i++) {
            bytes[i] = decoded.charCodeAt(i);
        }

        return bytes;
    }
</script>

<style>
    section {
        display: flex;
        justify-content: center;

        width: 50%;
        max-width: 400px;

        margin-top: 20vh;
    }

    .input-col {
        display: flex;
        flex-direction: column;
    }

    @media only screen and (max-width: 768px) {
        section {
            width: 100%;
        }
    }
</style>
