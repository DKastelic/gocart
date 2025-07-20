<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { connectWebSocket, registerCallback, type SocketData } from '@/state';

const props = defineProps({
    cart_id: Number,
});

const leftBorder = ref(0);
const rightBorder = ref(0);
const goal = ref(0);
const state = ref<'Idle' | 'Moving' | 'Avoiding'>('Idle');
const position = ref(0);
const velocity = ref(0);
const acceleration = ref(0);
const jerk = ref(0);

function updateData(data: SocketData) {
    if (data.id !== props.cart_id) return;
    leftBorder.value = data.leftBorder;
    rightBorder.value = data.rightBorder;
    goal.value = data.goal;
    state.value = data.state as 'Idle' | 'Moving' | 'Avoiding';
    position.value = data.position;
    velocity.value = data.velocity;
    acceleration.value = data.acceleration;
    jerk.value = data.jerk;
}

onMounted(() => {
  connectWebSocket();

  registerCallback((data) => {
    updateData(data);
  });
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
    <p>Velocity: {{ velocity.toFixed(2) }}</p>
    <p>Acceleration: {{ acceleration.toFixed(2) }}</p>
    <p>Jerk: {{ jerk.toFixed(2) }}</p>
    </div>
  </div>
</template>