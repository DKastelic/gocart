import { ref } from 'vue';

export type SocketData = {
    id: number;
    position: number;
    velocity: number;
    acceleration: number;
    jerk: number;
};

const connection = ref<WebSocket | null>(null)

function sendMessage(message: string) {
  if (connection.value) {
    connection.value.send(message)
  }
}

const callbacks = ref<Array<(data: SocketData) => void>>([])

export function registerCallback(callback: (data: SocketData) => void) {
    // if the connection is not established, try to establish it
    if (!connection.value || connection.value.readyState !== WebSocket.OPEN) {
      connection.value = new WebSocket('ws://localhost:8080/ws')
      connection.value.onmessage = (event: MessageEvent) => {
          const val: SocketData = JSON.parse(event.data)
          // Call all registered callbacks
          callbacks.value.forEach(cb => cb(val))
      }
    }

    callbacks.value.push(callback)
}