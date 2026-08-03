package asr

import "math"

const ditherSeed uint64 = 0x6a09e667f3bcc909

func applyDither(samples []float32, amplitude float32) {
	if amplitude <= 0 {
		return
	}

	state := ditherSeed
	for i := 0; i < len(samples); i += 2 {
		u1 := randomUnit(&state)
		u2 := randomUnit(&state)
		radius := float64(amplitude) * math.Sqrt(-2*math.Log(u1))
		angle := 2 * math.Pi * u2
		samples[i] = clampSample(float64(samples[i]) + radius*math.Cos(angle))
		if i+1 < len(samples) {
			samples[i+1] = clampSample(float64(samples[i+1]) + radius*math.Sin(angle))
		}
	}
}

func randomUnit(state *uint64) float64 {
	*state += 0x9e3779b97f4a7c15
	value := *state
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	value ^= value >> 31
	return float64(value>>11+1) / float64(uint64(1)<<53)
}

func clampSample(value float64) float32 {
	return float32(max(-1, min(1, value)))
}
