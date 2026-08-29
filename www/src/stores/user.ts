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
  must_change_pwd?: boolean
  default_admin?: boolean
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
      body: JSON.stringify({ id: user.value?.id ?? 0, password })
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const json = await res.json()
    if (json.code !== 20000) throw new Error(json.data?.message || '修改失败')
  }

  async function fetchUsers() {
    if (!token.value) throw new Error('未登录')
    const res = await fetch(`${API}/users`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'x-token': token.value },
      body: '{}'
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const json = await res.json()
    if (json.code !== 20000) throw new Error(json.message || '获取用户列表失败')
    return json.data
  }

  async function createUser(payload: any) {
    if (!token.value) throw new Error('未登录')
    const res = await fetch(`${API}/user/create`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'x-token': token.value },
      body: JSON.stringify(payload)
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const json = await res.json()
    if (json.code !== 20000) throw new Error(json.message || '新增用户失败')
    if (json.data?.isok === 1) throw new Error(json.data?.message || '新增用户失败')
    return json.data
  }

  async function deleteUser(id: number) {
    if (!token.value) throw new Error('未登录')
    const res = await fetch(`${API}/user/delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'x-token': token.value },
      body: JSON.stringify({ id })
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const json = await res.json()
    if (json.code !== 20000) throw new Error(json.message || '删除用户失败')
    if (json.data?.isok === 1) throw new Error(json.data?.message || '删除用户失败')
    return json.data
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
  }

  return { token, user, login, fetchUser, changePassword, fetchUsers, createUser, deleteUser, logout }
})
