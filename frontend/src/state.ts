import { ref } from 'vue';

export type SocketData = {
  id: number;
  position: number;
  velocity: number;
  acceleration: number;
  jerk: number;
  timestamp: string;

  leftBorder: number;
  rightBorder: number;
  goal: number;
  setpoint: number;
  state: 'Idle' | 'Processing' | 'Requesting' | 'Moving' | 'Avoiding';
};

const connection = ref<WebSocket | null>(null)

function sendMessage(message: string) {
  if (connection.value) {
    connection.value.send(message)
  }
}

const callbacks = ref<Array<(data: SocketData) => void>>([])

export function registerCallback(callback: (data: SocketData) => void) {
  callbacks.value.push(callback)
}

export function connectWebSocket(url: string = 'ws://localhost:8080/ws') {
  connection.value = new WebSocket(url)
  connection.value.onmessage = (event: MessageEvent) => {
      const val: SocketData = JSON.parse(event.data)
      // Call all registered callbacks
      callbacks.value.forEach(cb => cb(val))
  }

  connection.value.onclose = () => {
      // Attempt to reconnect after a delay
      setTimeout(() => connectWebSocket(url), 1000)
  }
}