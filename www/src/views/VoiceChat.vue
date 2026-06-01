<template>
  <div class="voice-chat">
    <h2>Web 语音对讲</h2>
    <div class="room-list">
      <n-card v-for="room in rooms" :key="room.room_key" class="room-card" :class="{ active: room.active }">
        <div class="room-header">
          <span class="room-name">{{ room.room_name }}</span>
          <span v-if="room.active" class="callsign">{{ room.callsign }}-{{ room.ssid }}</span>
          <n-tag v-if="subscribed[room.room_key]" size="small" type="success">收听中</n-tag>
        </div>
        <div class="room-actions">
          <n-button size="small" :type="subscribed[room.room_key] ? 'warning' : 'primary'" @click="toggleSubscribe(room.room_key)">
            {{ subscribed[room.room_key] ? '取消收听' : '收听' }}
          </n-button>
        </div>
      </n-card>
    </div>
    <div class="status-bar">
      <span v-if="lastCodec" class="codec-badge">{{ lastCodec }}</span>
      <span v-if="speakerList.length > 0" class="speakers">
        {{ speakerList.join(', ') }} 正在说话
      </span>
    </div>
    <div class="ptt-area">
      <n-button class="ptt-btn" :type="pttActive ? 'error' : 'primary'" size="large" block
        @mousedown="startPTT" @mouseup="stopPTT" @mouseleave="stopPTT"
        @touchstart.prevent="startPTT" @touchend="stopPTT">
        {{ pttActive ? '松开停止' : '按住说话' }}
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { NCard, NButton, NTag } from 'naive-ui'
import { useUserStore } from '../stores/user'
import { decodeAudioFrame, playAudio } from '../utils/audio-decoder'

const userStore = useUserStore()
const rooms = ref<any[]>([])
const subscribed = ref<Record<string, boolean>>({})
const pttActive = ref(false)
const lastCodec = ref('')
const speakerList = ref<string[]>([])

let ws: WebSocket | null = null
let audioBufferQueue: ArrayBuffer[] = []
let isPlaying = false

// 编码类型名称
const CodecNames: Record<number, string> = {
  1: 'G.711',
  8: 'Opus',
  12: 'Codec2 700C',
  13: 'Codec2 1300',
  14: 'Codec2 1600',
  15: 'Codec2 2400',
  16: 'Codec2 3200'
}

function connectWS() {
  const token = userStore.token
  ws = new WebSocket(`ws://${location.host}/ws/calls?token=${token}`)
  ws.binaryType = 'arraybuffer'

  ws.onmessage = async (e) => {
    if (e.data instanceof ArrayBuffer) {
      // 音频帧: [1字节CodecType][N字节音频数据]
      await handleAudioFrame(e.data)
    } else {
      try {
        const msg = JSON.parse(e.data)
        if (msg.type === 'snapshot') {
          rooms.value = msg.rooms || []
        } else if (msg.type === 'room_state' && msg.room) {
          const idx = rooms.value.findIndex((r: any) => r.room_key === msg.room.room_key)
          if (idx >= 0) {
            rooms.value[idx] = msg.room
          } else {
            rooms.value.push(msg.room)
          }
          // 更新说话者列表
          if (msg.room.speakers && msg.room.active) {
            speakerList.value = msg.room.speakers.map((s: any) => s.callsign)
          } else if (!msg.room.active) {
            speakerList.value = []
          }
        } else if (msg.type === 'subscriptions') {
          const subs: Record<string, boolean> = {}
          msg.subscriptions?.forEach((k: string) => { subs[k] = true })
          subscribed.value = subs
        }
      } catch { /* ignore parse errors */ }
    }
  }

  ws.onclose = () => {
    lastCodec.value = ''
    setTimeout(connectWS, 3000)
  }
}

async function handleAudioFrame(frame: ArrayBuffer) {
  if (frame.byteLength < 1) return

  // 读取编码类型
  const view = new Uint8Array(frame)
  const codecType = view[0]
  lastCodec.value = CodecNames[codecType] || `Type ${codecType}`

  // 解码
  const pcm = await decodeAudioFrame(frame)
  if (!pcm) return

  // 播放
  audioBufferQueue.push(pcm.buffer as ArrayBuffer)
  if (!isPlaying) {
    isPlaying = true
    await processQueue()
  }
}

async function processQueue() {
  while (audioBufferQueue.length > 0) {
    const item = audioBufferQueue.shift()!
    const pcm = new Int16Array(item)
    await playAudio(pcm)
  }
  isPlaying = false
}

function toggleSubscribe(key: string) {
  if (!ws) return
  const action = subscribed.value[key] ? 'unsubscribe' : 'subscribe'
  ws.send(JSON.stringify({ action, room_keys: [key] }))
}

async function startPTT() {
  pttActive.value = true
  // TODO: PTT 实现 - getUserMedia + 编码 + WS 发送
  // 需要获取麦克风音频，编码为 Opus/Codec2 发送
}

function stopPTT() {
  pttActive.value = false
}

onMounted(connectWS)
onUnmounted(() => {
  ws?.close()
  audioBufferQueue = []
})
</script>

<style scoped>
.voice-chat { padding: 24px; }
.room-list { display: grid; gap: 12px; margin-bottom: 24px; }
.room-card { background: var(--bg-card); border: 1px solid var(--border); }
.room-card.active { border-color: var(--accent); }
.room-header { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.room-name { font-size: 16px; font-weight: 600; }
.callsign { color: var(--accent); font-size: 13px; }
.status-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; min-height: 24px; }
.codec-badge { background: var(--bg-card); padding: 2px 8px; border-radius: 4px; font-size: 12px; color: var(--accent); }
.speakers { color: var(--accent); font-size: 13px; }
.ptt-area { position: sticky; bottom: 24px; }
.ptt-btn { height: 56px; font-size: 18px; border-radius: 28px; }
</style>
