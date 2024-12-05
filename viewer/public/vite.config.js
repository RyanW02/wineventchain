import {sveltekit} from '@sveltejs/kit/vite';
import {defineConfig, loadEnv} from 'vite';

export default defineConfig(({command, mode}) => {
    const env = loadEnv(mode, process.cwd(), '')

    return {
        plugins: [sveltekit()],
        define: {
            '__DEFAULT_API_URL__': `"${env.DEFAULT_API_URL || "http://localhost:4000"}"`,
        }
    };
});
