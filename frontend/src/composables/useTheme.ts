import { ref, computed } from 'vue'

export type Theme = 'light' | 'dark'

const currentTheme = ref<Theme>('light')

export function useTheme() {
  const theme = computed(() => currentTheme.value)
  
  const toggleTheme = () => {
    currentTheme.value = currentTheme.value === 'light' ? 'dark' : 'light'
  }
  
  const setTheme = (newTheme: Theme) => {
    currentTheme.value = newTheme
  }
  
  // Theme configurations
  const themes = {
    light: {
      // App container
      appBackground: '#ffffff',
      appColor: '#333333',
      
      // Header
      headerBackground: '#f8f9fa',
      headerBorder: '#dee2e6',
      titleColor: '#212529',
      
      // Tabs
      tabBackground: '#f8f9fa',
      tabBorder: '#ced4da',
      tabColor: '#495057',
      tabHoverBackground: '#e9ecef',
      tabActiveBackground: '#dee2e6',
      tabActiveColor: '#212529',
      
      // Sidebar
      sidebarBackground: '#ffffff',
      sidebarBorder: '#dee2e6',
      
      // Content sections
      contentBackground: '#ffffff',
      contentBorder: '#dee2e6',
      
      // Control panel
      panelBackground: '#ffffff',
      panelColor: '#333333',
      panelTitleColor: '#212529',
      panelTitleBorder: '#dee2e6',
      sectionBackground: '#f8f9fa',
      sectionBorder: '#dee2e6',
      sectionTitleColor: '#212529',
      labelColor: '#495057',
      inputBackground: '#ffffff',
      inputBorder: '#ced4da',
      inputColor: '#495057',
      inputFocusBorder: '#80bdff',
      buttonBackground: '#f8f9fa',
      buttonBorder: '#ced4da',
      buttonColor: '#495057',
      buttonHoverBackground: '#e9ecef',
      buttonDisabledBackground: '#e9ecef',
      buttonDisabledColor: '#6c757d',
      checkboxTextColor: '#212529',
      statusItemBackground: '#ffffff',
      statusItemBorder: '#dee2e6',
      statusLabelColor: '#495057',
      
      // Charts
      chartsBackground: '#ffffff',
      chartControlBackground: '#f8f9fa',
      chartControlBorder: '#dee2e6',
      chartControlTitleColor: '#212529',
      chartControlTextColor: '#495057',
      chartControlHoverBackground: '#e9ecef',
      chartBackground: '#ffffff',
      chartBorder: '#dee2e6',
      chartHoverBorder: '#adb5bd',
      chartTitleColor: '#212529',
      chartTooltipBackground: '#f8f9fa',
      chartTooltipBorder: '#dee2e6',
      chartTooltipColor: '#212529',
      chartLegendColor: '#495057',
      chartAxisColor: '#dee2e6',
      chartAxisLabelColor: '#495057',
      chartSplitLineColor: '#e9ecef',
      
      // Canvas
      canvasBackground: '#ffffff',
      canvasBorder: '#dee2e6',
      canvasTextColor: '#000000',
      canvasLegendColor: '#000000',
      setpointColor: '#000000',
      
      // Scrollbar
      scrollbarTrack: '#f1f1f1',
      scrollbarThumb: '#c1c1c1',
      scrollbarThumbHover: '#a8a8a8'
    },
    dark: {
      // App container
      appBackground: '#222',
      appColor: '#e0e0e0',
      
      // Header
      headerBackground: '#2a2a2a',
      headerBorder: '#444',
      titleColor: '#fff',
      
      // Tabs
      tabBackground: '#333',
      tabBorder: '#444',
      tabColor: '#ccc',
      tabHoverBackground: '#444',
      tabActiveBackground: '#555',
      tabActiveColor: '#fff',
      
      // Sidebar
      sidebarBackground: '#222',
      sidebarBorder: '#444',
      
      // Content sections
      contentBackground: '#222',
      contentBorder: '#444',
      
      // Control panel
      panelBackground: '#222',
      panelColor: '#e0e0e0',
      panelTitleColor: '#fff',
      panelTitleBorder: '#444',
      sectionBackground: '#2a2a2a',
      sectionBorder: '#444',
      sectionTitleColor: '#fff',
      labelColor: '#ccc',
      inputBackground: '#333',
      inputBorder: '#555',
      inputColor: '#fff',
      inputFocusBorder: '#666',
      buttonBackground: '#444',
      buttonBorder: '#555',
      buttonColor: '#fff',
      buttonHoverBackground: '#555',
      buttonDisabledBackground: '#333',
      buttonDisabledColor: '#888',
      checkboxTextColor: '#fff',
      statusItemBackground: '#333',
      statusItemBorder: '#444',
      statusLabelColor: '#ccc',
      
      // Charts
      chartsBackground: '#222',
      chartControlBackground: '#2a2a2a',
      chartControlBorder: '#444',
      chartControlTitleColor: '#fff',
      chartControlTextColor: '#ccc',
      chartControlHoverBackground: '#333',
      chartBackground: '#2a2a2a',
      chartBorder: '#444',
      chartHoverBorder: '#555',
      chartTitleColor: '#fff',
      chartTooltipBackground: '#444',
      chartTooltipBorder: '#666',
      chartTooltipColor: '#fff',
      chartLegendColor: '#ccc',
      chartAxisColor: '#666',
      chartAxisLabelColor: '#ccc',
      chartSplitLineColor: '#444',
      
      // Canvas
      canvasBackground: '#1a1a1a',
      canvasBorder: '#555',
      canvasTextColor: '#ffffff',
      canvasLegendColor: '#ffffff',
      setpointColor: '#ffffff',
      
      // Scrollbar
      scrollbarTrack: '#2a2a2a',
      scrollbarThumb: '#555',
      scrollbarThumbHover: '#666'
    }
  }
  
  const currentThemeConfig = computed(() => themes[theme.value])
  
  return {
    theme,
    toggleTheme,
    setTheme,
    currentThemeConfig
  }
}
