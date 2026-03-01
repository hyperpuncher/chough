package asr

import (
	"fmt"
	"path/filepath"

	"github.com/hyperpuncher/chough/internal/audio"
	"github.com/hyperpuncher/chough/internal/models"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Recognizer wraps Sherpa-ONNX recognizer with proper cleanup
type Recognizer struct {
	Config     *Config
	recognizer *sherpa.OfflineRecognizer
}

// NewRecognizer creates a new ASR recognizer
func NewRecognizer(cfg *Config) (*Recognizer, error) {
	sherpaConfig := sherpa.OfflineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: cfg.SampleRate,
			FeatureDim: cfg.FeatureDim,
		},
		ModelConfig: sherpa.OfflineModelConfig{
			Transducer: sherpa.OfflineTransducerModelConfig{
				Encoder: filepath.Join(cfg.ModelPath, models.EncoderFile),
				Decoder: filepath.Join(cfg.ModelPath, models.DecoderFile),
				Joiner:  filepath.Join(cfg.ModelPath, models.JoinerFile),
			},
			Tokens:     filepath.Join(cfg.ModelPath, models.TokensFile),
			NumThreads: cfg.NumThreads,
			Provider:   cfg.Provider,
			ModelType:  "nemo_transducer",
		},
	}

	recognizer := sherpa.NewOfflineRecognizer(&sherpaConfig)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create recognizer")
	}

	return &Recognizer{
		Config:     cfg,
		recognizer: recognizer,
	}, nil
}

// Transcribe transcribes an audio file
func (r *Recognizer) Transcribe(audioPath string) (*Result, error) {
	if r == nil || r.recognizer == nil {
		return nil, fmt.Errorf("recognizer not initialized")
	}

	// Read wave file using pure Go implementation (no C memory leaks!)
	wave, err := audio.ReadWave(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wave file: %w", err)
	}

	// Create stream with EXPLICIT cleanup via defer
	stream := sherpa.NewOfflineStream(r.recognizer)
	defer sherpa.DeleteOfflineStream(stream) // ← KEY: prevents memory leak!

	// Process audio
	stream.AcceptWaveform(wave.SampleRate, wave.Samples)
	r.recognizer.Decode(stream)

	// Get result
	sherpaResult := stream.GetResult()

	return &Result{
		Text:       sherpaResult.Text,
		Timestamps: sherpaResult.Timestamps,
		Tokens:     sherpaResult.Tokens,
	}, nil
}

// Close cleans up the recognizer
func (r *Recognizer) Close() {
	if r.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(r.recognizer)
		r.recognizer = nil
	}
}

// Result holds transcription result
type Result struct {
	Text       string
	Timestamps []float32
	Tokens     []string
}
