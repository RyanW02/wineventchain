export function isLoggedIn() {
    return window.localStorage.getItem('token') && window.localStorage.getItem('server_url');
}