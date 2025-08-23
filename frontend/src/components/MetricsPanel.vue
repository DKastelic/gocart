<template>
  <div 
    class="metrics-panel"
    :style="{ 
      backgroundColor: currentThemeConfig.chartControlBackground,
      borderColor: currentThemeConfig.chartControlBorder
    }"
  >
    <h3 :style="{ color: currentThemeConfig.chartControlTitleColor }">
      Metrike komunikacije
    </h3>
    
    <div class="metrics-grid">
      <div class="metric-group">
        <h4 :style="{ color: currentThemeConfig.chartControlTitleColor }">
          Povprečni čas odgovora
        </h4>
        <div class="metric-cards">
          <div 
            v-for="(cartData, index) in cartMetrics" 
            :key="index"
            class="metric-card"
            :style="{ 
              backgroundColor: currentThemeConfig.chartsBackground,
              borderColor: currentThemeConfig.chartControlBorder,
              color: currentThemeConfig.chartControlTextColor
            }"
          >
            <div class="cart-label">Agent {{ cartData.id }}</div>
            <div class="metric-value">
              {{ formatDuration(cartData.metrics.averageRoundTripTime) }}
            </div>
            <div class="metric-subtext">
              {{ cartData.metrics.roundTripTimeCount }} sporočil
            </div>
          </div>
        </div>
      </div>

      <div class="metric-group">
        <h4 :style="{ color: currentThemeConfig.chartControlTitleColor }">
          Čas od cilja do začetka gibanja
        </h4>
        <div class="metric-cards">
          <div 
            v-for="(cartData, index) in cartMetrics" 
            :key="index"
            class="metric-card"
            :style="{ 
              backgroundColor: currentThemeConfig.chartsBackground,
              borderColor: currentThemeConfig.chartControlBorder,
              color: currentThemeConfig.chartControlTextColor
            }"
          >
            <div class="cart-label">Agent {{ cartData.id }}</div>
            <div class="metric-value">
              {{ formatDuration(cartData.metrics.averageGoalToMovementTime) }}
            </div>
            <div class="metric-subtext">
              {{ cartData.metrics.goalToMovementCount }} premikov
            </div>
          </div>
        </div>
      </div>

      <div class="metric-group">
        <h4 :style="{ color: currentThemeConfig.chartControlTitleColor }">
          Število sporočil v scenariju
        </h4>
        <div class="metric-cards">
          <div 
            v-for="(cartData, index) in cartMetrics" 
            :key="index"
            class="metric-card"
            :style="{ 
              backgroundColor: currentThemeConfig.chartsBackground,
              borderColor: currentThemeConfig.chartControlBorder,
              color: currentThemeConfig.chartControlTextColor
            }"
          >
            <div class="cart-label">Agent {{ cartData.id }}</div>
            <div class="metric-value">
              {{ cartData.metrics.scenarioMessageCount }}
            </div>
            <div class="metric-subtext">
              od tega prošenj: {{ cartData.metrics.totalMessageCount }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useWebSocket, type SocketData } from '@/state';
import { useTheme } from '@/composables/useTheme';

const { latestData } = useWebSocket();
const { currentThemeConfig } = useTheme();

const cartMetrics = computed(() => {
  if (!latestData.value?.carts) return [];
  return latestData.value.carts.filter(cart => cart.metrics);
});

function formatDuration(nanoseconds: number): string {
  if (nanoseconds === 0) return '0ms';
  
  const milliseconds = nanoseconds / 1_000_000;
  
  if (milliseconds < 1) {
    return `${(nanoseconds / 1_000).toFixed(1)}μs`;
  } else if (milliseconds < 1000) {
    return `${milliseconds.toFixed(1)}ms`;
  } else {
    return `${(milliseconds / 1000).toFixed(2)}s`;
  }
}
</script>

<style scoped>
.metrics-panel {
  border: 1px solid;
  border-radius: 8px;
  padding: 16px;
  margin: 16px;
  min-width: 600px;
}

.metrics-grid {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.metric-group h4 {
  margin: 0 0 12px 0;
  font-size: 16px;
  font-weight: 600;
}

.metric-cards {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.metric-card {
  border: 1px solid;
  border-radius: 6px;
  padding: 12px;
  min-width: 120px;
  text-align: center;
  transition: transform 0.2s ease;
}

.metric-card:hover {
  transform: translateY(-2px);
}

.cart-label {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 4px;
  opacity: 0.8;
}

.metric-value {
  font-size: 18px;
  font-weight: 700;
  margin-bottom: 4px;
}

.metric-subtext {
  font-size: 11px;
  opacity: 0.6;
}

@media (max-width: 768px) {
  .metrics-panel {
    min-width: auto;
    margin: 8px;
  }
  
  .metric-cards {
    justify-content: center;
  }
  
  .metric-card {
    min-width: 100px;
  }
}
</style>
