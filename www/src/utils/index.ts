// 音频解码工具统一导出
export { decodeAudioFrame, playAudio, CodecNames, getCodecName } from './audio-decoder'

// CodecType 枚举
export const CodecType = {
  G711: 1,
  OPUS: 8,
  CODEC2_700C: 12,
  CODEC2_1300: 13,
  CODEC2_1600: 14,
  CODEC2_2400: 15,
  CODEC2_3200: 16,
} as const

// CodecCaps 位图
export const CodecCap = {
  G711: 1 << 0,
  OPUS: 1 << 1,
  CODEC2: 1 << 2,
} as const

// 判断设备是否支持指定编码
export function deviceSupportsCodec(deviceCaps: number, codecType: number): boolean {
  if (codecType === CodecType.G711) return (deviceCaps & CodecCap.G711) !== 0
  if (codecType === CodecType.OPUS) return (deviceCaps & CodecCap.OPUS) !== 0
  if (codecType >= CodecType.CODEC2_700C && codecType <= CodecType.CODEC2_3200) {
    return (deviceCaps & CodecCap.CODEC2) !== 0
  }
  return false
}
