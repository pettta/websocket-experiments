<template>
  <header>
    <h1>Producer-Consumer Example App</h1> 
  </header>

  <main>
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
      wsConnection: 'Disconnected'
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
  }
})


</script>



