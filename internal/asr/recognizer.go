package asr

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hyperpuncher/chough/internal/audio"
	"github.com/hyperpuncher/chough/internal/models"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

// Recognizer owns the native sherpa recognizer.
type Recognizer struct {
	recognizer *sherpa.OfflineRecognizer
	dither     float32
}

func NewRecognizer(cfg Config) (*Recognizer, error) {
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

	return &Recognizer{recognizer: recognizer, dither: cfg.Dither}, nil
}

// SupportsTokenTimestamps reports that results can be trimmed at context boundaries.
func (*Recognizer) SupportsTokenTimestamps() bool {
	return true
}

func (r *Recognizer) Transcribe(audioPath string) (*Result, error) {
	if r == nil || r.recognizer == nil {
		return nil, fmt.Errorf("recognizer not initialized")
	}

	wave, err := audio.ReadWave(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wave file: %w", err)
	}

	originalSamples := append([]float32(nil), wave.Samples...)
	applyDither(wave.Samples, r.dither)
	result := r.decode(wave.SampleRate, wave.Samples)
	if r.dither > 0 && len(result.Tokens) == 0 && strings.TrimSpace(result.Text) == "" {
		result = r.decode(wave.SampleRate, originalSamples)
	}
	return result, nil
}

func (r *Recognizer) decode(sampleRate int, samples []float32) *Result {
	stream := sherpa.NewOfflineStream(r.recognizer)
	// Sherpa streams own native memory and must be deleted explicitly.
	defer sherpa.DeleteOfflineStream(stream)

	stream.AcceptWaveform(sampleRate, samples)
	r.recognizer.Decode(stream)

	sherpaResult := stream.GetResult()
	if sherpaResult == nil {
		return &Result{}
	}
	return &Result{
		Text:       sherpaResult.Text,
		Timestamps: sherpaResult.Timestamps,
		Tokens:     sherpaResult.Tokens,
		Durations:  sherpaResult.Durations,
		LogProbs:   sherpaResult.YsLogProbs,
	}
}

func (r *Recognizer) Close() {
	if r.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(r.recognizer)
		r.recognizer = nil
	}
}

type Result struct {
	Text       string
	Timestamps []float32
	Tokens     []string
	Durations  []float32
	LogProbs   []float32
}
