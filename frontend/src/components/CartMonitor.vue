<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { useWebSocket, type SocketData } from '@/state';

const props = defineProps({
    cart_id: Number,
});

const leftBorder = ref(0);
const rightBorder = ref(0);
const goal = ref(0);
const state = ref<'Idle' | 'Busy' | 'Requesting' | 'Moving' | 'Avoiding' | 'Stopping'>('Idle');
const position = ref(0);

const { onCartData, isConnected } = useWebSocket();

function updateData(data: SocketData) {
    if (data.id !== props.cart_id) return;
    leftBorder.value = data.leftBorder;
    rightBorder.value = data.rightBorder;
    goal.value = data.goal;
    state.value = data.state as 'Idle' | 'Busy' | 'Requesting' | 'Moving' | 'Avoiding' | 'Stopping';
    position.value = data.position;
}

let cleanup: (() => void) | null = null;

onMounted(() => {
  cleanup = onCartData(updateData);
});

onUnmounted(() => {
  if (cleanup) {
    cleanup();
  }
});
</script>

<template>
  <div>
    <h2>Cart Monitor</h2>
    <div>
    <p>Left Border: {{ leftBorder.toFixed(2) }}</p>
    <p>Right Border: {{ rightBorder.toFixed(2) }}</p>
    <p>Goal: {{ goal.toFixed(2) }}</p>
    <p>State: {{ state }}</p>
    <p>Position: {{ position.toFixed(2) }}</p>
    </div>
  </div>
</template>