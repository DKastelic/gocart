<template>
  <div 
    class="combined-chart"
    :style="{ 
      backgroundColor: currentThemeConfig.chartBackground,
      borderColor: currentThemeConfig.chartBorder
    }"
  >
    <v-chart :option="chartOptions" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { use } from 'echarts/core'
import VChart from 'vue-echarts'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { TooltipComponent, TitleComponent, GridComponent, LegendComponent, MarkLineComponent } from 'echarts/components'
import type { AllCartsData } from '../state'
import { useWebSocket } from '../state'
import { useTheme } from '../composables/useTheme'

use([CanvasRenderer, LineChart, TooltipComponent, TitleComponent, GridComponent, LegendComponent, MarkLineComponent])

const { currentThemeConfig } = useTheme()
const { clearChartsSignal } = useWebSocket()

const props = defineProps<{
  title: string
  value_type: "chartPosition" | "chartVelocity" | "chartAcceleration" | "chartJerk"
  data: AllCartsData | null
  num_carts: number
  cart_visibility: Record<number, boolean>
  show_trajectory_transitions: boolean
}>()

// Generate colors for different carts
const cartColors = ['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#3ba272', '#fc8452', '#9a60b4']

const unitMap: Record<string, string> = {
  chartPosition: 'mm',
  chartVelocity: 'mm/s',
  chartAcceleration: 'mm/s²',
  chartJerk: 'mm/s³'
}

const chartOptions = ref({
  backgroundColor: '',
  title: {
    text: props.title,
    textStyle: {
      color: ''
    }
  },
  tooltip: {
    trigger: 'axis',
    axisPointer: {
      animation: false,
    },
    backgroundColor: '',
    borderColor: '',
    textStyle: {
      color: ''
    }
  },
  legend: {
    data: [] as string[],
    top: 30,
    textStyle: {
      color: ''
    }
  },
  xAxis: {
    type: 'time',
    axisLine: {
      lineStyle: {
        color: ''
      }
    },
    axisLabel: {
      color: '',
      hideOverlap: true, // Hide overlapping labels
    },
    splitLine: {
      lineStyle: {
        color: ''
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
        color: ''
      }
    },
    axisLabel: {
      color: '',
      formatter: '{value} ' + (unitMap[props.value_type] || '')
    },
    splitLine: {
      lineStyle: {
        color: ''
      }
    }
  },
  series: [] as any[],
  animation: false,
})

// Function to update chart colors based on current theme
function updateChartColors() {
  const config = currentThemeConfig.value
  chartOptions.value.backgroundColor = config.chartBackground
  chartOptions.value.title.textStyle.color = config.chartTitleColor
  chartOptions.value.tooltip.backgroundColor = config.chartTooltipBackground
  chartOptions.value.tooltip.borderColor = config.chartTooltipBorder
  chartOptions.value.tooltip.textStyle.color = config.chartTooltipColor
  chartOptions.value.legend.textStyle.color = config.chartLegendColor
  chartOptions.value.xAxis.axisLine.lineStyle.color = config.chartAxisColor
  chartOptions.value.xAxis.axisLabel.color = config.chartAxisLabelColor
  chartOptions.value.xAxis.splitLine.lineStyle.color = config.chartSplitLineColor
  chartOptions.value.yAxis.axisLine.lineStyle.color = config.chartAxisColor
  chartOptions.value.yAxis.axisLabel.color = config.chartAxisLabelColor
  chartOptions.value.yAxis.splitLine.lineStyle.color = config.chartSplitLineColor
}

// Watch for theme changes and update colors
watch(currentThemeConfig, () => {
  updateChartColors()
}, { immediate: true })

