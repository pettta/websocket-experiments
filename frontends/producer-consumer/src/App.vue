<template>
  <header>
    <h1>Producer-Consumer Example App</h1> 
  </header>

  <main>
    <div class="subscribe">
      <label class="subscribe__label" for="subscribe-input">Subscribe ID</label>
      <div class="subscribe__controls">
        <input
          id="subscribe-input"
          v-model="subscribeId"
          type="text"
          placeholder="Enter subscribe-id"
          autocomplete="off"
          class="subscribe__input"
        />
        <button class="subscribe__button" @click="handleSubscribe" :disabled="!canSubscribe">Subscribe</button>
      </div>
    </div>

    <TheWelcome />
  </main>
</template>


<script lang="ts">
import { defineComponent } from 'vue' 
import { useWsStore } from './stores/ws'


export default defineComponent({
  name: 'App',
  data() {
    return {
      wsConnection: 'Disconnected',
      subscribeId: ''
    }
  },
  computed: {
    canSubscribe(): boolean {
      return this.subscribeId.trim().length > 0
    }
  },
  setup() {
    return {
      wsStore: useWsStore()
    }
  },
  beforeMount() {
    this.wsStore.close() 
    this.wsStore.init() 
  },
  unmounted() {
    this.wsStore.close()
  },
  methods: {
    handleSubscribe() {
      const target = this.subscribeId.trim()
      if (!target) {
        return
      }
      this.wsStore.subscribe(target)
      this.subscribeId = ''
    }
  }
})


</script>

<style scoped>
.subscribe {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 2rem;
  max-width: 420px;
}

.subscribe__label {
  font-weight: 600;
}

.subscribe__controls {
  display: flex;
  gap: 0.5rem;
}

.subscribe__input {
  flex: 1;
  padding: 0.5rem 0.75rem;
  border: 1px solid #ced2d9;
  border-radius: 6px;
}

.subscribe__input:focus {
  outline: 2px solid #3b82f6;
  border-color: #3b82f6;
}

.subscribe__button {
  padding: 0.5rem 1.25rem;
  border: none;
  border-radius: 6px;
  background: #3b82f6;
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.subscribe__button:disabled {
  cursor: not-allowed;
  background: #9bb5f9;
}
</style>







