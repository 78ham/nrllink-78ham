package main

import (
	"fmt"
	"sync"

	opus "gopkg.in/hraban/opus.v2"
)

// opusCodec 实现 VoiceCodec 接口，包装 libopus 编解码。
type opusCodec struct {
	encoder   *opus.Encoder
	decoder   *opus.Decoder
	encMu     sync.Mutex
	decMu     sync.Mutex
}

func newOpusCodec() (*opusCodec, error) {
	encoder, err := opus.NewEncoder(16000, 1, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("opus: create encoder: %w", err)
	}

	decoder, err := opus.NewDecoder(16000, 1)
	if err != nil {
		return nil, fmt.Errorf("opus: create decoder: %w", err)
	}

	return &opusCodec{
		encoder: encoder,
		decoder: decoder,
	}, nil
}

func (c *opusCodec) Type() byte { return CodecTypeOpus }

func (c *opusCodec) DecodeToPCM(frame []byte) ([]int16, error) {
	c.decMu.Lock()
	defer c.decMu.Unlock()

	pcm := make([]int16, 320) // 20ms @ 16kHz = 320 samples
	n, err := c.decoder.Decode(frame, pcm)
	if err != nil {
		return nil, fmt.Errorf("opus: decode: %w", err)
	}
	return pcm[:n], nil
}

func (c *opusCodec) EncodeFromPCM(pcm []int16) ([]byte, error) {
	c.encMu.Lock()
	defer c.encMu.Unlock()

	buf := make([]byte, 400) // opus max frame size for 20ms @ 16k
	n, err := c.encoder.Encode(pcm, buf)
	if err != nil {
		return nil, fmt.Errorf("opus: encode: %w", err)
	}
	return buf[:n], nil
}

func (c *opusCodec) SampleRate() int   { return 16000 }
func (c *opusCodec) FrameSamples() int { return 320 }
