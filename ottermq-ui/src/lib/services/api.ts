
import { auth } from "$lib/stores/auth.svelte";
export async function apiFetch(
    url: string,
    options: RequestInit = {}
): Promise<Response> {

    const headers = new Headers(options.headers || {});
    if (auth.token) {
        headers.set("Authorization", `Bearer ${auth.token}`)
    }
    headers.set("Content-type", "application/json");

    const response = await fetch(url, {
        ...options,
        headers,
    });

    if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
    }

    return response;
}

export const api = {
    get: (url: string) => apiFetch(url),
    post: (url: string, body: unknown) =>
        apiFetch(url, {
            method: 'POST',
            body: JSON.stringify(body)
        }),
    put: (url: string, body: unknown) =>
        apiFetch(url, {
            method: 'PUT',
            body: JSON.stringify(body)
        }),
    delete: (url: string, body: unknown) =>
        apiFetch(url, {
            method: 'DELETE',
            body: JSON.stringify(body)
        }),
}