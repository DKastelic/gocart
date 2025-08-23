<template>
  <div 
    class="scenario-panel"
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
      Testni scenariji
    </h3>
    
    <!-- Scenario Control -->
    <div 
      class="control-section"
      :style="{ 
        backgroundColor: currentThemeConfig.sectionBackground,
        borderColor: currentThemeConfig.sectionBorder
      }"
    >

      <!-- Scenario List -->
      <div v-if="scenarios && scenarios.length > 0" class="scenarios-list">
        <div 
          v-for="scenario in scenarios" 
          :key="scenario.name"
          class="scenario-item"
          :style="{ backgroundColor: currentThemeConfig.headerBackground }"
        >
          <div class="scenario-header">
            <div class="scenario-info">
              <span class="scenario-name">{{ formatScenarioName(scenario.name) }}</span>
              <!-- <span class="scenario-description">{{ scenario.description }}</span> -->
            </div>
            <div class="scenario-controls">
              <button 
                @click="runScenario(scenario.name)"
                :disabled="!isConnected || scenario.status === 'running'"
                class="scenario-button"
                :style="{ 
                  backgroundColor: !isConnected || scenario.status === 'running' ? 
                    currentThemeConfig.buttonDisabledBackground : currentThemeConfig.buttonBackground,
                  borderColor: currentThemeConfig.buttonBorder,
                  color: !isConnected || scenario.status === 'running' ? 
                    currentThemeConfig.buttonDisabledColor : currentThemeConfig.buttonColor
                }"
              >
                Poženi
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Loading State -->
      <div v-else-if="isConnected" class="loading-state">
        <span>Scenariji niso naloženi...</span>
      </div>

      <!-- Refresh Button -->
      <div class="control-actions">
        <button 
          @click="refreshScenarios"
          :disabled="!isConnected"
          class="refresh-button"
          :style="{ 
            backgroundColor: !isConnected ? 
              currentThemeConfig.buttonDisabledBackground : currentThemeConfig.inputBackground,
            borderColor: currentThemeConfig.buttonBorder,
            color: !isConnected ? 
              currentThemeConfig.buttonDisabledColor : currentThemeConfig.buttonColor
          }"
        >
          Osveži
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue';
import { useWebSocket } from '@/state';
import { useTheme } from '@/composables/useTheme';

const { currentThemeConfig } = useTheme();
const { 
  isConnected,
  scenarios,
  runScenario: runScenarioAction,
  listScenarios: listScenariosAction,
  onScenarioUpdate
} = useWebSocket();

// Methods
function runScenario(scenarioName: string) {
  if (!isConnected.value) return;
  
  console.log('Running scenario:', scenarioName);
  runScenarioAction(scenarioName);
}

function refreshScenarios() {
  if (!isConnected.value) return;
  
  console.log('Refreshing scenarios');
  listScenariosAction();
}

function formatScenarioName(name: string): string {
  return name
    .replace(/_/g, ' ')
    .replace(/\b\w/g, l => l.toUpperCase());
}

// Lifecycle
onMounted(() => {
  // Set up scenario update listener
  const unsubscribe = onScenarioUpdate((data: any) => {
    console.log('Received scenario update:', data);
  });
  
  // Request initial scenario list when connected
  if (isConnected.value) {
    refreshScenarios();
  }
  
  // Cleanup on unmount
  onUnmounted(() => {
    unsubscribe();
  });
});
</script>

<style scoped>
.scenario-panel {
  border: none;
  padding: 15px;
  font-family: Arial, sans-serif;
  margin: 0;
}

.panel-title {
  margin: 0 0 15px 0;
  padding: 0 0 10px 0;
  border-bottom: 1px solid;
  font-size: 16px;
  font-weight: 600;
}

.control-section {
  border: 1px solid;
  padding: 15px;
  margin-bottom: 15px;
}

.scenario-controls {
  display: flex;
  flex-direction: row;
  gap: 8px;
}

.scenario-button {
  border: 1px solid;
  padding: 4px 8px;
  cursor: pointer;
  font-size: 12px;
  transition: opacity 0.2s ease;
}

.refresh-button {
  border: 1px solid;
  padding: 4px 8px;
  cursor: pointer;
  font-size: 12px;
  transition: opacity 0.2s ease;
}

.scenario-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.scenario-button:not(:disabled):hover {
  opacity: 0.8;
}

.scenario-button.primary {
  font-weight: 600;
}

.scenario-button.secondary {
  font-size: 12px;
  padding: 6px 10px;
}

.scenario-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.section-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.scenario-status-compact {
  display: flex;
  align-items: center;
}

.status-badge {
  padding: 3px 6px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: bold;
  border: 1px solid;
}

.status-badge.small {
  padding: 2px 4px;
  font-size: 12px;
  border-radius: 8px;
}

.scenario-summary-compact {
  margin-bottom: 12px;
}

.summary-stats-compact {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  align-items: center;
}

.stat {
  font-weight: 600;
}

.stat.passed { color: #4CAF50; }
.stat.failed { color: #F44336; }
.stat.running { color: #FF9800; }

.stat-total {
  color: #666;
  font-size: 11px;
  margin-left: auto;
}

.scenario-list-toggle {
  margin-bottom: 10px;
}

.toggle-button {
  padding: 6px 10px;
  border: 1px solid;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  width: 100%;
}

.scenarios-list-detailed {
  max-height: 300px;
  overflow-y: auto;
}

.scenario-item-compact {
  border: 1px solid #ddd;
  border-radius: 4px;
  margin-bottom: 6px;
  padding: 8px;
}

.scenario-item-header-compact {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.scenario-name-compact {
  font-weight: 500;
  font-size: 12px;
  flex: 1;
  margin-right: 8px;
  line-height: 1.3;
}

.scenario-details-compact {
  margin-top: 6px;
  display: flex;
  align-items: flex-start;
  gap: 6px;
}

.toggle-details-compact {
  padding: 2px 6px;
  border: 1px solid;
  border-radius: 3px;
  cursor: pointer;
  font-size: 11px;
  font-weight: bold;
  min-width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.scenario-output-compact {
  background: #f8f9fa;
  border-radius: 3px;
  padding: 6px;
  max-height: 150px;
  overflow-y: auto;
  flex: 1;
}

.scenario-output-compact h6 {
  margin: 0 0 4px 0;
  font-size: 10px;
  text-transform: uppercase;
  opacity: 0.7;
  font-weight: 600;
}

.scenario-output-compact pre {
  margin: 0;
  font-size: 10px;
  line-height: 1.3;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.scenario-error-compact {
  margin-bottom: 8px;
}

.scenario-error-compact pre {
  color: #d32f2f;
}

.scenario-log-compact pre {
  color: #333;
}
</style>
