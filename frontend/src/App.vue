<script setup lang="ts">
import { ref } from 'vue';
import WebsocketCharts from './components/WebsocketCharts.vue';
import CartVisualization from './components/CartVisualization.vue';
import ControlPanel from './components/ControlPanel.vue';

const activeTab = ref<'visualization' | 'charts'>('visualization');

function setActiveTab(tab: 'visualization' | 'charts') {
  activeTab.value = tab;
}
</script>

<template>
  <div class="app-container">
    <!-- Header -->
    <header class="app-header">
      <h1 class="app-title">GoCart Control Dashboard</h1>
      
      <!-- Tab Navigation -->
      <nav class="tab-navigation">
        <button 
          class="tab-button" 
          :class="{ active: activeTab === 'visualization' }"
          @click="setActiveTab('visualization')"
        >
          Visualization
        </button>
        <button 
          class="tab-button" 
          :class="{ active: activeTab === 'charts' }"
          @click="setActiveTab('charts')"
        >
          Charts
        </button>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="app-main">
      <div class="main-layout">
        <!-- Content Area -->
        <div class="content-area">
          <!-- Cart Visualization Tab -->
          <section v-if="activeTab === 'visualization'" class="content-section">
            <CartVisualization />
          </section>

          <!-- Charts Tab -->
          <section v-if="activeTab === 'charts'" class="content-section">
            <WebsocketCharts />
          </section>
        </div>

        <!-- Control Panel Sidebar -->
        <div class="sidebar">
          <ControlPanel />
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.app-container {
  height: 100vh;
  width: 100vw;
  background: #222;
  color: #e0e0e0;
  margin: 0;
  padding: 0;
  font-family: Arial, sans-serif;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.app-header {
  background: #2a2a2a;
  padding: 15px;
  border-bottom: 1px solid #444;
  flex-shrink: 0;
}
.app-title {
  font-size: 18px;
  margin: 0 0 10px 0;
  color: #fff;
  text-align: center;
}

.tab-navigation {
  display: flex;
  justify-content: center;
  gap: 5px;
}

.tab-button {
  padding: 6px 12px;
  border: 1px solid #444;
  background: #333;
  font-size: 13px;
  cursor: pointer;
  color: #ccc;
}

.tab-button:hover {
  background: #444;
}

.tab-button.active {
  background: #555;
  color: #fff;
}

.app-main {
  flex: 1;
  overflow: hidden;
  display: flex;
}

.main-layout {
  display: flex;
  width: 100%;
  height: 100%;
}

.content-area {
  flex: 1;
  overflow: hidden;
}

.sidebar {
  width: 300px;
  background: #222;
  border-left: 1px solid #444;
  overflow-y: auto;
  flex-shrink: 0;
}

.content-section {
  background: #222;
  border: 1px solid #444;
  height: 100%;
  overflow: auto;
}

/* Responsive Design */
@media (max-width: 1200px) {
  .app-main {
    padding: 0 15px 20px 15px;
  }
  
  .app-title {
    font-size: 24px;
  }
  
  .tab-button {
    padding: 8px 15px;
    font-size: 13px;
  }
}

@media (max-width: 768px) {
  .app-header {
    padding: 15px 10px 10px 10px;
  }
  
  .app-title {
    font-size: 20px;
  }
  
  .tab-navigation {
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }
  
  .tab-button {
    width: 200px;
    justify-content: center;
  }
  
  .content-section {
    padding: 15px;
  }
}

@media (max-width: 480px) {
  .content-section {
    padding: 10px;
  }
  
  .app-title {
    font-size: 18px;
  }
  
  .tab-button {
    padding: 8px 12px;
    font-size: 12px;
    width: 150px;
  }
}
</style>