// Initialize series for each cart
function initializeSeries() {
  const series = []
  const legendData = []
  
  for (let i = 1; i <= props.num_carts; i++) {
    const cartName = `Agent ${i}`
    if (props.num_carts > 1) {
      legendData.push(cartName)
    }

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
      },
      markLine: {
        silent: true,
        animation: false,
        symbol: 'none',
        lineStyle: {
          type: 'dashed',
          color: '#888888',
          width: 1,
          opacity: 0.6
        },
        data: [] as any[]
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

// Watch for trajectory transitions visibility changes
watch(() => props.show_trajectory_transitions, () => {
  updateTrajectoryTransitionLines()
})

// Function to clear all chart data
function clearChartData() {
  // Clear all series data
  chartOptions.value.series.forEach(series => {
    series.data = []
    if (series.markLine) {
      series.markLine.data = []
    }
  })
  
  // Clear trajectory transitions
  trajectoryTransitions.value.clear()
}

// Watch for clear charts signal (when scenarios are run)
watch(clearChartsSignal, () => {
  clearChartData()
})

let updateTimeout: ReturnType<typeof setTimeout> | null = null
let pendingData: AllCartsData[] = []

// Store trajectory transition timestamps and phases for vertical lines
const trajectoryTransitions = ref<Map<string, string>>(new Map()) // timestamp -> phase label

// Time window in milliseconds (e.g., 30 seconds)
const TIME_WINDOW_MS = 10000

function filterDataByTimeWindow(data: [string, number][]): [string, number][] {
  if (data.length === 0) return data
  
  const now = new Date().getTime()
  const cutoffTime = now - TIME_WINDOW_MS
  
  return data.filter(([timestamp]) => {
    const pointTime = new Date(timestamp).getTime()
    return pointTime >= cutoffTime
  })
}

// Filter trajectory transitions by time window
function filterTransitionsByTimeWindow(transitions: Map<string, string>): Array<{timestamp: string, phase: string}> {
  const now = new Date().getTime()
  const cutoffTime = now - TIME_WINDOW_MS
  
  return Array.from(transitions.entries())
    .filter(([timestamp]) => {
      const pointTime = new Date(timestamp).getTime()
      return pointTime >= cutoffTime
    })
    .map(([timestamp, phase]) => ({ timestamp, phase }))
}

// Update trajectory transitions with new data
function updateTrajectoryTransitions(data: AllCartsData) {
  if (!props.show_trajectory_transitions) return
  
  data.carts.forEach(cartData => {
    cartData.trajectoryTransitions.forEach((timestamp, index) => {
      const phase = cartData.trajectoryPhases[index] || 'Unknown'
      trajectoryTransitions.value.set(timestamp, phase)
    })
  })
  
  // Filter old transitions outside the time window
  const validTransitions = filterTransitionsByTimeWindow(trajectoryTransitions.value)
  trajectoryTransitions.value.clear()
  validTransitions.forEach(({ timestamp, phase }) => trajectoryTransitions.value.set(timestamp, phase))
}

// Update vertical lines for trajectory transitions
function updateTrajectoryTransitionLines() {
  if (!props.show_trajectory_transitions) {
    // Clear all markLine data when disabled
    chartOptions.value.series.forEach(series => {
      if (series.markLine) {
        series.markLine.data = []
      }
    })
    return
  }
  
  const validTransitions = filterTransitionsByTimeWindow(trajectoryTransitions.value)

  const translations: { [key: string]: string } = {
    "Start": "(1)",
    "Increasing acceleration": "(1)",
    "Constant acceleration": "(2)",
    "Decreasing acceleration": "(3)",
    "Constant velocity": "(4)",
    "Increasing deceleration": "(5)",
    "Constant deceleration": "(6)",
    "Decreasing deceleration": "(7)",
    "Final state": "(8)",
  }
  
  // Add vertical lines for each transition timestamp with phase labels
  const markLineData = validTransitions.map(({ timestamp, phase }) => ({
    xAxis: timestamp,
    lineStyle: {
      type: 'dashed',
      color: '#000000',
      width: 1,
      opacity: 0.6
    },
    label: {
      show: true,
      formatter: translations[phase] || phase,
      rotate: 0, // Angle the text to prevent overlap
      color: '#000000',
      fontSize: 12,
      fontWeight: 'normal',
      backgroundColor: 'rgba(255, 255, 255, 0.8)',
      borderRadius: 2,
      // padding: [2, 4], 
      align: 'center',
    }
  }))
  
  // Update markLine data for the first series only (to avoid duplicate lines)
  if (chartOptions.value.series.length > 0 && chartOptions.value.series[0].markLine) {
    chartOptions.value.series[0].markLine.data = markLineData
  }
}

watch(() => props.data, (newData) => {
  if (!newData) return

  // Add to pending updates
  pendingData.push(newData)

  if (!updateTimeout) {
    updateTimeout = setTimeout(() => {
      // Process all pending updates
      pendingData.forEach(dataUpdate => {
        // Update trajectory transitions
        updateTrajectoryTransitions(dataUpdate)
        
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
      
      // Update trajectory transition lines
      updateTrajectoryTransitionLines()
      
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
  height: 290px;
  border: 1px solid;
  padding: 10px;
  transition: all 0.2s ease;
}

.combined-chart:hover {
  opacity: 0.9;
}
</style>
