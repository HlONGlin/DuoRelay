import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import './assets/styles/main.css'
import { initializeTheme } from './stores/appearance'
import { loadVariant } from './stores/variant'

initializeTheme()

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { message: { placement: 'bottom-right', offset: 24 } })

void loadVariant().then(() => app.mount('#app'))
