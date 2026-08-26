<div class="chart-container">
    <h2>Queued Messages</h2>
    <div class="chart" bind:this={chartEL}></div>
</div>



<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import ApexCharts from 'apexcharts'

	import type { MessageStatsTimeSeries, TimeSeries } from "$lib/stores/charts.svelte";
	import { resolveThemeColors } from "$lib/theme";

    let chartEL: HTMLDivElement;
    let chart: ApexCharts | undefined;

    interface Props {
        chartData: MessageStatsTimeSeries | null
    }

    let { chartData }: Props = $props();

    const options: ApexCharts.ApexOptions = {
        chart:
        {
            type:"line",
            height:300,
            toolbar: { show:false },
            animations: { enabled: false },
            zoom: {enabled:false}
        },
        stroke: {
            curve: 'smooth',
            width:2
        },
        xaxis:{
            type: 'datetime',
            labels:{
                datetimeUTC:false,
                format: 'HH:mm:ss'
            }
        },
        yaxis:{
            title:{
                text:'Messages'
            },
            labels:{
                formatter: (val:number) => Math.round(val).toLocaleString()
            }
        },
        tooltip: {
            x: { format:'HH:mm:ss' },
            y:{ formatter: (val:number) => Math.round(val).toLocaleString() }
        },
        legend:{
            position:'top',
            horizontalAlign:'center'
        },
        grid: {
            borderColor: '#f1f1f1'
        },
    }

   const series = $derived.by(() => {
    if (!chartData) return []
    return [
        {
            name: 'Ready',
            data: windowFilter(chartData.ready)
        },
        {
            name: 'Unacked',
            data: windowFilter(chartData.unacked)
        },
        {
            name: 'Total',
            data: windowFilter(chartData.total)
        }
    ]
   });



   const windowFilter = (points:TimeSeries[]) => {
        if (!points || points.length === 0) return [];
        
        const now = Date.now();
        let selectedWindow=1; // currently, 1min -- TODO: make this dynamic (1min, 10min, 1hr)
        const window = now - selectedWindow * 60 * 1000;
        return points
            .filter(d => new Date(d.timestamp).getTime() >= window)
            .map(d=> ({
                x:new Date(d.timestamp).getTime(),
                y:Math.round(d.value)
            }))
   }
   
 onMount(()=> {
        options.colors = resolveThemeColors([
            '--color-series-1',
            '--color-series-2',
            '--color-series-3'
        ]);

        chart = new ApexCharts(chartEL,{...
            options,
            series
        });

        chart.render()
    })

    onDestroy(()=>{
        chart?.destroy(); 
    });

    $effect(()=>{
        chart?.updateSeries(series);
    });

</script>
