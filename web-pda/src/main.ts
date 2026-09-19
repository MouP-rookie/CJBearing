import { createApp } from 'vue'
import { createPinia } from 'pinia'
import Vant from 'vant'
import 'vant/lib/index.css'
import App from './App.vue'
import router from './router'
import './style.css'
createApp(App).use(createPinia()).use(router).use(Vant).mount('#app')
