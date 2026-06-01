package main

// g711Codec 实现 VoiceCodec 接口，包装现有 g711.go 的 A-law 编解码。
type g711Codec struct{}

func (c *g711Codec) Type() byte { return CodecTypeG711 }

func (c *g711Codec) DecodeToPCM(frame []byte) ([]int16, error) {
	pcm := make([]int16, len(frame))
	for i, b := range frame {
		pcm[i] = alaw2linear(b)
	}
	return pcm, nil
}

func (c *g711Codec) EncodeFromPCM(pcm []int16) ([]byte, error) {
	encoded := make([]byte, len(pcm))
	for i, s := range pcm {
		encoded[i] = Linear2Alaw(s)
	}
	return encoded, nil
}

func (c *g711Codec) SampleRate() int    { return 8000 }
func (c *g711Codec) FrameSamples() int  { return 160 }
