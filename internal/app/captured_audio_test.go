package app

import (
	"encoding/binary"
	"testing"
)

func TestCapturedWAVHasSpeech(t *testing.T) {
	tests := []struct {
		name    string
		samples []int16
		valid   bool
		speech  bool
	}{
		{name: "quiet recording", samples: make([]int16, 16000), valid: true, speech: false},
		{name: "audible phrase", samples: voicedSamples(16000), valid: true, speech: true},
		{name: "invalid header", samples: nil, valid: false, speech: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := testWAV(test.samples)
			if test.name == "invalid header" {
				data = []byte("not a wav")
			}
			valid, speech := capturedWAVHasSpeech(data)
			if valid != test.valid || speech != test.speech {
				t.Fatalf("got valid=%t speech=%t", valid, speech)
			}
		})
	}
}

func voicedSamples(sampleRate int) []int16 {
	samples := make([]int16, sampleRate)
	for index := sampleRate / 4; index < sampleRate/2; index++ {
		if index%8 < 4 {
			samples[index] = 2200
		} else {
			samples[index] = -2200
		}
	}
	return samples
}

func testWAV(samples []int16) []byte {
	data := make([]byte, 44+len(samples)*2)
	copy(data[0:], "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], 16000)
	binary.LittleEndian.PutUint32(data[28:], 32000)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(len(samples)*2))
	for index, sample := range samples {
		binary.LittleEndian.PutUint16(data[44+index*2:], uint16(sample))
	}
	return data
}
