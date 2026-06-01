package main

/*
#cgo CFLAGS: -I${SRCDIR}/codec2/src
#cgo LDFLAGS: -L${SRCDIR}/codec2/build/src -lcodec2
#include <codec2/codec2.h>
*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"
)

// Codec2 模式常量 (与 libcodec2 定义一致)
const (
	Codec2Mode700C = 5  // CODEC2_MODE_700C
	Codec2Mode1300 = 6  // CODEC2_MODE_1300
	Codec2Mode1600 = 7  // CODEC2_MODE_1600
	Codec2Mode2400 = 8  // CODEC2_MODE_2400
	Codec2Mode3200 = 9  // CODEC2_MODE_3200
)

// codec2Codec 实现 VoiceCodec 接口，包装 libcodec2。
type codec2Codec struct {
	mode       int
	enc        *C.struct_codec2
	dec        *C.struct_codec2
	encMu      sync.Mutex
	decMu      sync.Mutex
	samplesPerFrame int
	bytesPerFrame   int
}

func newCodec2Codec(bitrate int) (*codec2Codec, error) {
	var c2mode int
	switch bitrate {
	case 700:
		c2mode = Codec2Mode700C
	case 1300:
		c2mode = Codec2Mode1300
	case 1600:
		c2mode = Codec2Mode1600
	case 2400:
		c2mode = Codec2Mode2400
	case 3200:
		c2mode = Codec2Mode3200
	default:
		return nil, fmt.Errorf("codec2: unsupported bitrate %d", bitrate)
	}

	enc := C.codec2_create(C.int(c2mode))
	if enc == nil {
		return nil, fmt.Errorf("codec2: create encoder failed")
	}

	dec := C.codec2_create(C.int(c2mode))
	if dec == nil {
		C.codec2_destroy(enc)
		return nil, fmt.Errorf("codec2: create decoder failed")
	}

	samples := int(C.codec2_samples_per_frame(enc))
	bytes := int(C.codec2_bytes_per_frame(enc))

	return &codec2Codec{
		mode:            bitrate,
		enc:             enc,
		dec:             dec,
		samplesPerFrame: samples,
		bytesPerFrame:   bytes,
	}, nil
}

func (c *codec2Codec) typeByte() byte {
	switch c.mode {
	case 700:
		return CodecTypeCodec2_700C
	case 1300:
		return CodecTypeCodec2_1300
	case 1600:
		return CodecTypeCodec2_1600
	case 2400:
		return CodecTypeCodec2_2400
	case 3200:
		return CodecTypeCodec2_3200
	default:
		return CodecTypeCodec2_700C
	}
}

func (c *codec2Codec) Type() byte { return c.typeByte() }

func (c *codec2Codec) DecodeToPCM(frame []byte) ([]int16, error) {
	c.decMu.Lock()
	defer c.decMu.Unlock()

	if len(frame) < c.bytesPerFrame {
		return nil, fmt.Errorf("codec2: frame too short, need %d got %d", c.bytesPerFrame, len(frame))
	}

	pcm := make([]int16, c.samplesPerFrame)
	C.codec2_decode(c.dec, (*C.short)(unsafe.Pointer(&pcm[0])), (*C.uchar)(unsafe.Pointer(&frame[0])))
	return pcm, nil
}

func (c *codec2Codec) EncodeFromPCM(pcm []int16) ([]byte, error) {
	c.encMu.Lock()
	defer c.encMu.Unlock()

	if len(pcm) < c.samplesPerFrame {
		return nil, fmt.Errorf("codec2: pcm too short, need %d got %d", c.samplesPerFrame, len(pcm))
	}

	frame := make([]byte, c.bytesPerFrame)
	C.codec2_encode(c.enc, (*C.uchar)(unsafe.Pointer(&frame[0])), (*C.short)(unsafe.Pointer(&pcm[0])))
	return frame, nil
}

func (c *codec2Codec) SampleRate() int { return 8000 }

func (c *codec2Codec) FrameSamples() int { return c.samplesPerFrame }

// Close 释放 codec2 资源
func (c *codec2Codec) Close() {
	c.encMu.Lock()
	c.decMu.Lock()
	defer c.encMu.Unlock()
	defer c.decMu.Unlock()

	if c.enc != nil {
		C.codec2_destroy(c.enc)
		c.enc = nil
	}
	if c.dec != nil {
		C.codec2_destroy(c.dec)
		c.dec = nil
	}
}
