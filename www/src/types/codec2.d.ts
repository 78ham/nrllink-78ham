declare global {
  interface Window {
    Codec2Module?: Codec2Module
    Module?: any
  }
}

export interface Codec2Module {
  // 内存管理
  _malloc(size: number): number
  _free(ptr: number): void

  // 堆访问
  HEAPU8: Uint8Array
  HEAP16: Int16Array
  HEAP32: Int32Array

  // Codec2 API
  codec2_create(mode: number): number
  codec2_destroy(codec: number): void
  codec2_samples_per_frame(codec: number): number
  codec2_bytes_per_frame(codec: number): number
  codec2_decode(codec: number, pcm: number, bits: number): void
  codec2_encode(codec: number, bits: number, pcm: number): void
}

// Codec2 模式常量
export const CODEC2_MODES = {
  450: 0,   // CODEC2_MODE_450
  700B: 1,  // CODEC2_MODE_700B
  700C: 5,  // CODEC2_MODE_700C
  1200: 2,  // CODEC2_MODE_1200
  1300: 3,  // CODEC2_MODE_1300
  1400: 4,  // CODEC2_MODE_1400
  1600: 6,  // CODEC2_MODE_1600
  2400: 7,  // CODEC2_MODE_2400
  3200: 8,  // CODEC2_MODE_3200
} as const

export type Codec2Mode = typeof CODEC2_MODES[keyof typeof CODEC2_MODES]
