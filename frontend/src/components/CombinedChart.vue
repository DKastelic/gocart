<template>
  <div class="combined-chart">
    <v-chart :option="chartOptions" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { use } from 'echarts/core'
import VChart from 'vue-echarts'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { TooltipComponent, TitleComponent, GridComponent, LegendComponent } from 'echarts/components'
import type { SocketData, AllCartsData } from '../state'

use([CanvasRenderer, LineChart, TooltipComponent, TitleComponent, GridComponent, LegendComponent])

const props = defineProps<{
  title: string
  value_type: "chartPosition" | "chartVelocity" | "chartAcceleration" | "chartJerk"
  data: AllCartsData | null
  num_carts: number
  cart_visibility: Record<number, boolean>
}>()

// Generate colors for different carts
const cartColors = ['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#3ba272', '#fc8452', '#9a60b4']

const chartOptions = ref({
  backgroundColor: '#2a2a2a',
  title: {
    text: props.title,
    textStyle: {
      color: '#fff'
    }
  },
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      animation: false,
    },
    backgroundColor: '#444',
    borderColor: '#666',
    textStyle: {
      color: '#fff'
    }
  },
  legend: {
    data: [] as string[],
    top: 30,
    textStyle: {
      color: '#ccc'
    }
  },
  xAxis: {
    type: 'time',
    axisLine: {
      lineStyle: {
        color: '#666'
      }
    },
    axisLabel: {
      color: '#ccc',
      hideOverlap: true, // Hide overlapping labels
    },
    splitLine: {
      lineStyle: {
        color: '#444'
      }
    }
  },
  yAxis: {
    type: 'value',
    max: function (value: { min: number, max: number }) {
      // undefined will let the internal logic decide the max value
      return value.max < 1 ? 1 : undefined; 
    },
    axisLine: {
      lineStyle: {
        color: '#666'
      }
    },
    axisLabel: {
      color: '#ccc'
    },
    splitLine: {
      lineStyle: {
        color: '#444'
      }
    }
  },
  series: [] as any[],
  animation: false,
})

// Initialize series for each cart
function initializeSeries() {
  const series = []
  const legendData = []
  
  for (let i = 1; i <= props.num_carts; i++) {
    const cartName = `Cart ${i}`
    legendData.push(cartName)
    
    series.push({
      name: cartName,
      type: 'line',
      showSymbol: false,
      data: [] as [string, number][],
      itemStyle: {
        color: cartColors[(i - 1) % cartColors.length]
      },
      lineStyle: {
        color: cartColors[(i - 1) % cartColors.length]
      }
    })
  }
  
  chartOptions.value.series = series
  chartOptions.value.legend.data = legendData
}

// Update series visibility based on cart_visibility prop
function updateSeriesVisibility() {
  chartOptions.value.series.forEach((series: any, index: number) => {
    const cartId = index + 1
    const isVisible = props.cart_visibility[cartId]
    
    // Use lineStyle opacity to show/hide lines
    series.lineStyle.opacity = isVisible ? 1 : 0.1
    series.itemStyle.opacity = isVisible ? 1 : 0.1
    
    // Optionally hide the data points when not visible
    if (!isVisible) {
      series.emphasis = { disabled: true }
    } else {
      series.emphasis = { disabled: false }
    }
  })
}

// Initialize series on mount
initializeSeries()
updateSeriesVisibility() // Apply initial visibility settings

// Watch for changes in num_carts prop to reinitialize series
watch(() => props.num_carts, () => {
  initializeSeries()
  updateSeriesVisibility()
})

// Watch for cart visibility changes
watch(() => props.cart_visibility, () => {
  updateSeriesVisibility()
}, { deep: true })

let updateTimeout: ReturnType<typeof setTimeout> | null = null
let pendingData: AllCartsData[] = []

// Time window in milliseconds (e.g., 30 seconds)
const TIME_WINDOW_MS = 30000

function filterDataByTimeWindow(data: [string, number][]): [string, number][] {
  if (data.length === 0) return data
  
  const now = new Date().getTime()
  const cutoffTime = now - TIME_WINDOW_MS
  
  return data.filter(([timestamp]) => {
    const pointTime = new Date(timestamp).getTime()
    return pointTime >= cutoffTime
  })
}

watch(() => props.data, (newData) => {
  if (!newData) return

  // Add to pending updates
  pendingData.push(newData)

  if (!updateTimeout) {
    updateTimeout = setTimeout(() => {
      // Process all pending updates
      pendingData.forEach(dataUpdate => {
        dataUpdate.carts.forEach(cartData => {
          const cartId = cartData.id
          const seriesIndex = cartId - 1
          const series = chartOptions.value.series[seriesIndex]
          
          // Skip if series doesn't exist for this cart
          if (!series) return

          const currentData = series.data as [string, number][]
          const newPoint: [string, number] = [dataUpdate.timestamp, cartData[props.value_type] as number]
          
          // Add the new data point
          currentData.push(newPoint)
        })
      })

      // Apply filtering to all series after processing all updates
      chartOptions.value.series.forEach(series => {
        series.data = filterDataByTimeWindow(series.data)
      })
      
      // Clear pending updates and reset timeout
      pendingData = []
      updateTimeout = null
    }, 10)
  }
})
</script>

<style scoped>
.combined-chart {
  width: 100%;
  height: 300px;
  background: #2a2a2a;
  border: 1px solid #444;
  padding: 10px;
  margin-bottom: 10px;
}

.combined-chart:hover {
  border-color: #555;
}

@media (max-width: 1400px) {
  .combined-chart {
    height: 380px;
  }
}

@media (max-width: 900px) {
  .combined-chart {
    height: 320px;
    padding: 10px;
  }
}

@media (max-width: 600px) {
  .combined-chart {
    height: 280px;
    padding: 8px;
  }
}
</style>
