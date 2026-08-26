<div class="chart-container">
    <h2>Message Rates (last 60s)</h2>
    <div class="chart" bind:this={chartEL}></div>
</div>



<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import ApexCharts from 'apexcharts'

	import type { MessageRatesTimeSeries, TimeSeries } from "$lib/stores/charts.svelte";
	import { resolveThemeColors } from "$lib/theme";

    let chartEL: HTMLDivElement;
    let chart: ApexCharts | undefined;

    interface Props {
        chartData: MessageRatesTimeSeries | null
    }

    let { chartData }: Props = $props();

    const options: ApexCharts.ApexOptions = {
        chart:
        {
            type:"line",
            height:300,
            toolbar: { show:false },
            animations: { enabled: false },
            zoom: { enabled:false }
        },
        stroke: {
            curve: 'smooth',
            width: 2.5
        },
        xaxis:{
            type: 'datetime',
            labels:{
                datetimeUTC:false,
                format: 'HH:mm:ss'
            }
        },
        yaxis:{
            title:{ text:'Message Rates' },
            min: 0,
            labels:{
                formatter: (val:number) => val.toFixed(1)
            }
        },    
        tooltip: {
            x: { format: 'HH:mm:ss'},
            y: { formatter: (val:number) => `${val.toFixed(2)} msg/s`},
        },
        legend:{
            position:'top',
            horizontalAlign:'center'
        },
        grid: {
            borderColor: '#f1f1f1'
        },
        dataLabels: {enabled:false},
    }
    const now = Date.now();
    const selectedWindow=1; // currently, 1min -- TODO: make this dynamic (1min, 10min, 1hr)
    const cuttoff = now - selectedWindow * 60 * 1000;

    const transformToSeries = (points:TimeSeries[], name:string) => {
        if (!points || points.length === 0) return null;

        const data = points
            .filter(d => new Date(d.timestamp).getTime() >= cuttoff)
            .map(p => ({
                x:new Date(p.timestamp).getTime(),
                y:Math.max(0, Number(p.value.toFixed(2)))
            }))

        return data.length > 0 ? { name, data } : null
   }

   const series = $derived.by(() => {
    if (!chartData) return []

    return [
        transformToSeries(chartData.publish, 'Publish'),
        transformToSeries(chartData.deliver_auto_ack, 'Deliver (auto ack)'),
        transformToSeries(chartData.deliver_manual_ack, 'Deliver (manual ack)'),
        transformToSeries(chartData.ack, 'Ack')
    ].filter((s): s is {name:string; data: {x:number, y:number}[] } => s != null);
   });
   
   
 onMount(()=> {
        options.colors = resolveThemeColors([
            '--color-series-1',
            '--color-series-8',
            '--color-series-5',
            '--color-series-4',
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
