<template>
  <div class="page">
    <h2>系统设置</h2>
    <n-card title="个人信息" style="margin-bottom:16px;">
      <n-descriptions>
        <n-descriptions-item label="呼号">{{ userStore.user?.callsign }}</n-descriptions-item>
        <n-descriptions-item label="手机">{{ userStore.user?.phone }}</n-descriptions-item>
        <n-descriptions-item label="角色">{{ userStore.user?.roles?.join(', ') }}</n-descriptions-item>
      </n-descriptions>
    </n-card>

    <!-- 两步验证 (TOTP) -->
    <n-card title="两步验证 (TOTP 2FA)" style="margin-bottom:16px;">
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <div>
          <p style="margin: 0 0 8px 0; font-weight: 500;">
            状态：
            <n-tag :type="userStore.user?.totp_enabled === 1 ? 'success' : 'default'">
              {{ userStore.user?.totp_enabled === 1 ? '已启用' : '未启用' }}
            </n-tag>
          </p>
          <p style="color: var(--text-secondary); margin: 0; font-size: 13px;">
            启用后，登录时除密码外还需输入身份验证器（如 Google Authenticator、Microsoft Authenticator 等）生成的 6 位动态验证码。
          </p>
        </div>
        <div>
          <n-button v-if="userStore.user?.totp_enabled !== 1" type="primary" @click="handleOpenTOTPSetup">
            绑定身份验证器
          </n-button>
          <n-button v-else type="error" @click="showDisableTOTPModal = true">
            关闭两步验证
          </n-button>
        </div>
      </div>
    </n-card>

    <!-- 通行密钥 (Passkey) -->
    <n-card title="通行密钥 (Passkey) 免密登录" style="margin-bottom:16px;">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;">
        <p style="color: var(--text-secondary); margin: 0; font-size: 13px;">
          使用本设备的生物识别（指纹 / 面容）或硬件安全密钥（YubiKey）直接登录，无需输入密码。
        </p>
        <n-button type="primary" :loading="passkeyRegistering" @click="handleRegisterPasskey">
          绑定新通行密钥
        </n-button>
      </div>

      <n-data-table
        :columns="passkeyColumns"
        :data="passkeys"
        :loading="passkeyLoading"
        :empty-description="'尚未绑定通行密钥'"
      />
    </n-card>

    <!-- 数据维护与灾备（仅管理员） -->
    <n-card v-if="isAdmin" title="数据维护与灾备" style="margin-bottom:16px;">
      <p style="color: var(--text-secondary); margin-bottom: 16px;">
        生成当前系统数据库的一致性全量快照并下载。支持在线热备份，包含所有用户账号、设备参数、频道房间及系统配置。
      </p>
      <n-button type="primary" :loading="downloading" @click="handleDownloadBackup">
        下载数据库快照 (.sqlite3)
      </n-button>
    </n-card>

    <!-- 绑定 TOTP 弹窗 -->
    <n-modal v-model:show="showTOTPModal" preset="card" title="绑定两步验证身份验证器" style="width: 440px;" :mask-closable="false">
      <div style="text-align: center;">
        <p style="color: var(--text-secondary); font-size: 13px; margin-bottom: 12px;">
          请使用手机身份验证器应用扫描下方二维码：
        </p>
        <div v-if="totpQR" style="margin-bottom: 12px;">
          <img :src="totpQR" alt="TOTP QR" style="width: 180px; height: 180px; border-radius: 8px; border: 1px solid var(--border);" />
        </div>
        <p style="font-size: 12px; color: var(--text-secondary); margin-bottom: 16px; word-break: break-all;">
          若无法扫码，可手动添加密钥：<strong style="color: var(--accent);">{{ totpSecret }}</strong>
        </p>
        <n-input
          v-model:value="verifyCode"
          placeholder="请输入验证器中的 6 位动态码"
          maxlength="6"
          size="large"
          style="text-align: center; letter-spacing: 4px; font-size: 16px; margin-bottom: 16px;"
        />
        <div style="display: flex; justify-content: flex-end; gap: 8px;">
          <n-button @click="showTOTPModal = false">取消</n-button>
          <n-button type="primary" :loading="verifyingTOTP" @click="handleConfirmTOTP">
            确认并激活
          </n-button>
        </div>
      </div>
    </n-modal>

    <!-- 关闭 TOTP 弹窗 -->
    <n-modal v-model:show="showDisableTOTPModal" preset="card" title="关闭两步验证" style="width: 400px;" :mask-closable="false">
      <p style="color: var(--text-secondary); font-size: 13px; margin-bottom: 12px;">
        为确保安全，请输入当前登录密码以关闭两步验证：
      </p>
      <n-input
        v-model:value="disablePassword"
        type="password"
        placeholder="请输入登录密码"
        size="large"
        style="margin-bottom: 16px;"
      />
      <div style="display: flex; justify-content: flex-end; gap: 8px;">
        <n-button @click="showDisableTOTPModal = false">取消</n-button>
        <n-button type="error" :loading="disablingTOTP" @click="handleConfirmDisableTOTP">
          确认关闭
        </n-button>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NButton,
  NTag,
  NModal,
  NInput,
  NDataTable,
  NPopconfirm,
  useMessage
} from 'naive-ui'
import { useUserStore } from '../stores/user'
import {
  transformCreationOptions,
  serializeCreationResponse
} from '../utils/webauthn'

