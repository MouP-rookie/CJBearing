/** 文件说明：PC 应用入口，安装 Pinia、路由和 Element Plus。 */

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import './style.css'
const app = createApp(App)
app.use(createPinia()).use(router).use(ElementPlus, { locale: zhCn })
app.mount('#app')
