<!--lib/components/Statcard.svelte -->
<script lang="ts">
    interface Props {
        title: string;
        value: number;
    }
    let { title, value }: Props = $props()
    
</script>

<div class="stat-card">
    <div class="stat-data">
        <span>{value}</span>
    </div>
    <div class="stat-name">
        <strong>{title}</strong>
    </div>
</div>

<style>
    .stat-card {
        text-align: center;
        border: 1px solid #ddd;
        border-radius: 4px;
        padding: 16px;
    }

    .stat-card strong {
        font-size: 12px;
        display: block;
        color: #757575;
        margin-top: 8px;
    }

    .stat-card span {
        display:  block;
        font-size: 48px;
    }
</style>