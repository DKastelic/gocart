<template>
  <div class="websocket-chart" style="width: 800px; height: 400px;">
    <v-chart :option="chartOptions" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { use } from 'echarts/core'
import VChart from 'vue-echarts'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { TooltipComponent, TitleComponent, GridComponent } from 'echarts/components'
import { registerCallback } from '../state'

use([CanvasRenderer, LineChart, TooltipComponent, TitleComponent, GridComponent])

const props = defineProps<{
  title: string
  cart_id: number
  value_type: "position" | "velocity" | "acceleration" | "jerk"
}>()

const chartOptions = ref({
  title: {
    text: props.title,
  },
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      animation: false,
    },
  },
  xAxis: {
    type: 'time',
  },
  yAxis: {
    type: 'value',
  },
  series: [
    {
      name: 'Value',
      type: 'line',
      data: [] as [string, number][],
    },
  ],
  animation: false,
})

onMounted(() => {
  registerCallback((data) => {
    if (data.id !== props.cart_id) {
      return
    }

    const currentData = chartOptions.value.series[0].data as [string, number][]
    currentData.push([new Date().toISOString(), data[props.value_type] as number])
    
    // Limit the number of data points to the last 100
    if (currentData.length > 100) {
      currentData.shift()
    }
    
    chartOptions.value.series[0].data = currentData
  })
})
</script>