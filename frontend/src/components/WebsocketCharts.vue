<template>
    <div 
        class="charts-container"
        :style="{ backgroundColor: currentThemeConfig.chartsBackground }"
    >
        <div class="content-layout">
            <div 
                class="cart-controls"
                :style="{ 
                    backgroundColor: currentThemeConfig.chartControlBackground,
                    borderColor: currentThemeConfig.chartControlBorder
                }"
            >
                <h3 :style="{ color: currentThemeConfig.chartControlTitleColor }">Cart Visibility</h3>
                <div class="cart-checkboxes">
                    <label 
                        v-for="cartId in numColumns" 
                        :key="cartId" 
                        class="cart-checkbox"
                        :style="{ color: currentThemeConfig.chartControlTextColor }"
                    >
                        <input 
                            type="checkbox" 
                            v-model="cartVisibility[cartId]"
                            @change="updateVisibility"
                        />
                        <span>Cart {{ cartId }}</span>
                    </label>
                </div>
            </div>
            <div class="charts-grid">
                <CombinedChart 
                    v-for="type in chartTypes" 
                    :key="type"
                    :title="getChartTitle(type)" 
                    :value_type="type" 
                    :data="allCartsData" 
                    :num_carts="numColumns"
                    :cart_visibility="cartVisibility"
                />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue';
import CombinedChart from './CombinedChart.vue';
import { useWebSocket, type SocketData, type AllCartsData } from '@/state';
import { useTheme } from '@/composables/useTheme';

const { currentThemeConfig } = useTheme();

const numColumns = ref(2);
const chartTypes = ['chartPosition', 'chartVelocity', 'chartAcceleration', 'chartJerk'] as const;
const allCartsData = ref<AllCartsData | null>(null);

const { latestData } = useWebSocket();

// Initialize cart visibility - only Cart 1 enabled by default
const cartVisibility = reactive<Record<number, boolean>>({});

// Function to update cart visibility based on the number of carts
function updateCartVisibility(numCarts: number) {
  // Add new carts (disabled by default, except Cart 1)
  for (let i = 1; i <= numCarts; i++) {
    if (!(i in cartVisibility)) {
      cartVisibility[i] = i === 1; // Only Cart 1 is enabled by default
    }
  }
  
  // Remove carts that no longer exist
  Object.keys(cartVisibility).forEach(key => {
    const cartId = parseInt(key)
    if (cartId > numCarts) {
      delete cartVisibility[cartId]
    }
  })
}

// Initialize with default 2 carts
updateCartVisibility(numColumns.value)

function updateVisibility() {
  // This function will be called when checkboxes change
  // The reactive cartVisibility object will automatically trigger updates
}

// Watch for changes in latestData
watch(latestData, (newData) => {
  if (newData) {
    allCartsData.value = {
      carts: [...newData.carts], // Create mutable copy
      timestamp: newData.timestamp
    };
    
    // Update number of columns based on actual cart data
    const actualNumCarts = newData.carts.length;
    if (actualNumCarts !== numColumns.value) {
      numColumns.value = actualNumCarts;
      updateCartVisibility(actualNumCarts);
    }
  }
}, { immediate: true });

function getChartTitle(type: typeof chartTypes[number]): string {
  const titles: Record<typeof chartTypes[number], string> = {
    chartPosition: 'Position - All Carts',
    chartVelocity: 'Velocity - All Carts',
    chartAcceleration: 'Acceleration - All Carts',
    chartJerk: 'Jerk - All Carts'
  };
  return titles[type];
}
</script>

<style scoped>
.charts-container {
  width: 100%;
  padding: 0;
  height: 100%;
  overflow: hidden;
}

.content-layout {
  display: flex;
  gap: 15px;
  width: 100%;
  height: 100%;
  padding: 10px;
}

.cart-controls {
  border: 1px solid;
  padding: 10px;
  width: 150px;
  flex-shrink: 0;
  height: fit-content;
}

.cart-controls h3 {
  margin: 0 0 8px 0;
  font-size: 14px;
}

.cart-checkboxes {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.cart-checkbox {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-size: 12px;
  padding: 3px;
  transition: all 0.2s ease;
}

.cart-checkbox:hover {
  opacity: 0.8;
}

.cart-checkbox input[type="checkbox"] {
  width: 14px;
  height: 14px;
  cursor: pointer;
}

.charts-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  grid-template-rows: 1fr 1fr;
  gap: 10px;
  width: 100%;
  min-height: 750px;
  flex: 1;
}

/* Responsive adjustments */
@media (max-width: 1400px) {
  .charts-grid {
    grid-template-columns: 1fr;
    grid-template-rows: repeat(4, 1fr);
    min-height: 1500px;
  }
}

@media (max-width: 1200px) {
  .content-layout {
    flex-direction: column;
    gap: 15px;
  }
  
  .cart-controls {
    width: 100%;
  }
  
  .cart-checkboxes {
    flex-direction: row;
    flex-wrap: wrap;
    gap: 15px;
  }
}

@media (max-width: 900px) {
  .charts-container {
    padding: 15px;
  }
  
  .charts-grid {
    gap: 15px;
    min-height: 1200px;
  }
  
  .cart-controls {
    padding: 10px;
  }
}

@media (max-width: 600px) {
  .charts-container {
    padding: 10px;
  }
  
  .charts-grid {
    gap: 10px;
    min-height: 1000px;
  }
  
  .cart-controls h3 {
    font-size: 14px;
  }
}
</style>