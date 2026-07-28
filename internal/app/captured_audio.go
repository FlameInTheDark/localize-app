package app

import (
	"encoding/binary"
	"math"
)

const (
	capturedSpeechFrameMilliseconds = 20
	capturedSpeechMinimumFrames     = 6
	capturedSpeechRMS               = 0.0075
	capturedSpeechPeak              = 0.018
)

// capturedWAVHasSpeech validates Localize's PCM WAV capture and returns true
// only when it contains enough audible speech. The browser capture path always
// writes 16-bit PCM, so rejecting quiet audio here prevents it reaching Whisper
// and being decoded as a hallucinated phrase.
func capturedWAVHasSpeech(data []byte) (valid bool, hasSpeech bool) {
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return false, false
	}

	var channels, sampleRate, bitsPerSample uint16
	var samples []byte
	for offset := 12; offset+8 <= len(data); {
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkStart := offset + 8
		chunkEnd := chunkStart + chunkSize
		if chunkSize < 0 || chunkEnd < chunkStart || chunkEnd > len(data) {
			return false, false
		}
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if chunkSize < 16 || binary.LittleEndian.Uint16(data[chunkStart:chunkStart+2]) != 1 {
				return false, false
			}
			channels = binary.LittleEndian.Uint16(data[chunkStart+2 : chunkStart+4])
			sampleRate32 := binary.LittleEndian.Uint32(data[chunkStart+4 : chunkStart+8])
			if sampleRate32 == 0 || sampleRate32 > 65535 {
				return false, false
			}
			sampleRate = uint16(sampleRate32)
			bitsPerSample = binary.LittleEndian.Uint16(data[chunkStart+14 : chunkStart+16])
		case "data":
			samples = data[chunkStart:chunkEnd]
		}
		offset = chunkEnd
		if chunkSize%2 == 1 {
			offset++
		}
	}

	if channels != 1 || sampleRate == 0 || bitsPerSample != 16 || len(samples) < 2 {
		return false, false
	}
	frameSamples := int(sampleRate) * capturedSpeechFrameMilliseconds / 1000
	if frameSamples < 1 {
		return false, false
	}
	activeFrames := 0
	for offset := 0; offset+frameSamples*2 <= len(samples); offset += frameSamples * 2 {
		var sumSquares int64
		peak := int32(0)
		for index := 0; index < frameSamples; index++ {
			value := int32(binary.LittleEndian.Uint16(samples[offset+index*2 : offset+index*2+2]))
			if value >= 32768 {
				value -= 65536
			}
			absolute := value
			if absolute < 0 {
				absolute = -absolute
			}
			if absolute > peak {
				peak = absolute
			}
			sumSquares += int64(value) * int64(value)
		}
		rms := math.Sqrt(float64(sumSquares)/float64(frameSamples)) / 32768
		if rms >= capturedSpeechRMS && float64(peak)/32768 >= capturedSpeechPeak {
			activeFrames++
			if activeFrames >= capturedSpeechMinimumFrames {
				return true, true
			}
		}
	}
	return true, false
}
