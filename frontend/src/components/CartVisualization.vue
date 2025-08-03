<template>
  <div 
    class="cart-visualization"
    :style="{ backgroundColor: currentThemeConfig.canvasBackground }"
  >
    <canvas 
      ref="canvas" 
      :width="SCREEN_WIDTH" 
      :height="SCREEN_HEIGHT"
      class="cart-canvas"
      :style="{ 
        backgroundColor: currentThemeConfig.canvasBackground,
        borderColor: currentThemeConfig.canvasBorder
      }"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { useWebSocket, type SocketData } from '../state';
import { useTheme } from '../composables/useTheme';

const SCREEN_WIDTH = 1600;
const SCREEN_HEIGHT = 900;

const canvas = ref<HTMLCanvasElement | null>(null);
const { currentThemeConfig } = useTheme();
let ctx: CanvasRenderingContext2D | null = null;
let animationId: number | null = null;

// Cart data storage
const carts = ref<Map<number, SocketData>>(new Map());

const { onCartData } = useWebSocket();
let cleanup: (() => void) | null = null;

// Approximate cart dimensions (these should ideally come from the backend)
const CART_WIDTH = 50;
const CART_HEIGHT = 40;

// State colors matching the Go implementation
const stateColors = {
  'Idle': '#0000FF',        // Blue
  'Busy': '#9932CC',        // Purple
  'Requesting': '#FFA500',  // Orange
  'Moving': '#00FF00',      // Green
  'Avoiding': '#FF0000'     // Red
} as const;

// Register websocket callback to receive cart data
const handleCartData = (data: SocketData) => {
  carts.value.set(data.id, data);
};

// Drawing functions
function clearCanvas() {
  if (!ctx) return;
  ctx.fillStyle = currentThemeConfig.value.canvasBackground;
  ctx.fillRect(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT);
}

function drawCart(cartData: SocketData) {
  if (!ctx) return;
  
  // Calculate cart position
  const x = cartData.position - CART_WIDTH / 2;
  const y = SCREEN_HEIGHT / 2 - CART_HEIGHT / 2;
  
  // Set cart color based on state
  const color = stateColors[cartData.state] || stateColors['Idle'];
  ctx.fillStyle = color;
  ctx.fillRect(x, y, CART_WIDTH, CART_HEIGHT);
  
  // Draw cart ID on top
  ctx.fillStyle = currentThemeConfig.value.canvasTextColor;
  ctx.font = '12px Arial';
  ctx.textAlign = 'center';
  ctx.fillText(cartData.id.toString(), cartData.position, y - 5);
}

function drawGoal(cartData: SocketData) {
  if (!ctx) return;
  
  const goalRadius = 8;
  const x = cartData.goal;
  const y = SCREEN_HEIGHT / 2 - CART_HEIGHT / 4;
  
  ctx.fillStyle = '#FF0000'; // Red
  ctx.beginPath();
  ctx.arc(x, y, goalRadius, 0, 2 * Math.PI);
  ctx.fill();
}

function drawSetpoint(cartData: SocketData) {
  if (!ctx) return;
  
  const setpointRadius = 6;
  const x = cartData.setpoint;
  const y = SCREEN_HEIGHT / 2 + CART_HEIGHT / 4;
  
  ctx.fillStyle = currentThemeConfig.value.setpointColor;
  ctx.beginPath();
  ctx.arc(x, y, setpointRadius, 0, 2 * Math.PI);
  ctx.fill();
}

function drawBounds(cartData: SocketData) {
  if (!ctx) return;
  
  // Left bound
  ctx.fillStyle = '#00FF00'; // Green
  ctx.fillRect(cartData.leftBorder, 0, 2, SCREEN_HEIGHT);
  
  // Right bound
  ctx.fillRect(cartData.rightBorder - 2, 0, 2, SCREEN_HEIGHT);
}

function drawFrame() {
  clearCanvas();
  
  // Draw all carts and their associated elements
  carts.value.forEach((cartData) => {
    drawBounds(cartData);
    drawCart(cartData);
    drawGoal(cartData);
    drawSetpoint(cartData);
  });
  
  // Draw legend
  drawLegend();
}

