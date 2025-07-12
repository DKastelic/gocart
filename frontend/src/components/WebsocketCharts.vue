<template>
    <div>
        <div style="display: flex; gap: 16px; margin-top: 16px;">
            <div
                v-for="col in numColumns"
                :key="col"
                style="flex: 1; display: flex; flex-direction: column; gap: 16px;"
            >
                <WebsocketChart v-for="type in chartTypes" :title="type" :cart_id="col" :value_type="type" />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import WebsocketChart from './WebsocketChart.vue';
const numColumns = ref(2);
const chartTypes = ['position', 'velocity', 'acceleration', 'jerk'] as const;
</script>