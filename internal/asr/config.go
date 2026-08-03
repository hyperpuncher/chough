package asr

// Config holds ASR configuration.
type Config struct {
	ModelPath  string
	NumThreads int
	SampleRate int
	FeatureDim int
	Provider   string
	Dither     float32
}

func DefaultConfig(modelPath string) Config {
	return Config{
		ModelPath:  modelPath,
		NumThreads: 4,
		SampleRate: 16000,
		FeatureDim: 80,
		Provider:   "cpu",
		Dither:     0.001,
	}
}
