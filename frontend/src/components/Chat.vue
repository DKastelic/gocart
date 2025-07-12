<template>
  <div id="app">
    <div v-for="(message, index) in messages" :key="index">
      <p>{{ message }}</p>
    </div>
    <div>
      <input v-model="newMessage" @keyup.enter="sendMessage(newMessage)" placeholder="Type your message..." />
    </div>
    <button v-on:click="sendMessage(newMessage)">Send Message</button>
  </div>
</template>

<script setup lang="ts">
  import { ref, onMounted } from 'vue';

  const connection = ref<WebSocket | null>(null);
  const messages = ref<string[]>([]);
  const newMessage = ref<string>('');

  function sendMessage(message: string) {
    if (connection.value) {
      connection.value.send(message);
    }
  }

  onMounted(() => {
    console.log("Starting connection to WebSocket Server");
    connection.value = new WebSocket("ws://localhost:8080/ws");

    connection.value.onmessage = function(event: MessageEvent) {
      messages.value.push(event.data);
    };

    connection.value.onopen = function(event: Event) {
      console.log(event);
      console.log("Successfully connected to the echo websocket server...");
    };
  });
</script>