<template>
  <div 
    class="control-panel"
    :style="{ 
      backgroundColor: currentThemeConfig.panelBackground,
      color: currentThemeConfig.panelColor
    }"
  >
    <h3 
      class="panel-title"
      :style="{ 
        color: currentThemeConfig.panelTitleColor,
        borderBottomColor: currentThemeConfig.panelTitleBorder
      }"
    >
      Nadzorna plošča simulacije
    </h3>
    
    <!-- Cart Goal Controls -->
    <div 
      class="control-section"
      :style="{ 
        backgroundColor: currentThemeConfig.sectionBackground,
        borderColor: currentThemeConfig.sectionBorder
      }"
    >
      <h4 
        class="section-title"
        :style="{ color: currentThemeConfig.sectionTitleColor }"
      >
        Cilji
      </h4>
      <div v-for="cart in activeCarts" :key="cart.id" class="cart-control">
        <label 
          class="cart-label"
          :style="{ color: currentThemeConfig.labelColor }"
        >
          Agent {{ cart.id }}:
        </label>
        <input 
          type="number" 
          v-model="cartGoals[cart.id]" 
          :placeholder="`Cilj za agenta ${cart.id}`"
          step="10"
          class="goal-input"
          :style="{ 
            backgroundColor: currentThemeConfig.inputBackground,
            borderColor: currentThemeConfig.inputBorder,
            color: currentThemeConfig.inputColor
          }"
        />
        <button 
          @click="setGoal(cart.id, cartGoals[cart.id])" 
          class="set-button"
          :disabled="!cartGoals[cart.id]"
          :style="{ 
            backgroundColor: !cartGoals[cart.id] ? currentThemeConfig.buttonDisabledBackground : currentThemeConfig.buttonBackground,
            borderColor: currentThemeConfig.buttonBorder,
            color: !cartGoals[cart.id] ? currentThemeConfig.buttonDisabledColor : currentThemeConfig.buttonColor
          }"
        >
          Nastavi cilj
        </button>
        <button 
          @click="emergencyStop(cart.id)" 
          class="emergency-button"
          :style="{ 
            backgroundColor: '#dc3545',
            borderColor: '#dc3545',
            color: '#ffffff'
          }"
        >
          Ustavi
        </button>
      </div>
    </div>

    <!-- Random Goals Control -->
    <div 
      class="control-section"
      :style="{ 
        backgroundColor: currentThemeConfig.sectionBackground,
        borderColor: currentThemeConfig.sectionBorder
      }"
    >
      <h4 
        class="section-title"
        :style="{ color: currentThemeConfig.sectionTitleColor }"
      >
        Naključni cilji
      </h4>
      <div class="random-control">
        <label class="checkbox-label">
          <input 
            type="checkbox" 
            v-model="randomGoalsEnabled" 
            @change="toggleRandomGoals"
            class="checkbox-input"
          />
          <span 
            class="checkbox-text"
            :style="{ color: currentThemeConfig.checkboxTextColor }"
          >
            Omogoči generiranje naključnih ciljev
          </span>
        </label>
      </div>
    </div>

    <!-- Status -->
    <div 
      class="control-section"
      :style="{ 
        backgroundColor: currentThemeConfig.sectionBackground,
        borderColor: currentThemeConfig.sectionBorder
      }"
    >
      <h4 
        class="section-title"
        :style="{ color: currentThemeConfig.sectionTitleColor }"
      >
        Status
      </h4>
      <div 
        class="status-item"
        :style="{ 
          backgroundColor: currentThemeConfig.statusItemBackground,
          borderColor: currentThemeConfig.statusItemBorder
        }"
      >
        <span 
          class="status-label"
          :style="{ color: currentThemeConfig.statusLabelColor }"
        >
          Dostopnost simulacije:
        </span>
        <span :class="['status-value', isConnected ? 'connected' : 'disconnected']">
          {{ isConnected ? 'povezana' : 'odklopljena' }}
        </span>
      </div>
      <div 
        class="status-item"
        :style="{ 
          backgroundColor: currentThemeConfig.statusItemBackground,
          borderColor: currentThemeConfig.statusItemBorder
        }"
      >
        <span 
          class="status-label"
          :style="{ color: currentThemeConfig.statusLabelColor }"
        >
          Naključni cilji:
        </span>
        <span :class="['status-value', randomGoalsEnabled ? 'enabled' : 'disabled']">
          {{ randomGoalsEnabled ? 'omogočeni' : 'onemogočeni' }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue';
import { useWebSocket } from '@/state';
import { useTheme } from '@/composables/useTheme';

const { currentThemeConfig } = useTheme();

// Get cart data from WebSocket state
const { cartDataMap, setGoal: sendSetGoal, emergencyStop: sendEmergencyStop, toggleRandomGoals: sendToggleRandomGoals, isConnected } = useWebSocket();

// Reactive goals for each cart
const cartGoals = reactive<Record<number, number | null>>({});

// Computed property for active carts
const activeCarts = computed(() => {
  return Array.from(cartDataMap.values()).sort((a, b) => a.id - b.id);
});

const randomGoalsEnabled = ref(false);

function setGoal(cartId: number, goal: number | null) {
  if (goal !== null) {
    sendSetGoal(cartId, goal); // Cart IDs are already 1-based
    console.log(`Set goal for cart ${cartId}: ${goal}`);
  }
}

function emergencyStop(cartId: number) {
  sendEmergencyStop(cartId); // Cart IDs are already 1-based
  console.log(`Emergency stop for cart ${cartId}`);
}

function emergencyStopAll() {
  // Stop all active carts
  activeCarts.value.forEach(cart => {
    sendEmergencyStop(cart.id);
  });
  console.log('Emergency stop for all carts');
}

function toggleRandomGoals() {
  sendToggleRandomGoals(randomGoalsEnabled.value);
  console.log(`Random goals ${randomGoalsEnabled.value ? 'enabled' : 'disabled'}`);
}
</script>

<style scoped>
.control-panel {
  border: none;
  padding: 15px;
  font-family: Arial, sans-serif;
  margin: 0;
}

.panel-title {
  margin: 0 0 15px 0;
  border-bottom: 1px solid;
  padding-bottom: 8px;
  font-size: 16px;
}

.control-section {
  margin-bottom: 15px;
  padding: 10px;
  border: 1px solid;
}

.section-title {
  margin: 0 0 10px 0;
  font-size: 14px;
}

.cart-control {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  padding: 5px;
}

.cart-label {
  min-width: 50px;
  font-size: 13px;
}

.goal-input {
  border: 1px solid;
  padding: 4px 6px;
  font-size: 13px;
  width: 80px;
}

.goal-input::-webkit-outer-spin-button,
.goal-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.goal-input:focus {
  outline: 1px solid #80bdff;
}

.set-button {
  border: 1px solid;
  padding: 4px 8px;
  cursor: pointer;
  font-size: 12px;
  transition: opacity 0.2s ease;
}

.set-button:hover:not(:disabled) {
  opacity: 0.8;
}

.set-button:disabled {
  cursor: not-allowed;
}

.emergency-button {
  border: 1px solid;
  padding: 4px 8px;
  cursor: pointer;
  font-size: 12px;
  transition: opacity 0.2s ease;
  margin-left: 5px;
}

.emergency-button:hover {
  opacity: 0.8;
}

.emergency-control {
  padding: 8px;
  text-align: center;
}

.emergency-all-button {
  border: 2px solid;
  padding: 10px 20px;
  cursor: pointer;
  font-size: 14px;
  font-weight: bold;
  transition: opacity 0.2s ease;
  width: 100%;
}

.emergency-all-button:hover {
  opacity: 0.8;
}

.random-control {
  padding: 8px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  margin-bottom: 5px;
}

.checkbox-input {
  width: 14px;
  height: 14px;
}

.checkbox-text {
  font-size: 13px;
}

.random-description {
  margin: 0;
  font-size: 11px;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px;
  margin-bottom: 5px;
  border: 1px solid;
}

.status-label {
  font-size: 12px;
}

.status-value {
  font-size: 12px;
  padding: 2px 4px;
}

.status-value.connected {
  color: #90EE90;
}

.status-value.disconnected {
  color: #FF6B6B;
}

.status-value.enabled {
  color: #90EE90;
}

.status-value.disabled {
  color: #FFB366;
}

/* Theme-aware scrollbar styling for control panel */
.control-panel ::-webkit-scrollbar {
  width: 8px;
}

.control-panel ::-webkit-scrollbar-track {
  background: v-bind('currentThemeConfig.scrollbarTrack');
}

.control-panel ::-webkit-scrollbar-thumb {
  background: v-bind('currentThemeConfig.scrollbarThumb');
  border-radius: 4px;
}

.control-panel ::-webkit-scrollbar-thumb:hover {
  background: v-bind('currentThemeConfig.scrollbarThumbHover');
}

/* Firefox scrollbar styling */
.control-panel {
  scrollbar-width: thin;
  scrollbar-color: v-bind('currentThemeConfig.scrollbarThumb') v-bind('currentThemeConfig.scrollbarTrack');
}
</style>
