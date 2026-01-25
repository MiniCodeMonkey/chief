package notify

import "math"

// GenerateWAV generates a WAV file with a pleasant completion chime.
// This is exported for use in generating the embedded asset.
func GenerateWAV() []byte {
	sampleRate := 22050 // Lower sample rate for smaller file
	duration := 0.4     // seconds - short pleasant chime
	numSamples := int(float64(sampleRate) * duration)

	// WAV file format: 16-bit mono for smaller size
	channels := 1
	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := numSamples * channels * bitsPerSample / 8

	// Create buffer for WAV file
	buf := make([]byte, 44+dataSize)

	// RIFF header
	copy(buf[0:4], "RIFF")
	writeUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")

	// fmt chunk
	copy(buf[12:16], "fmt ")
	writeUint32(buf[16:20], 16) // chunk size
	writeUint16(buf[20:22], 1)  // PCM format
	writeUint16(buf[22:24], uint16(channels))
	writeUint32(buf[24:28], uint32(sampleRate))
	writeUint32(buf[28:32], uint32(byteRate))
	writeUint16(buf[32:34], uint16(blockAlign))
	writeUint16(buf[34:36], uint16(bitsPerSample))

	// data chunk
	copy(buf[36:40], "data")
	writeUint32(buf[40:44], uint32(dataSize))

	// Generate audio samples - a pleasant two-tone chime
	offset := 44
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		// Envelope: quick attack, gradual decay
		envelope := math.Exp(-t * 4.0)
		if t < 0.01 {
			// Quick attack
			envelope = t / 0.01
		}

		// Two harmonious frequencies (C5 and E5 for a major third)
		freq1 := 523.25 // C5
		freq2 := 659.26 // E5
		freq3 := 783.99 // G5 - adds brightness

		// Mix frequencies with different amplitudes
		sample := 0.5 * math.Sin(2*math.Pi*freq1*t)
		sample += 0.35 * math.Sin(2*math.Pi*freq2*t)
		sample += 0.15 * math.Sin(2*math.Pi*freq3*t)

		// Apply envelope and scale to 16-bit
		sample *= envelope * 0.7 // 70% max volume
		value := int16(sample * 32767)

		// Write mono sample
		writeInt16(buf[offset:offset+2], value)
		offset += 2
	}

	return buf
}

func writeUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func writeUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func writeInt16(b []byte, v int16) {
	writeUint16(b, uint16(v))
}
