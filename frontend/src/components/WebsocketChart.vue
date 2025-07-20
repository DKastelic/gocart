<template>
  <div class="websocket-chart">
    <v-chart :option="chartOptions" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { use } from 'echarts/core'
import VChart from 'vue-echarts'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { TooltipComponent, TitleComponent, GridComponent } from 'echarts/components'
import type { SocketData } from '../state'

use([CanvasRenderer, LineChart, TooltipComponent, TitleComponent, GridComponent])

const props = defineProps<{
  title: string
  cart_id: number
  value_type: "position" | "velocity" | "acceleration" | "jerk"
  data: SocketData | null
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
    max: function (value: { min: number, max: number }) {
      // undefined will let the internal logic decide the max value
      return value.max < 1 ? 1 : undefined; 
    },
  },
  series: [
    {
      name: 'Value',
      type: 'line',
      showSymbol: false,
      data: [] as [string, number][],
    },
  ],
  animation: false,
})

let updateTimeout: ReturnType<typeof setTimeout> | null = null
let pendingData: {data: SocketData, time: string}[] = []

watch(() => props.data, (newData) => {
  if (!newData) return
  if (!pendingData) pendingData = []
  pendingData.push({data: newData, time: newData.timestamp}) // Use backend timestamp

  if (!updateTimeout) {
    updateTimeout = setTimeout(() => {
      const currentData = chartOptions.value.series[0].data as [string, number][]
      if (pendingData && pendingData.length) {
        // Use Array.prototype.push.apply for batch insert
        const newPoints = pendingData.map(d => [d.time, d.data[props.value_type] as number] as [string, number])
        Array.prototype.push.apply(currentData, newPoints)
        // Slice to keep only the last 600 points
        chartOptions.value.series[0].data = currentData.slice(-600)
        pendingData = []
      }
      updateTimeout = null
    }, 100)
  }
})
</script>

<style scoped>
.websocket-chart {
  width: 100%;
  height: 350px;
  background: white;
  border-radius: 12px;
  padding: 1rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  border: 1px solid rgba(0, 0, 0, 0.05);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.websocket-chart:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
}

@media (max-width: 900px) {
  .websocket-chart {
    height: 300px;
    padding: 0.8rem;
  }
}

@media (max-width: 600px) {
  .websocket-chart {
    height: 250px;
    padding: 0.5rem;
  }
}
</style>