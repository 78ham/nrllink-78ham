import { defineStore } from 'pinia'
import { ref } from 'vue'

const API = '/api/v1'

interface User {
  id: number
  callsign: string
  name: string
  phone: string
  roles: string[]
  avatar: string
  status: number
}

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref<User | null>(null)

  async function login(username: string, password: string) {
    const res = await fetch(`${API}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const json = await res.json()
    if (json.code !== 20000) throw new Error(json.message || '登录失败')
    token.value = json.data.token
    localStorage.setItem('token', token.value)
    return json.data
  }

  async function fetchUser() {
    if (!token.value) return null
    const res = await fetch(`${API}/auth/me`, {
      headers: { 'x-token': token.value }
    })
    if (res.status === 401) {
      logout()
      return null
    }
    const json = await res.json()
    if (json.code === 20000) {
      user.value = json.data
      return json.data
    }
    return null
  }

  async function changePassword(password: string) {
    if (!token.value) throw new Error('未登录')
    const res = await fetch(`${API}/user/password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'x-token': token.value },
      body: JSON.stringify({ password })
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const json = await res.json()
    if (json.code !== 20000) throw new Error(json.data?.message || '修改失败')
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }

  return { token, user, login, fetchUser, changePassword, logout }
})
