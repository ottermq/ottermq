import { api } from "$lib/services/api";

type JwtPayload = {
    username?: string;
    role?: string;
    exp?: number;
};

function decodeJwtPayload(token: string): JwtPayload {
    try {
        return JSON.parse(atob(token.split('.')[1]));
    } catch {
        return {};
    }
}

function getInitialToken() {
    if (typeof localStorage === 'undefined') {
        return '';
    }

    return localStorage.getItem('ottermq_token') ?? '';
}

const initialToken = getInitialToken();
const payload = decodeJwtPayload(initialToken);

export const auth = $state({
    token: initialToken,
    username: payload.username ?? '',
    role: payload.role ?? '',
    loading: false,
    error: ''
});

export async function login(username: string, password: string) {
    auth.loading = true;
    auth.error = '';
    try {
        const data = await (await api.post('/api/login', { username, password })).json();
        const token = data?.token;
        if (!token) throw new Error('Token missing');
        auth.token = token
        const payload = decodeJwtPayload(token);
        auth.username = payload?.username ?? '';
        auth.role = payload?.role ?? '';
        localStorage.setItem('ottermq_token', token)

    } catch (err) {
        auth.error = 'Failed to login: ' + err
        throw err
    } finally {
        auth.loading = false;
    }
}

export async function logout() {
    auth.token = '';
    auth.username = '';
    auth.role = '';
    localStorage.removeItem('ottermq_token');
}