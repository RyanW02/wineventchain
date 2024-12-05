import {writable} from 'svelte/store';

export const toasts = writable([]);

export function addToast(success, content) {
    if (content.length > 0) {
        content = content.charAt(0).toUpperCase() + content.slice(1);
    }

    const toast = {success, content};
    toasts.update(t => [...t, toast]);

    // Remove toast after 15 seconds
    setTimeout(() => {
        removeToast(toast);
    }, 10 * 1000);
}

export function removeToast(toast) {
    toasts.update(t => t.filter(t => t !== toast));
}
