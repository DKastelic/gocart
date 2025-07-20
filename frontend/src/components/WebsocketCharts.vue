<template>
    <div class="charts-container">
        <div class="content-layout">
            <div class="cart-controls">
                <h3>Cart Visibility</h3>
                <div class="cart-checkboxes">
                    <label v-for="cartId in numColumns" :key="cartId" class="cart-checkbox">
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
                    :title="`${type.charAt(0).toUpperCase() + type.slice(1)} - All Carts`" 
                    :value_type="type" 
                    :data="socketData" 
                    :num_carts="numColumns"
                    :cart_visibility="cartVisibility"
                />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref, reactive } from 'vue';
import CombinedChart from './CombinedChart.vue';
import { connectWebSocket, registerCallback, type SocketData } from '@/state';

const numColumns = ref(2);
const chartTypes = ['position', 'velocity', 'acceleration', 'jerk'] as const;
const socketData = ref<SocketData | null>(null);

// Initialize cart visibility - only Cart 1 enabled by default
const cartVisibility = reactive<Record<number, boolean>>({});
for (let i = 1; i <= numColumns.value; i++) {
  cartVisibility[i] = i === 1; // Only Cart 1 is enabled by default
}

function updateVisibility() {
  // This function will be called when checkboxes change
  // The reactive cartVisibility object will automatically trigger updates
}

onMounted(() => {
  connectWebSocket();

  registerCallback((data: SocketData) => {
    socketData.value = data;
  })
})
</script>

<style scoped>
.charts-container {
  width: 100%;
  padding: 0;
  background-color: #222;
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
  background: #2a2a2a;
  border: 1px solid #444;
  padding: 10px;
  width: 150px;
  flex-shrink: 0;
  height: fit-content;
}

.cart-controls h3 {
  margin: 0 0 8px 0;
  color: #fff;
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
  color: #ccc;
  padding: 3px;
}

.cart-checkbox:hover {
  background: #333;
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