// Package processor handles audio analysis and processing
package processor

import (
	"math"
)

// AdaptConfig tunes all filter parameters based on Pass 1 measurements.
// This is the main entry point for adaptive configuration.
// It returns per-file effective config and diagnostics without mutating the caller's base seed.
func AdaptConfig(config *BaseFilterConfig, measurements *AudioMeasurements) (*EffectiveFilterConfig, *AdaptiveDiagnostics) {
	effectiveConfig := deriveEffectiveFilterConfig(config)
	if effectiveConfig == nil {
		return nil, nil
	}
	diagnostics := &AdaptiveDiagnostics{}

	// Tune each filter adaptively based on measurements
	// Order matters: gate threshold calculated BEFORE denoise filters
	// The rumble highpass is fixed (80 Hz, 12 dB/oct) from defaultRumbleHighPassConfig; no tuning step.
	tuneBandlimitLowPass(effectiveConfig, diagnostics, measurements) // Unconditional 20.5 kHz band-limit

	// NoiseReduction (anlmdn + afftdn) has no adaptive tuning: anlmdn is fixed from
	// spike validation and afftdn nr is fixed at 12 to avoid warble.

	tuneSpeechGate(effectiveConfig, diagnostics, measurements) // Soft expander gate cleaning inter-speech gaps
	tuneDeesser(effectiveConfig, measurements)
	tuneLevellingCompressor(effectiveConfig, diagnostics, measurements, config.logger)
	// The limiter lives in Pass 4 and is tuned from Pass 3 measurements, not here.

	// Final safety checks
	sanitizeConfig(effectiveConfig)

	return effectiveConfig, diagnostics
}

// sanitizeConfig ensures no NaN or Inf values remain after adaptive tuning.
func sanitizeConfig(config *EffectiveFilterConfig) {
	sanitizeBiquadConfig(&config.RumbleHighPass, rumbleHPDefaultFreq)
	sanitizeBiquadConfig(&config.BandlimitLowPass, bandlimitLPFreq)
	sanitizeNoiseReductionConfig(&config.NoiseReduction)
	sanitizeSpeechGateConfig(&config.SpeechGate)
	sanitizeLevellingCompressorConfig(&config.LevellingCompressor)
	sanitizeDeesserConfig(&config.Deesser)
}

func sanitizeBiquadConfig(config *BiquadFilterConfig, defaultFreq float64) {
	config.Frequency = sanitizeFloat(config.Frequency, defaultFreq)
	config.Width = sanitizeFloat(config.Width, 0.707)
	config.Mix = sanitizeFloat(config.Mix, 1.0)
}

func sanitizeNoiseReductionConfig(config *NoiseReductionConfig) {
	defaults := defaultNoiseReductionConfig()
	config.Strength = sanitizeFloat(config.Strength, defaults.Strength)
	config.PatchSec = sanitizeFloat(config.PatchSec, defaults.PatchSec)
	config.ResearchSec = sanitizeFloat(config.ResearchSec, defaults.ResearchSec)
	config.Smooth = sanitizeFloat(config.Smooth, defaults.Smooth)
	config.AfftdnNoiseReduction = sanitizeFloat(config.AfftdnNoiseReduction, defaults.AfftdnNoiseReduction)
}

func sanitizeSpeechGateConfig(config *SpeechGateConfig) {
	defaults := defaultSpeechGateConfig()
	if math.IsNaN(config.Threshold) || math.IsInf(config.Threshold, 0) || config.Threshold <= 0 {
		config.Threshold = speechGateDefaultThreshold
	}
	config.Ratio = sanitizeFloat(config.Ratio, defaults.Ratio)
	config.Attack = sanitizeFloat(config.Attack, defaults.Attack)
	config.Release = sanitizeFloat(config.Release, defaults.Release)
	config.Range = sanitizeFloat(config.Range, defaults.Range)
	config.Knee = sanitizeFloat(config.Knee, defaults.Knee)
	config.Makeup = sanitizeFloat(config.Makeup, defaults.Makeup)
}

func sanitizeLevellingCompressorConfig(config *LevellingCompressorConfig) {
	defaults := defaultLevellingCompressorConfig()
	config.Ratio = sanitizeFloat(config.Ratio, defaults.Ratio)
	config.Threshold = sanitizeFloat(config.Threshold, defaultLevellingCompressorThreshold)
	config.Attack = sanitizeFloat(config.Attack, defaults.Attack)
	config.Release = sanitizeFloat(config.Release, defaults.Release)
	config.Makeup = sanitizeFloat(config.Makeup, defaults.Makeup)
	config.Knee = sanitizeFloat(config.Knee, defaults.Knee)
	config.Mix = sanitizeFloat(config.Mix, defaults.Mix)
}

func sanitizeDeesserConfig(config *DeesserConfig) {
	defaults := defaultDeesserConfig()
	config.Intensity = sanitizeFloat(config.Intensity, defaultDeessIntensity)
	config.Amount = sanitizeFloat(config.Amount, defaults.Amount)
	config.Frequency = sanitizeFloat(config.Frequency, defaults.Frequency)
}
