import {goto} from "$app/navigation";
import {addToast} from "./stores.js";
import axios from "axios";

export default axios.create({
    baseURL: window.localStorage.getItem('server_url'),
    headers: {
        'Authorization': window.localStorage.getItem('token')
    },
    validateStatus: () => true,
    interceptors: {
        response: [
            (response) => {
                if (response.status === 401) {
                    window.localStorage.removeItem('token');
                    addToast(false, 'Your session has expired. Please sign in again.');
                    goto('/sign-in');
                } else {
                    return response;
                }
            },
            (error) => {
                return Promise.reject(error);
            }
        ]
    }
});
