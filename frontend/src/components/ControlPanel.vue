<template>
  <div class="control-panel">
    <h3 class="panel-title">Simulation Controls</h3>
    
    <!-- Cart Goal Controls -->
    <div class="control-section">
      <h4 class="section-title">Cart Goals</h4>
      <div v-for="(cart, index) in carts" :key="index" class="cart-control">
        <label class="cart-label">Cart {{ index + 1 }}:</label>
        <input 
          type="number" 
          v-model="cart.goal" 
          :placeholder="`Goal for cart ${index + 1}`"
          step="10"
          class="goal-input"
        />
        <button 
          @click="setGoal(index, cart.goal)" 
          class="set-button"
          :disabled="!cart.goal"
        >
          Set Goal
        </button>
      </div>
    </div>

    <!-- Random Goals Control -->
    <div class="control-section">
      <h4 class="section-title">Random Goals</h4>
      <div class="random-control">
        <label class="checkbox-label">
          <input 
            type="checkbox" 
            v-model="randomGoalsEnabled" 
            @change="toggleRandomGoals"
            class="checkbox-input"
          />
          <span class="checkbox-text">Enable Random Goal Generation</span>
        </label>
      </div>
    </div>

    <!-- Status -->
    <div class="control-section">
      <h4 class="section-title">Status</h4>
      <div class="status-item">
        <span class="status-label">Connection:</span>
        <span :class="['status-value', isConnected ? 'connected' : 'disconnected']">
          {{ isConnected ? 'connected' : 'disconnected' }}
        </span>
      </div>
      <div class="status-item">
        <span class="status-label">Random Goals:</span>
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

interface Cart {
  goal: number | null;
}

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
  background: #222;
  border: none;
  padding: 15px;
  color: #e0e0e0;
  font-family: Arial, sans-serif;
  margin: 0;
  height: 100%;
}

.panel-title {
  margin: 0 0 15px 0;
  color: #fff;
  border-bottom: 1px solid #444;
  padding-bottom: 8px;
  font-size: 16px;
}

.control-section {
  margin-bottom: 15px;
  padding: 10px;
  background: #2a2a2a;
  border: 1px solid #444;
}

.section-title {
  margin: 0 0 10px 0;
  color: #fff;
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
  color: #ccc;
  font-size: 13px;
}

.goal-input {
  background: #333;
  border: 1px solid #555;
  padding: 4px 6px;
  color: #fff;
  font-size: 13px;
  width: 80px;
}

.goal-input::-webkit-outer-spin-button,
.goal-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.goal-input:focus {
  outline: 1px solid #666;
}

.set-button {
  background: #444;
  border: 1px solid #555;
  color: #fff;
  padding: 4px 8px;
  cursor: pointer;
  font-size: 12px;
}

.set-button:hover:not(:disabled) {
  background: #555;
}

.set-button:disabled {
  background: #333;
  color: #888;
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
  color: #fff;
  font-size: 13px;
}

.random-description {
  margin: 0;
  color: #aaa;
  font-size: 11px;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px;
  margin-bottom: 5px;
  background: #333;
  border: 1px solid #444;
}

.status-label {
  color: #ccc;
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
</style>
