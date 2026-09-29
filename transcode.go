package main

import (
	"fmt"
	"sync"
)

type transcodeKey struct {
	SrcType byte
	DstType byte
}

type transcodeCache struct {
	mu     sync.Mutex
	cached map[transcodeKey][]byte
}

var tcache = &transcodeCache{cached: make(map[transcodeKey][]byte)}

// transcodeVoice 将音频帧从 srcCodecType 转码到 dstCodecType。
// 流程：解码 → PCM → 重采样(如需要) → 编码。
// 支持 intra-frame 缓存：同一帧数据同 src→dst 只转一次。
func transcodeVoice(data []byte, srcCodecType, dstCodecType byte) ([]byte, error) {
	if srcCodecType == dstCodecType {
		return data, nil
	}

	key := transcodeKey{SrcType: srcCodecType, DstType: dstCodecType}

	tcache.mu.Lock()
	if cached, ok := tcache.cached[key]; ok {
		delete(tcache.cached, key)
		tcache.mu.Unlock()
		return cached, nil
	}
	tcache.mu.Unlock()

	// Opus/Codec2 是重量级 C 资源，必须走池化实例；NewCodec 每帧新建且无法被 GC 回收
	srcCodec, err := GetPooledCodec(srcCodecType)
	if err != nil {
		return nil, fmt.Errorf("transcode: source codec: %w", err)
	}

	dstCodec, err := GetPooledCodec(dstCodecType)
	if err != nil {
		return nil, fmt.Errorf("transcode: dest codec: %w", err)
	}

	pcm, err := srcCodec.DecodeToPCM(data)
	if err != nil {
		return nil, fmt.Errorf("transcode: decode: %w", err)
	}

	if srcCodec.SampleRate() != dstCodec.SampleRate() {
		pcm = ResamplePCM(pcm, srcCodec.SampleRate(), dstCodec.SampleRate())
	}

	out, err := dstCodec.EncodeFromPCM(pcm)
	if err != nil {
		return nil, fmt.Errorf("transcode: encode: %w", err)
	}

	tcache.mu.Lock()
	tcache.cached[key] = out
	tcache.mu.Unlock()

	return out, nil
}

// transcodePacket 转码整个 NRL2 包，生成新的包发给目标设备。
// 只替换 DATA 和 Type、CodecType 字段，其余保持。
func transcodePacket(packet []byte, srcCodecType, dstCodecType byte, targetDev *deviceInfo) ([]byte, error) {
	data := packet[48:]
	if len(data) == 0 {
		return nil, fmt.Errorf("transcode: empty data")
	}

	newData, err := transcodeVoice(data, srcCodecType, dstCodecType)
	if err != nil {
		return nil, err
	}

	newPacket := make([]byte, 48+len(newData))
	copy(newPacket, packet[:48])
	newPacket[20] = dstCodecType
	newPacket[32] = dstCodecType      // 更新 CodecType
	newPacket[33] = targetDev.SupportedCodecs // 更新目标设备的 CodecCaps
	copy(newPacket[48:], newData)

	return newPacket, nil
}

// clearTranscodeCache 清理转码缓存（每帧结束后调用）。
// 上一帧没有转码发生时无需重建 map，避免每帧白抢锁+分配
func clearTranscodeCache() {
	tcache.mu.Lock()
	if len(tcache.cached) > 0 {
		tcache.cached = make(map[transcodeKey][]byte)
	}
	tcache.mu.Unlock()
}
