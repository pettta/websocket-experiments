import { defineStore } from 'pinia'
export const useWsStore = defineStore('ws', {
  state: () => ({
    wsUrl: 'ws://localhost:8080/ws',
    ws: null as WebSocket | null, 
    wsConnection: 'Disconnected',
    reconnectTimeout: null as number | null, 
    pingInterval: null as number | null,
        subscribed: [] as string[],
  }),
  actions: {
  init() {
    this.ws = new WebSocket(this.wsUrl)
    this.ws.onopen = () => {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            console.log("Connected to WS")
            this.wsConnection = 'Connected'
            this.ping() 
            this.subscribed.forEach((channel) => {
                this.ws?.send(JSON.stringify({ type: channel }))
            })
        }
    }
    this.ws.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data) // TODO IMPLEMENT LOGIC FOR THIS 
        } catch (error) {
            console.error('Error parsing WebSocket message:', error)
        }
    }
    this.ws.onclose = () => {
        console.log("Disconnected from WS")
        this.ws = null
        this.wsConnection = 'Disconnected'
        if (!this.reconnectTimeout) {
            this.reconnectTimeout = window.setTimeout(() => {
                this.init()
                this.reconnectTimeout = null
            }, 5000)
        }
    }
  },
  ping() {
    if (this.pingInterval) window.clearInterval(this.pingInterval)
    this.pingInterval = window.setInterval(() => {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ type: 'ping' }))
        }
    }, 10000)
  },
  close(){
    if (this.ws) {
        this.ws.close()
    }
    if (this.reconnectTimeout) {
        clearTimeout(this.reconnectTimeout)
        this.reconnectTimeout = null
    }
    if (this.pingInterval) {
        window.clearInterval(this.pingInterval)
        this.pingInterval = null
    }
  },
  subscribe(channel: string) {
    const normalized = channel.trim()
    if (!normalized) {
        return
    }
    if (!this.subscribed.includes(normalized)) {
        this.subscribed = [...this.subscribed, normalized]
    }
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: normalized }))
    }
    },
}})