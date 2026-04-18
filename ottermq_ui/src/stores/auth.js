import { defineStore } from 'pinia';
import api from 'src/services/api';

function decodeJwtPayload(token) {
    try {
        return JSON.parse(atob(token.split('.')[1]));
    } catch {
        return {};
    }
}

function roleFromToken(token) {
    return decodeJwtPayload(token)?.role ?? '';
}

export const useAuthStore = defineStore('auth', {
    state: () => {
        const token = localStorage.getItem('ottermq_token') || '';
        return {
            token,
            username: decodeJwtPayload(token)?.username ?? '',
            role: roleFromToken(token),
            loading: false,
            error: null,
        };
    },
    getters: {
        isAuthed: (state) => !!state.token,
        isAdmin: (state) => state.role === 'admin',
    },
    actions: {
        async login(username, password) {
            this.loading = true;
            this.error = null;
            try {
                const { data } = await api.post('/login', { username, password });
                const token = data?.token;
                if (!token) throw new Error('Token missing');
                this.token = token;
                this.username = username;
                this.role = roleFromToken(token);
                localStorage.setItem('ottermq_token', token);
            } catch (e) {
                this.error = e?.response?.data?.error || e.message;
                throw e;
            } finally {
                this.loading = false;
            }
        },
        logout() {
            this.token = '';
            this.username = '';
            this.role = '';
            localStorage.removeItem('ottermq_token');
        },
    },
});