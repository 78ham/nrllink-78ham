<template>
  <div class="login-container">
    <div class="login-card">
      <h1 class="title">NRLLink</h1>
      <p class="subtitle">无线电网络互联系统</p>
      <n-form ref="formRef" :model="form" :rules="rules">
        <n-form-item path="username">
          <n-input v-model:value="form.username" placeholder="呼号 / 手机号" size="large" />
        </n-form-item>
        <n-form-item path="password">
          <n-input v-model:value="form.password" type="password" placeholder="密码" size="large" @keyup.enter="handleLogin" />
        </n-form-item>
        <n-button type="primary" block size="large" :loading="loading" @click="handleLogin">
          登录
        </n-button>
      </n-form>
      <p v-if="errorMsg" class="error">{{ errorMsg }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { NForm, NFormItem, NInput, NButton } from 'naive-ui'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref()
const loading = ref(false)
const errorMsg = ref('')

const form = reactive({ username: '', password: '' })
const rules = {
  username: { required: true, message: '请输入呼号', trigger: 'blur' },
  password: { required: true, message: '请输入密码', trigger: 'blur' }
}

async function handleLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await userStore.login(form.username, form.password)
    if (res.must_change_pwd) {
      router.push('/force-password')
    } else {
      router.push('/')
    }
  } catch (e: any) {
    errorMsg.value = e.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex; align-items: center; justify-content: center;
  height: 100vh; background: var(--bg-primary);
}
.login-card {
  width: 360px; padding: 40px;
  background: var(--bg-card); border-radius: 12px;
  border: 1px solid var(--border);
}
.title { text-align: center; color: var(--accent); margin: 0 0 4px; font-size: 28px; }
.subtitle { text-align: center; color: var(--text-secondary); margin: 0 0 32px; font-size: 14px; }
.error { color: var(--offline); text-align: center; margin-top: 16px; font-size: 13px; }
</style>
