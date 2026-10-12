<template>
  <div class="login-container">
    <div class="login-card">
      <h1 class="title">NRLLink</h1>
      <p class="subtitle">无线电网络互联系统</p>

      <!-- 模式一：密码登录 -->
      <div v-if="step === 'password'">
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

        <div style="margin-top: 16px;">
          <n-button secondary block size="large" :loading="passkeyLoading" @click="handlePasskeyLogin">
            使用通行密钥 (Passkey) 登录
          </n-button>
        </div>
      </div>

      <!-- 模式二：两步验证动态码输入 -->
      <div v-else-if="step === 'totp'">
        <p style="color: var(--text-secondary); margin-bottom: 16px; font-size: 14px; text-align: center;">
          该账号已开启两步验证，请输入身份验证器（Authenticator）上的 6 位动态验证码。
        </p>
        <n-input
          v-model:value="totpCode"
          placeholder="6 位动态验证码"
          size="large"
          maxlength="6"
          style="margin-bottom: 16px; text-align: center; font-size: 18px; letter-spacing: 4px;"
          @keyup.enter="handleTOTPLogin"
        />
        <n-button type="primary" block size="large" :loading="loading" @click="handleTOTPLogin">
          验证并登录
        </n-button>
        <n-button text block style="margin-top: 12px;" @click="step = 'password'">
          返回密码登录
        </n-button>
      </div>

      <p v-if="errorMsg" class="error">{{ errorMsg }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { NForm, NFormItem, NInput, NButton } from 'naive-ui'
import { transformAssertionOptions, serializeAssertionResponse } from '../utils/webauthn'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref()
const loading = ref(false)
const passkeyLoading = ref(false)
const errorMsg = ref('')

const step = ref<'password' | 'totp'>('password')
const totpTicket = ref('')
const totpCode = ref('')

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
    if (res.require_2fa) {
      totpTicket.value = res.totp_ticket
      step.value = 'totp'
      totpCode.value = ''
      return
    }
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

async function handleTOTPLogin() {
  if (!totpCode.value || totpCode.value.length < 6) {
    errorMsg.value = '请输入完整的 6 位验证码'
    return
  }
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await userStore.loginTOTP(totpTicket.value, totpCode.value)
    if (res.must_change_pwd) {
      router.push('/force-password')
    } else {
      router.push('/')
    }
  } catch (e: any) {
    errorMsg.value = e.message || '两步验证失败'
  } finally {
    loading.value = false
  }
}

async function handlePasskeyLogin() {
  if (!window.PublicKeyCredential) {
    errorMsg.value = '当前浏览器不支持通行密钥 (WebAuthn)'
    return
  }
  passkeyLoading.value = true
  errorMsg.value = ''
  try {
    // 1. 发起登录流程获取挑战
    const beginRes = await fetch(`/api/v1/webauthn/login/begin?username=${encodeURIComponent(form.username)}`, {
      method: 'POST'
    })
    const beginJson = await beginRes.json()
    if (beginJson.code !== 20000) {
      throw new Error(beginJson.message || '发起通行密钥认证失败')
    }

    const { session_id, options } = beginJson.data
    const credOptions = transformAssertionOptions(options)

    // 2. 唤起系统通行密钥（Touch ID, Face ID, Windows Hello, YubiKey 等）
    const assertion = await navigator.credentials.get(credOptions) as PublicKeyCredential
    if (!assertion) {
      throw new Error('未获取到有效凭据')
    }

    const payload = serializeAssertionResponse(assertion)

    // 3. 提交断言完成登录
    const finishRes = await fetch(`/api/v1/webauthn/login/finish?session_id=${session_id}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    const finishJson = await finishRes.json()
    if (finishJson.code !== 20000) {
      throw new Error(finishJson.message || '通行密钥校验失败')
    }

    userStore.setToken(finishJson.data.token)
    if (finishJson.data.must_change_pwd) {
      router.push('/force-password')
    } else {
      router.push('/')
    }
  } catch (e: any) {
    if (e.name === 'NotAllowedError') {
      errorMsg.value = '已取消通行密钥验证'
    } else {
      errorMsg.value = e.message || '通行密钥登录失败'
    }
  } finally {
    passkeyLoading.value = false
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
