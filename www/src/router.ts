import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'Login', component: () => import('./views/Login.vue') },
  {
    path: '/',
    component: () => import('./views/Layout.vue'),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('./views/Dashboard.vue') },
      { path: 'devices', name: 'Devices', component: () => import('./views/DeviceList.vue') },
      { path: 'groups', name: 'Groups', component: () => import('./views/GroupList.vue') },
      { path: 'voice', name: 'VoiceChat', component: () => import('./views/VoiceChat.vue') },
      { path: 'settings', name: 'Settings', component: () => import('./views/Settings.vue') },
    ]
  },
  { path: '/force-password', name: 'ForcePassword', component: () => import('./views/ForceChangePwd.vue') }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

router.beforeEach((to, _from, next) => {
  if (to.matched.some(r => r.meta.requiresAuth)) {
    const token = localStorage.getItem('token')
    if (!token) {
      return next({ name: 'Login', query: { redirect: to.fullPath } })
    }
  }
  next()
})

export default router