const userStore = useUserStore()
const message = useMessage()
const downloading = ref(false)

// TOTP 状态
const showTOTPModal = ref(false)
const showDisableTOTPModal = ref(false)
const totpQR = ref('')
const totpSecret = ref('')
const verifyCode = ref('')
const verifyingTOTP = ref(false)
const disablePassword = ref('')
const disablingTOTP = ref(false)

// Passkey 状态
const passkeys = ref<any[]>([])
const passkeyLoading = ref(false)
const passkeyRegistering = ref(false)

const isAdmin = computed(() => {
  return userStore.user?.roles?.includes('admin') || userStore.user?.roles?.includes('master')
})

const passkeyColumns = [
  { title: '密钥名称', key: 'name' },
  { title: '创建时间', key: 'create_time' },
  { title: '最近使用', key: 'last_used_time' },
  {
    title: '操作',
    key: 'actions',
    render(row: any) {
      return h(
        NPopconfirm,
        {
          onPositiveClick: () => handleDeletePasskey(row.id)
        },
        {
          trigger: () => h(NButton, { size: 'small', type: 'error', quaternary: true }, { default: () => '删除' }),
          default: () => '确定要删除此通行密钥吗？'
        }
      )
    }
  }
]

onMounted(async () => {
  await fetchPasskeys()
})

async function fetchPasskeys() {
  if (!userStore.token) return
  passkeyLoading.value = true
  try {
    const res = await fetch('/api/v1/webauthn/credentials', {
      headers: { 'x-token': userStore.token }
    })
    const json = await res.json()
    if (json.code === 20000) {
      passkeys.value = json.data?.items || []
    }
  } catch (err) {
    console.error(err)
  } finally {
    passkeyLoading.value = false
  }
}

async function handleOpenTOTPSetup() {
  try {
    const res = await fetch('/api/v1/user/totp/setup', {
      method: 'POST',
      headers: { 'x-token': userStore.token }
    })
    const json = await res.json()
    if (json.code === 20000) {
      totpQR.value = json.data.qr_image
      totpSecret.value = json.data.secret
      verifyCode.value = ''
      showTOTPModal.value = true
    } else {
      message.error(json.message || '初始化两步验证失败')
    }
  } catch (e: any) {
    message.error(e.message || '初始化两步验证失败')
  }
}

async function handleConfirmTOTP() {
  if (!verifyCode.value || verifyCode.value.length < 6) {
    message.error('请输入 6 位动态验证码')
    return
  }
  verifyingTOTP.value = true
  try {
    const res = await fetch('/api/v1/user/totp/verify', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-token': userStore.token
      },
      body: JSON.stringify({ code: verifyCode.value })
    })
    const json = await res.json()
    if (json.code === 20000) {
      message.success('两步验证已成功开启')
      showTOTPModal.value = false
      if (userStore.user) {
        userStore.user.totp_enabled = 1
      }
    } else {
      message.error(json.data?.message || json.message || '验证失败')
    }
  } catch (e: any) {
    message.error(e.message || '验证失败')
  } finally {
    verifyingTOTP.value = false
  }
}

