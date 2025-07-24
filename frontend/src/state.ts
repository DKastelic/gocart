import { ref, reactive, onUnmounted, readonly } from 'vue';

export type SocketData = {
  id: number;
  
  // Chart data (planned trajectory values from MovementPlanner)
  chartPosition: number;
  chartVelocity: number;
  chartAcceleration: number;
  chartJerk: number;
  
  // Real-time data (actual cart physics values)
  position: number;
  timestamp: string;

  leftBorder: number;
  rightBorder: number;
  goal: number;
  setpoint: number;
  state: 'Idle' | 'Busy' | 'Requesting' | 'Moving' | 'Avoiding';
};

export type AllCartsData = {
  carts: SocketData[];
  timestamp: string;
};

const connection = ref<WebSocket | null>(null)
const isConnected = ref(false)
const latestData = ref<AllCartsData | null>(null)
const cartDataMap = reactive<Map<number, SocketData>>(new Map())
const historicalData = ref<AllCartsData[]>([])

async function fetchHistoricalData() {
  try {
    const response = await fetch('http://localhost:8080/api/historical-data')
    if (response.ok) {
      const data: AllCartsData[] = await response.json()
      historicalData.value = data
      console.log(`Loaded ${data.length} historical data points`)
      
      // Process historical data for charts
      data.forEach(point => {
        point.carts.forEach(cartData => {
          const cartDataWithTimestamp = {
            ...cartData,
            timestamp: point.timestamp
          }
          // Call all registered callbacks for historical data
          callbacks.value.forEach(cb => cb(cartDataWithTimestamp))
        })
      })
    } else {
      console.warn('Failed to fetch historical data:', response.statusText)
    }
  } catch (error) {
    console.error('Error fetching historical data:', error)
  }
}

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
  
  connection.value.onopen = () => {
    isConnected.value = true
    // Fetch historical data when connection opens
    fetchHistoricalData()
  }
  
  connection.value.onmessage = (event: MessageEvent) => {
      const allCartsData: AllCartsData = JSON.parse(event.data)
      latestData.value = allCartsData
      
      // Update cart data map and add timestamp to individual cart data
      allCartsData.carts.forEach(cartData => {
        const cartDataWithTimestamp = {
          ...cartData,
          timestamp: allCartsData.timestamp
        }
        cartDataMap.set(cartData.id, cartDataWithTimestamp)
        // Call all registered callbacks for each cart with timestamp added
        callbacks.value.forEach(cb => cb(cartDataWithTimestamp))
      })
  }

  connection.value.onclose = () => {
      isConnected.value = false
      // Attempt to reconnect after a delay
      setTimeout(() => connectWebSocket(url), 1000)
  }
}

// Composable for WebSocket functionality
export function useWebSocket() {
  // Auto-connect if not already connected
  if (!connection.value) {
    connectWebSocket()
  }

  // Send control messages
  const sendControlMessage = (command: string, controller?: number, position?: number, enabled?: boolean) => {
    const message = {
      command,
      ...(controller !== undefined && { controller }),
      ...(position !== undefined && { position }),
      ...(enabled !== undefined && { enabled })
    }
    sendMessage(JSON.stringify(message))
  }

  // Set goal for a specific cart
  const setGoal = (cartId: number, position: number) => {
    sendControlMessage('setGoal', cartId - 1, position) // Convert to 0-based index
  }

  // Toggle random goals
  const toggleRandomGoals = (enabled: boolean) => {
    sendControlMessage('randomGoals', undefined, undefined, enabled)
  }

  // Get data for a specific cart
  const getCartData = (cartId: number) => {
    return cartDataMap.get(cartId) || null
  }

  // Register a callback for cart data updates
  const onCartData = (callback: (data: SocketData) => void) => {
    callbacks.value.push(callback)
    
    // Return cleanup function
    return () => {
      const index = callbacks.value.indexOf(callback)
      if (index > -1) {
        callbacks.value.splice(index, 1)
      }
    }
  }

  return {
    // State
    isConnected: readonly(isConnected),
    latestData: readonly(latestData),
    cartDataMap: readonly(cartDataMap),
    historicalData: readonly(historicalData),
    
    // Actions
    setGoal,
    toggleRandomGoals,
    getCartData,
    onCartData,
    fetchHistoricalData,
    
    // Raw connection for advanced use
    connection: readonly(connection)
  }
}