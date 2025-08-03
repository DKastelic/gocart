<script setup lang="ts">
import { ref } from 'vue';
import WebsocketCharts from './components/WebsocketCharts.vue';
import CartVisualization from './components/CartVisualization.vue';
import ControlPanel from './components/ControlPanel.vue';
import { useTheme } from './composables/useTheme';

const activeTab = ref<'visualization' | 'charts'>('visualization');
const { theme, toggleTheme, currentThemeConfig } = useTheme();

function setActiveTab(tab: 'visualization' | 'charts') {
  activeTab.value = tab;
}
</script>

<template>
  <div class="app-container" :style="{ 
    backgroundColor: currentThemeConfig.appBackground, 
    color: currentThemeConfig.appColor 
  }">
    <!-- Header -->
    <header class="app-header" :style="{ 
      backgroundColor: currentThemeConfig.headerBackground, 
      borderBottomColor: currentThemeConfig.headerBorder 
    }">
      <div class="header-content">
        <h1 class="app-title" :style="{ color: currentThemeConfig.titleColor }">Dashboard</h1>
        
        <!-- Theme Toggle Button -->
        <button 
          @click="toggleTheme" 
          class="theme-toggle"
          :style="{ 
            backgroundColor: currentThemeConfig.buttonBackground,
            borderColor: currentThemeConfig.buttonBorder,
            color: currentThemeConfig.buttonColor
          }"
          :title="`Switch to ${theme === 'light' ? 'dark' : 'light'} theme`"
        >
          {{ theme === 'light' ? 'dark theme' : 'light theme' }}
        </button>
      </div>
      
      <!-- Tab Navigation -->
      <nav class="tab-navigation">
        <button 
          class="tab-button" 
          :class="{ active: activeTab === 'visualization' }"
          @click="setActiveTab('visualization')"
          :style="{ 
            backgroundColor: activeTab === 'visualization' ? currentThemeConfig.tabActiveBackground : currentThemeConfig.tabBackground,
            borderColor: currentThemeConfig.tabBorder,
            color: activeTab === 'visualization' ? currentThemeConfig.tabActiveColor : currentThemeConfig.tabColor
          }"
        >
          Visualization
        </button>
        <button 
          class="tab-button" 
          :class="{ active: activeTab === 'charts' }"
          @click="setActiveTab('charts')"
          :style="{ 
            backgroundColor: activeTab === 'charts' ? currentThemeConfig.tabActiveBackground : currentThemeConfig.tabBackground,
            borderColor: currentThemeConfig.tabBorder,
            color: activeTab === 'charts' ? currentThemeConfig.tabActiveColor : currentThemeConfig.tabColor
          }"
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
          <section 
            v-if="activeTab === 'visualization'" 
            class="content-section"
            :style="{ 
              backgroundColor: currentThemeConfig.contentBackground, 
              borderColor: currentThemeConfig.contentBorder 
            }"
          >
            <CartVisualization />
          </section>

          <!-- Charts Tab -->
          <section 
            v-if="activeTab === 'charts'" 
            class="content-section"
            :style="{ 
              backgroundColor: currentThemeConfig.contentBackground, 
              borderColor: currentThemeConfig.contentBorder 
            }"
          >
            <WebsocketCharts />
          </section>
        </div>

        <!-- Control Panel Sidebar -->
        <div 
          class="sidebar"
          :style="{ 
            backgroundColor: currentThemeConfig.sidebarBackground, 
            borderLeftColor: currentThemeConfig.sidebarBorder 
          }"
        >
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
  margin: 0;
  padding: 0;
  font-family: Arial, sans-serif;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.app-header {
  padding: 15px;
  border-bottom: 1px solid;
  flex-shrink: 0;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.app-title {
  font-size: 18px;
  margin: 0;
  text-align: center;
  flex: 1;
}

.theme-toggle {
  padding: 6px 12px;
  border: 1px solid;
  cursor: pointer;
  min-width: 40px;
}

.theme-toggle:hover {
  opacity: 0.8;
  transform: scale(1.05);
}

.tab-navigation {
  display: flex;
  justify-content: center;
  gap: 5px;
}

.tab-button {
  padding: 6px 12px;
  border: 1px solid;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-button:hover {
  opacity: 0.8;
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
  border-left: 1px solid;
  overflow-y: auto;
  flex-shrink: 0;
}

.content-section {
  border: 1px solid;
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

/* Global theme-aware scrollbar styling */
* ::-webkit-scrollbar {
  width: 12px;
}

* ::-webkit-scrollbar-track {
  background: v-bind('currentThemeConfig.scrollbarTrack');
}

* ::-webkit-scrollbar-thumb {
  background: v-bind('currentThemeConfig.scrollbarThumb');
  border-radius: 6px;
}

* ::-webkit-scrollbar-thumb:hover {
  background: v-bind('currentThemeConfig.scrollbarThumbHover');
}

/* Firefox scrollbar styling */
* {
  scrollbar-width: thin;
  scrollbar-color: v-bind('currentThemeConfig.scrollbarThumb') v-bind('currentThemeConfig.scrollbarTrack');
}
</style>

