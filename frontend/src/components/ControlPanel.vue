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
      Simulation Controls
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
        Cart Goals
      </h4>
      <div v-for="(cart, index) in carts" :key="index" class="cart-control">
        <label 
          class="cart-label"
          :style="{ color: currentThemeConfig.labelColor }"
        >
          Cart {{ index + 1 }}:
        </label>
        <input 
          type="number" 
          v-model="cart.goal" 
          :placeholder="`Goal for cart ${index + 1}`"
          step="10"
          class="goal-input"
          :style="{ 
            backgroundColor: currentThemeConfig.inputBackground,
            borderColor: currentThemeConfig.inputBorder,
            color: currentThemeConfig.inputColor
          }"
        />
        <button 
          @click="setGoal(index, cart.goal)" 
          class="set-button"
          :disabled="!cart.goal"
          :style="{ 
            backgroundColor: !cart.goal ? currentThemeConfig.buttonDisabledBackground : currentThemeConfig.buttonBackground,
            borderColor: currentThemeConfig.buttonBorder,
            color: !cart.goal ? currentThemeConfig.buttonDisabledColor : currentThemeConfig.buttonColor
          }"
        >
          Set Goal
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
        Random Goals
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
            Enable Random Goal Generation
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
          Connection:
        </span>
        <span :class="['status-value', isConnected ? 'connected' : 'disconnected']">
          {{ isConnected ? 'connected' : 'disconnected' }}
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
          Random Goals:
        </span>
        <span :class="['status-value', randomGoalsEnabled ? 'enabled' : 'disabled']">
          {{ randomGoalsEnabled ? 'Enabled' : 'Disabled' }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { useWebSocket } from '@/state';
import { useTheme } from '@/composables/useTheme';

interface Cart {
  goal: number | null;
}

const { currentThemeConfig } = useTheme();

const carts = reactive<Cart[]>([
  { goal: null },
  { goal: null },
  { goal: null },
  { goal: null }
]);

const randomGoalsEnabled = ref(false);

const { setGoal: sendSetGoal, toggleRandomGoals: sendToggleRandomGoals, isConnected } = useWebSocket();

function setGoal(cartIndex: number, goal: number | null) {
  if (goal !== null) {
    sendSetGoal(cartIndex + 1, goal); // Convert to 1-based index
    console.log(`Set goal for cart ${cartIndex + 1}: ${goal}`);
  }
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
  height: 100%;
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
