package main

import (
	"fmt"
)

// VoiceCodec 是所有语音编码的统一接口。
// 所有"需要 PCM 的地方"统一调 DecodeToPCM，消除散落各处的 alaw2linear 硬编码。
type VoiceCodec interface {
	// Type 返回对应的 NRL2 Type 值
	Type() byte

	// DecodeToPCM 将编码帧解码为 int16 PCM 样本
	DecodeToPCM(frame []byte) ([]int16, error)

	// EncodeFromPCM 将 int16 PCM 样本编码为编码帧
	EncodeFromPCM(pcm []int16) ([]byte, error)

	// SampleRate 返回编码的采样率
	SampleRate() int

	// FrameSamples 返回每帧的 PCM 样本数
	FrameSamples() int
}

// CodecType 枚举支持的编码类型
const (
	CodecTypeG711     byte = 1
	CodecTypeOpus     byte = 8
	CodecTypeCodec2_700C byte = 12
	CodecTypeCodec2_1300 byte = 13
	CodecTypeCodec2_1600 byte = 14
	CodecTypeCodec2_2400 byte = 15
	CodecTypeCodec2_3200 byte = 16
)

// CodecCaps 位图定义
const (
	CodecCapG711  byte = 1 << 0
	CodecCapOpus  byte = 1 << 1
	CodecCapCodec2 byte = 1 << 2
)

// NewCodec 根据 NRL2 Type 创建对应的编码实例。
func NewCodec(typeByte byte, mode ...int) (VoiceCodec, error) {
	switch typeByte {
	case CodecTypeG711:
		return &g711Codec{}, nil
	case CodecTypeOpus:
		return newOpusCodec()
	case CodecTypeCodec2_700C:
		return newCodec2Codec(700)
	case CodecTypeCodec2_1300:
		return newCodec2Codec(1300)
	case CodecTypeCodec2_1600:
		return newCodec2Codec(1600)
	case CodecTypeCodec2_2400:
		return newCodec2Codec(2400)
	case CodecTypeCodec2_3200:
		return newCodec2Codec(3200)
	default:
		return nil, fmt.Errorf("codec: unsupported type %d", typeByte)
	}
}

// DecodeToPCM 便捷函数：从编码类型和帧数据直接解码到 PCM
func DecodeToPCM(typeByte byte, frame []byte) ([]int16, error) {
	c, err := NewCodec(typeByte)
	if err != nil {
		return nil, err
	}
	return c.DecodeToPCM(frame)
}

// EncodeFromPCM 便捷函数：从 PCM 和编码类型直接编码
func EncodeFromPCM(typeByte byte, pcm []int16) ([]byte, error) {
	c, err := NewCodec(typeByte)
	if err != nil {
		return nil, err
	}
	return c.EncodeFromPCM(pcm)
}

// CodecSupports 将 NRL2 Type 映射到 CodecCaps 位图中的位
func TypeToCodecCap(typeByte byte) byte {
	switch typeByte {
	case CodecTypeG711:
		return CodecCapG711
	case CodecTypeOpus:
		return CodecCapOpus
	case CodecTypeCodec2_700C, CodecTypeCodec2_1300, CodecTypeCodec2_1600, CodecTypeCodec2_2400, CodecTypeCodec2_3200:
		return CodecCapCodec2
	default:
		return 0
	}
}

// CodecCapsNames 返回 CodecCaps 位图的可读描述
func CodecCapsNames(caps byte) []string {
	var names []string
	if caps&CodecCapG711 != 0 {
		names = append(names, "G.711")
	}
	if caps&CodecCapOpus != 0 {
		names = append(names, "Opus")
	}
	if caps&CodecCapCodec2 != 0 {
		names = append(names, "Codec2")
	}
	return names
}

// ResamplePCM 将 PCM 数据从一个采样率重采样到另一个采样率。
// 使用线性插值（适合语音场景，CPU 极低）。
func ResamplePCM(input []int16, fromRate, toRate int) []int16 {
	if fromRate == toRate {
		out := make([]int16, len(input))
		copy(out, input)
		return out
	}

	outLen := len(input) * toRate / fromRate
	output := make([]int16, outLen)

	for i := 0; i < outLen; i++ {
		srcPos := float64(i) * float64(fromRate) / float64(toRate)
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)

		if srcIdx >= len(input)-1 {
			output[i] = input[len(input)-1]
			continue
		}

		sample := float64(input[srcIdx])*(1-frac) + float64(input[srcIdx+1])*frac
		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}
		output[i] = int16(sample)
	}

	return output
}