function drawLegend() {
  if (!ctx) return;
  
  const legendX = 20;
  const legendY = 20;
  const legendItemHeight = 25;
  
  ctx.font = '14px Arial';
  ctx.textAlign = 'left';
  
  // Title
  ctx.fillStyle = currentThemeConfig.value.canvasLegendColor;
  ctx.fillText('Cart States:', legendX, legendY);
  
  // Legend items
  const states = [
    { name: 'Idle', color: stateColors.Idle },
    { name: 'Busy', color: stateColors.Busy },
    { name: 'Requesting', color: stateColors.Requesting },
    { name: 'Moving', color: stateColors.Moving },
    { name: 'Avoiding', color: stateColors.Avoiding }
  ];
  
  states.forEach((state, index) => {
    if (!ctx) return;
    
    const y = legendY + 20 + (index * legendItemHeight);
    
    // Color box (make it a circle)
    ctx.fillStyle = state.color;
    ctx.beginPath();
    ctx.arc(legendX + 7, y - 4, 7, 0, 2 * Math.PI);
    ctx.fill();
    
    // Text
    ctx.fillStyle = currentThemeConfig.value.canvasLegendColor;
    ctx.fillText(state.name, legendX + 20, y);
  });
  
  // Add indicators section
  const indicatorsY = legendY + 20 + (states.length * legendItemHeight) + 20;
  ctx.fillStyle = currentThemeConfig.value.canvasLegendColor;
  ctx.fillText('Indicators:', legendX, indicatorsY);
  
  const indicators = [
    { name: 'Goal', color: '#FF0000' },
    { name: 'Setpoint', color: currentThemeConfig.value.setpointColor },
    { name: 'Bounds', color: '#00FF00' }
  ];
  
  indicators.forEach((indicator, index) => {
    if (!ctx) return;
    
    const y = indicatorsY + 20 + (index * legendItemHeight);
    
    // Color box (make it a circle)
    ctx.fillStyle = indicator.color;
    ctx.beginPath();
    ctx.arc(legendX + 7, y - 4, 7, 0, 2 * Math.PI);
    ctx.fill();
    
    // Text
    ctx.fillStyle = currentThemeConfig.value.canvasLegendColor;
    ctx.fillText(indicator.name, legendX + 20, y);
  });
}

function startAnimation() {
  function animate() {
    drawFrame();
    animationId = requestAnimationFrame(animate);
  }
  animate();
}

function stopAnimation() {
  if (animationId) {
    cancelAnimationFrame(animationId);
    animationId = null;
  }
}

onMounted(() => {
  if (canvas.value) {
    ctx = canvas.value.getContext('2d');
    if (ctx) {
      // Register for cart data updates
      cleanup = onCartData(handleCartData);
      
      // Start animation loop
      startAnimation();
    }
  }
});

onUnmounted(() => {
  stopAnimation();
  if (cleanup) {
    cleanup();
  }
});
</script>

<style scoped>
.cart-visualization {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
  flex-direction: column;
}

.cart-canvas {
  border: 2px solid;
  max-width: 100%;
  height: auto;
}

@media (max-width: 1700px) {
  .cart-canvas {
    width: 90vw;
    height: auto;
  }
}

@media (max-width: 900px) {
  .cart-canvas {
    width: 95vw;
    height: auto;
  }
}

/* Theme-aware scrollbar styling */
.cart-visualization ::-webkit-scrollbar {
  width: 12px;
}

.cart-visualization ::-webkit-scrollbar-track {
  background: v-bind('currentThemeConfig.scrollbarTrack');
}

.cart-visualization ::-webkit-scrollbar-thumb {
  background: v-bind('currentThemeConfig.scrollbarThumb');
  border-radius: 6px;
}

.cart-visualization ::-webkit-scrollbar-thumb:hover {
  background: v-bind('currentThemeConfig.scrollbarThumbHover');
}

/* Firefox scrollbar styling */
.cart-visualization {
  scrollbar-width: thin;
  scrollbar-color: v-bind('currentThemeConfig.scrollbarThumb') v-bind('currentThemeConfig.scrollbarTrack');
}
</style>
