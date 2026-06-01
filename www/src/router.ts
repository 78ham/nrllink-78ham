import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'Login', component: () => import('./views/Login.vue') },
  {
    path: '/',
    component: () => import('./views/Layout.vue'),
    redirect: '/dashboard',
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

export default createRouter({
  history: createWebHashHistory(),
  routes
})
