<div class="chart-container">
    <h2>{title}</h2>
    <div class="chart" bind:this={chartEL}></div>
</div>

<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import ApexCharts from 'apexcharts'

	import type { TimeSeries } from "$lib/stores/charts.svelte";
	import { resolveThemeColors } from "$lib/theme";

    let chartEL: HTMLDivElement;
    let chart: ApexCharts | undefined;

    interface SeriesDef {
        name: string;
        points: TimeSeries[]
        color: string
    }

    interface Props {
        title: string;
        yAxisTitle: string;
        series: SeriesDef[];
        format: (v: number) => string;  // shared by y-axis label
        unit?: string;                  // e.g. 'msg/s' -- tooltip only
        decimals?: number;              // axis, default 0
        tooltipDecimals?: number        // default: same as decimals
        windowMinutes?: number;         // default 1 -- TODO: make this dynamic, like a dropdown (1min, 10min, 1hr)
        min?: number;                   // rates passes 0
    }

    let { 
        title,
        yAxisTitle,
        format,
        unit,
        decimals,
        tooltipDecimals,
        series,
        windowMinutes,
        min
     }: Props = $props();
 
    const nf = (v: number, digits: number) => 
     v.toLocaleString(undefined, {
        minimumFractionDigits: digits,
        maximumFractionDigits: digits
     });

    const axisFormat = (v: number) => nf(v,decimals ?? 0);
    const tooltipFormat = (v: number) => `${nf(v, tooltipDecimals ?? decimals ?? 0)}${unit ? ` ${unit}` : ''}`

    const windowFilter = (points:TimeSeries[]) => {
        if (!points || points.length===0) return[];

        const now = Date.now();
        const selectedWindow = windowMinutes?? 1
        const cutoff = now - selectedWindow * 60 * 1000;
        return points
            .filter(d =>new Date(d.timestamp).getTime() >= cutoff)
            .map(d => ({
                x: new Date(d.timestamp).getTime(),
                y: d.value
            })) 
    }

    const apexSeries = $derived(
        series.map(s=>({
            name:s.name,
            data:windowFilter(s.points)
        }))
    );

    onMount(()=> {
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
                title:{ text: yAxisTitle },
                min,
                labels:{ formatter: axisFormat }
            },
            tooltip: {
                x: { format:'HH:mm:ss' },
                y:{ formatter: tooltipFormat }
            },
            legend:{
                position:'top',
                horizontalAlign:'center'
            },
            grid: {
                borderColor: '#f1f1f1'
            },
        }

        chart = new ApexCharts(chartEL,{...
            options,
            colors: resolveThemeColors(series.map(s=>s.color)),
            series: apexSeries
        });

        chart.render()
    })

    onDestroy(()=>{
        chart?.destroy(); 
    });

    $effect(()=>{
        chart?.updateSeries(apexSeries);
    });


    </script>

