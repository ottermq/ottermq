export function usePolledList<T>(fetcher: () => Promise<T[] | null>, intervalMs = 5000) {
    let items = $state<T[] | null>(null)
    $effect(() => {
        function load() { fetcher().then((v) => (items = v)); }
        load();
        const interval = setInterval(load, intervalMs);
        return () => clearInterval(interval);
    });
    return { get items() { return items; } };
}