async function handleConfirmDisableTOTP() {
  if (!disablePassword.value) {
    message.error('请输入登录密码')
    return
  }
  disablingTOTP.value = true
  try {
    const res = await fetch('/api/v1/user/totp/disable', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-token': userStore.token
      },
      body: JSON.stringify({ password: disablePassword.value })
    })
    const json = await res.json()
    if (json.code === 20000) {
      message.success('两步验证已关闭')
      showDisableTOTPModal.value = false
      disablePassword.value = ''
      if (userStore.user) {
        userStore.user.totp_enabled = 0
      }
    } else {
      message.error(json.data?.message || json.message || '关闭失败')
    }
  } catch (e: any) {
    message.error(e.message || '关闭失败')
  } finally {
    disablingTOTP.value = false
  }
}

async function handleRegisterPasskey() {
  if (!window.PublicKeyCredential) {
    message.error('当前浏览器不支持通行密钥 (Passkey)')
    return
  }
  passkeyRegistering.value = true
  try {
    const beginRes = await fetch('/api/v1/webauthn/register/begin', {
      method: 'POST',
      headers: { 'x-token': userStore.token }
    })
    const beginJson = await beginRes.json()
    if (beginJson.code !== 20000) {
      throw new Error(beginJson.message || '发起注册失败')
    }

    const { session_id, options } = beginJson.data
    const credOptions = transformCreationOptions(options)

    const credential = (await navigator.credentials.create(credOptions)) as PublicKeyCredential
    if (!credential) {
      throw new Error('未创建有效凭据')
    }

    const payload = serializeCreationResponse(credential)
    const finishRes = await fetch(`/api/v1/webauthn/register/finish?session_id=${session_id}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-token': userStore.token
      },
      body: JSON.stringify(payload)
    })
    const finishJson = await finishRes.json()
    if (finishJson.code !== 20000) {
      throw new Error(finishJson.data?.message || finishJson.message || '保存凭据失败')
    }

    message.success('通行密钥已成功绑定')
    await fetchPasskeys()
  } catch (e: any) {
    if (e.name === 'NotAllowedError') {
      message.warning('已取消通行密钥创建')
    } else {
      message.error(e.message || '绑定通行密钥失败')
    }
  } finally {
    passkeyRegistering.value = false
  }
}

async function handleDeletePasskey(id: number) {
  try {
    const res = await fetch('/api/v1/webauthn/credentials/delete', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'x-token': userStore.token
      },
      body: JSON.stringify({ id })
    })
    const json = await res.json()
    if (json.code === 20000) {
      message.success('通行密钥已删除')
      await fetchPasskeys()
    } else {
      message.error(json.message || '删除失败')
    }
  } catch (e: any) {
    message.error(e.message || '删除失败')
  }
}

async function handleDownloadBackup() {
  if (!userStore.token) {
    message.error('请先登录')
    return
  }
  downloading.value = true
  try {
    const res = await fetch('/api/v1/admin/backup/database', {
      method: 'POST',
      headers: {
        'x-token': userStore.token
      }
    })
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const contentType = res.headers.get('content-type') || ''
    if (contentType.includes('application/json')) {
      const errJson = await res.json()
      throw new Error(errJson.data?.message || errJson.message || '备份失败')
    }

    const blob = await res.blob()
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const disposition = res.headers.get('content-disposition') || ''
    let filename = 'nrllink_backup.sqlite3'
    const match = disposition.match(/filename="?([^"]+)"?/)
    if (match && match[1]) {
      filename = match[1]
    }
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    window.URL.revokeObjectURL(url)
    message.success('数据库快照已下载')
  } catch (err: any) {
    message.error(err.message || '备份下载失败')
  } finally {
    downloading.value = false
  }
}
</script>

<style scoped>
.page { padding: 24px; }
</style>
