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
  state: 'Idle' | 'Busy' | 'Requesting' | 'Moving' | 'Avoiding' | 'Stopping';
  
  // Trajectory phase transitions (timestamps when trajectory phases change)
  trajectoryTransitions: readonly string[];
  
  // Trajectory phase information (phase labels corresponding to transitions)
  trajectoryPhases: readonly string[];
  
  // Performance metrics
  metrics: MessageMetricsReport;
};

export type MessageMetricsReport = {
  averageRoundTripTime: number; // in nanoseconds
  averageGoalToMovementTime: number; // in nanoseconds
  totalMessageCount: number;
  scenarioMessageCount: number;
  roundTripTimeCount: number;
  goalToMovementCount: number;
};

export type AllCartsData = {
  carts: SocketData[];
  timestamp: string;
};

export type TestResult = {
  name: string;
  status: 'running' | 'passed' | 'failed';
  duration: number;
  output: string;
  error?: string;
};

export type TestSuite = {
  name: string;
  status: 'running' | 'completed';
  tests: TestResult[];
  started: string;
  finished: string;
};

const connection = ref<WebSocket | null>(null)
const isConnected = ref(false)
const latestData = ref<AllCartsData | null>(null)
const cartDataMap = reactive<Map<number, SocketData>>(new Map())
const historicalData = ref<AllCartsData[]>([])
const currentTestSuite = ref<TestSuite | null>(null)
const testCallbacks = ref<((suite: TestSuite) => void)[]>([])

// Scenario-related reactive state
interface CoordinationScenario {
  name: string;
  description: string;
  status: string;
}

interface ScenarioResult {
  scenario: string;
  status: string;
  error?: string;
}

const scenarios = ref<CoordinationScenario[]>([])
const lastScenarioResult = ref<ScenarioResult | null>(null)
const scenarioCallbacks = ref<((message: any) => void)[]>([])

// Signal for clearing charts when scenarios start
const clearChartsSignal = ref(0)

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

function handleTestMessage(messageData: any) {
  switch (messageData.type) {
    case 'test_update':
    case 'test_status':
      currentTestSuite.value = messageData.data
      testCallbacks.value.forEach(cb => cb(messageData.data))
      break
    case 'test_error':
      console.error('Test error:', messageData.data.error)
      break
  }
}

function handleScenarioMessage(messageData: any) {
  switch (messageData.type) {
    case 'scenario_list':
      scenarios.value = messageData.data || []
      break
    case 'scenario_result':
      const result = messageData.data
      lastScenarioResult.value = {
        scenario: result.scenario,
        status: result.status,
        error: result.error?.toString()
      }
      
      // Update scenario status in the list
      const scenarioIndex = scenarios.value.findIndex(s => s.name === result.scenario)
      if (scenarioIndex >= 0) {
        scenarios.value[scenarioIndex].status = result.status
      }
      break
  }
  
  // Notify all scenario callbacks
  scenarioCallbacks.value.forEach(cb => cb(messageData))
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
    const messageData = JSON.parse(event.data)
    
    // Check if it's a test message
    if (messageData.type && (messageData.type.includes('test') || messageData.type.includes('results'))) {
      handleTestMessage(messageData)
      return
    }
    
    // Check if it's a scenario message
    if (messageData.type && (messageData.type.includes('scenario'))) {
      handleScenarioMessage(messageData)
      return
    }
    
    // Otherwise, treat as cart data
    const allCartsData: AllCartsData = messageData
    latestData.value = allCartsData
    
    // Clear existing cart data and update with only current active carts
    cartDataMap.clear()
    
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

  // Emergency stop for a specific cart
  const emergencyStop = (cartId: number) => {
    sendControlMessage('emergencyStop', cartId - 1) // Convert to 0-based index
  }

  // Toggle random goals
  const toggleRandomGoals = (enabled: boolean) => {
    sendControlMessage('randomGoals', undefined, undefined, enabled)
  }

  // Start coordination tests
  const startTests = () => {
    const message = {
      type: 'start_tests'
    }
    sendMessage(JSON.stringify(message))
  }

  // Get current test status
  const getTestStatus = () => {
    const message = {
      type: 'get_test_status'
    }
    sendMessage(JSON.stringify(message))
  }

  // Register a callback for test updates
  const onTestUpdate = (callback: (suite: TestSuite) => void) => {
    testCallbacks.value.push(callback)
    
    // Return cleanup function
    return () => {
      const index = testCallbacks.value.indexOf(callback)
      if (index > -1) {
        testCallbacks.value.splice(index, 1)
      }
    }
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

  // Scenario management functions
  function sendScenarioMessage(message: any) {
    sendMessage(JSON.stringify(message))
  }
  
  function listScenarios() {
    sendScenarioMessage({ type: 'list_scenarios' })
  }
  
  function runScenario(scenarioName: string) {
    // Clear charts when starting a new scenario
    clearChartsSignal.value++
    sendScenarioMessage({ type: 'run_scenario', scenario: scenarioName })
  }
  
  function getScenarioStatus() {
    sendScenarioMessage({ type: 'scenario_status' })
  }

  function onScenarioUpdate(callback: (message: any) => void) {
    scenarioCallbacks.value.push(callback)
    
    return () => {
      const index = scenarioCallbacks.value.indexOf(callback)
      if (index > -1) {
        scenarioCallbacks.value.splice(index, 1)
      }
    }
  }

  return {
    // State
    isConnected: readonly(isConnected),
    latestData: readonly(latestData),
    cartDataMap: readonly(cartDataMap),
    historicalData: readonly(historicalData),
    currentTestSuite: readonly(currentTestSuite),
    scenarios: readonly(scenarios),
    lastScenarioResult: readonly(lastScenarioResult),
    clearChartsSignal: readonly(clearChartsSignal),
    
    // Actions
    setGoal,
    emergencyStop,
    toggleRandomGoals,
    getCartData,
    onCartData,
    fetchHistoricalData,
    startTests,
    getTestStatus,
    onTestUpdate,
    
    // Scenario actions
    sendScenarioMessage,
    listScenarios,
    runScenario,
    getScenarioStatus,
    onScenarioUpdate,
    
    // Raw connection for advanced use
    connection: readonly(connection)
  }
}