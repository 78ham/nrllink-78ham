// 多编码音频解码器
// 格式：WebSocket 二进制帧 = [1 byte CodecType] + [N bytes Audio Data]
// CodecType: 1=G.711, 8=Opus, 12-16=Codec2

const CodecTypeG711 = 1
const CodecTypeOpus = 8
const CodecTypeCodec2_700C = 12

// Codec2 模式映射 (Type -> libcodec2 mode)
const Codec2Mode: Record<number, number> = {
  12: 5,  // CODEC2_MODE_700C
  13: 6,  // CODEC2_MODE_1300
  14: 7,  // CODEC2_MODE_1600
  15: 8,  // CODEC2_MODE_2400
  16: 9,  // CODEC2_MODE_3200
}

let audioCtx: AudioContext | null = null
let codec2Module: any = null
let codec2Loading: Promise<any> | null = null

function getAudioContext(): AudioContext {
  if (!audioCtx) {
    audioCtx = new AudioContext({ sampleRate: 8000 })
  }
  return audioCtx
}

// ==================== G.711 A-law 解码 ====================

const alawTable = new Int16Array(256)
for (let i = 0; i < 256; i++) {
  const code = i ^ 0x55
  const sign = code & 0x80 ? 1 : -1
  const exp = (code & 0x70) >> 4
  let mant = code & 0x0f
  if (exp > 0) mant += 16
  mant = (mant << 4) + 8
  if (exp > 1) mant <<= (exp - 1)
  alawTable[i] = sign * mant
}

function decodeG711(data: Uint8Array): Int16Array {
  const out = new Int16Array(data.length)
  for (let i = 0; i < data.length; i++) out[i] = alawTable[data[i]]
  return out
}

// ==================== Codec2 WASM 解码 ====================

async function loadCodec2Module(): Promise<any> {
  if (codec2Module) return codec2Module
  if (codec2Loading) return codec2Loading

  codec2Loading = (async () => {
    // 方式1: 检查全局是否已有 Codec2Module (emscripten 预加载)
    if ((window as any).Codec2Module) {
      codec2Module = (window as any).Codec2Module
      return codec2Module
    }

    // 方式2: 动态加载 emscripten 生成的 JS 模块
    try {
      await loadScript('/wasm/codec2.js')
      // emscripten 生成的模块会自动初始化
      if ((window as any).Codec2Module) {
        codec2Module = (window as any).Codec2Module
        return codec2Module
      }
      // 如果有 Module 函数，等待初始化
      if ((window as any).Module) {
        codec2Module = await (window as any).Module
        return codec2Module
      }
    } catch (e) {
      console.error('Failed to load Codec2 WASM:', e)
    }

    throw new Error('Codec2 WASM module not found. Run: bash scripts/build-codec2-wasm.sh')
  })()

  return codec2Loading
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`Failed to load ${src}`))
    document.head.appendChild(script)
  })
}

async function decodeCodec2(data: Uint8Array, codecType: number): Promise<Int16Array> {
  const c2 = await loadCodec2Module()
  const mode = Codec2Mode[codecType]
  if (!mode) throw new Error(`Unsupported Codec2 type: ${codecType}`)

  const samplesPerFrame = c2.codec2_samples_per_frame(mode)
  const bytesPerFrame = c2.codec2_bytes_per_frame(mode)

  // 分配 WASM 内存
  const pcmPtr = c2._malloc(samplesPerFrame * 2) // int16 = 2 bytes
  const dataPtr = c2._malloc(bytesPerFrame)

  try {
    // 复制编码数据到 WASM 内存
    c2.HEAPU8.set(data.slice(0, bytesPerFrame), dataPtr)

    // 解码
    c2.codec2_decode(mode, pcmPtr, dataPtr)

    // 读取 PCM 数据
    const pcm = new Int16Array(samplesPerFrame)
    pcm.set(c2.HEAP16.subarray(pcmPtr / 2, pcmPtr / 2 + samplesPerFrame))
    return pcm
  } finally {
    c2._free(pcmPtr)
    c2._free(dataPtr)
  }
}

// ==================== Opus 解码 ====================

// Opus 使用浏览器原生 WebCodecs API (Chrome 94+)
// 如果浏览器不支持，回退到服务器端转 G.711
async function decodeOpus(data: Uint8Array): Promise<Int16Array | null> {
  // 检查 WebCodecs API 支持
  if (typeof AudioDecoder === 'undefined') {
    console.warn('AudioDecoder API not available. Use Chrome 94+ or Firefox.')
    return null
  }

  // TODO: 实现 WebCodecs Opus 解码
  // 需要知道采样率和帧大小
  console.warn('Opus browser decode not yet implemented')
  return null
}

// ==================== 主解码函数 ====================

export async function decodeAudioFrame(frame: ArrayBuffer): Promise<Int16Array | null> {
  const view = new Uint8Array(frame)
  if (view.length < 1) return null

  const codecType = view[0]
  const data = view.slice(1)

  try {
    switch (codecType) {
      case CodecTypeG711:
        return decodeG711(data)
      case CodecTypeOpus:
        return await decodeOpus(data)
      case CodecTypeCodec2_700C:
      case 13:
      case 14:
      case 15:
      case 16:
        return await decodeCodec2(data, codecType)
      default:
        console.warn(`Unknown codec type: ${codecType}`)
        return null
    }
  } catch (e) {
    console.error(`Decode error (codec ${codecType}):`, e)
    return null
  }
}

// ==================== 播放队列 ====================

let playbackQueue: { pcm: Int16Array; sampleRate: number }[] = []
let isPlaying = false

export async function playAudio(pcm: Int16Array, sampleRate: number = 8000) {
  playbackQueue.push({ pcm, sampleRate })
  if (!isPlaying) {
    isPlaying = true
    await processPlaybackQueue()
  }
}

async function processPlaybackQueue() {
  while (playbackQueue.length > 0) {
    const item = playbackQueue.shift()!
    await playOneFrame(item.pcm, item.sampleRate)
  }
  isPlaying = false
}

function playOneFrame(pcm: Int16Array, sampleRate: number): Promise<void> {
  return new Promise(resolve => {
    const ctx = getAudioContext()
    const buffer = ctx.createBuffer(1, pcm.length, sampleRate)
    const float32 = new Float32Array(pcm.length)
    for (let i = 0; i < pcm.length; i++) {
      float32[i] = pcm[i] / 32768
    }
    buffer.getChannelData(0).set(float32)

    const source = ctx.createBufferSource()
    source.buffer = buffer
    source.connect(ctx.destination)
    source.onended = () => resolve()
    source.start()
  })
}

// ==================== 导出工具函数 ====================

export const CodecNames: Record<number, string> = {
  1: 'G.711',
  8: 'Opus',
  12: 'Codec2 700C',
  13: 'Codec2 1300',
  14: 'Codec2 1600',
  15: 'Codec2 2400',
  16: 'Codec2 3200',
}

export function getCodecName(codecType: number): string {
  return CodecNames[codecType] || `Type ${codecType}`
}
