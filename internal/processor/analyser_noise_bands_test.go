package processor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNoiseBandProfileEligibleForCustomAfftdnSkipReasons(t *testing.T) {
	tests := []struct {
		name        string
		measure     func() *AudioMeasurements
		wantProfile bool
	}{
		{
			name: "missing measurements",
			measure: func() *AudioMeasurements {
				return nil
			},
		},
		{
			name: "no noise profile",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.NoiseProfile = nil
				return m
			},
		},
		{
			name: "empty duration",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.NoiseProfile.Duration = 0
				return m
			},
			wantProfile: true,
		},
		{
			name: "voice activated",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Noise.VoiceActivated = true
				return m
			},
			wantProfile: true,
		},
		{
			name: "zero noise floor",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Noise.Floor = 0
				return m
			},
			wantProfile: true,
		},
		{
			name: "low gate separation",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.GateSeparationDB = afftdnCustomMinSeparationDB - 0.1
				return m
			},
			wantProfile: true,
		},
		{
			name: "low flatness",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.NoiseProfile.Spectral.Flatness = afftdnCustomMinFlatness - 0.01
				return m
			},
			wantProfile: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile, eligible := noiseBandProfileEligibleForCustomAfftdn(tc.measure())
			if eligible {
				t.Fatalf("eligible = true, want false")
			}
			if (profile != nil) != tc.wantProfile {
				t.Fatalf("profile returned = %v, want profile presence %v", profile != nil, tc.wantProfile)
			}
		})
	}
}

func TestNoiseBandProfileEligibleForCustomAfftdnAcceptsKnownGoodProfile(t *testing.T) {
	profile, eligible := noiseBandProfileEligibleForCustomAfftdn(noiseBandEligibleMeasurementsForTest())
	if !eligible {
		t.Fatal("eligible = false, want true")
	}
	if profile == nil {
		t.Fatal("profile = nil, want noise profile")
	}
}

func TestNoiseBandProfileEligibleForCustomAfftdnIgnoresBandsMeasured(t *testing.T) {
	m := noiseBandEligibleMeasurementsForTest()
	m.Regions.NoiseProfile.BandNoise = nil
	m.Regions.NoiseProfile.BandsMeasured = false

	profile, eligible := noiseBandProfileEligibleForCustomAfftdn(m)
	if !eligible {
		t.Fatal("eligible = false, want true")
	}
	if profile == nil {
		t.Fatal("profile = nil, want noise profile")
	}
}

func TestMeasureNoiseBandsSkipDrainsProgressAndClearsProfile(t *testing.T) {
	tests := []struct {
		name    string
		measure func() *AudioMeasurements
	}{
		{
			name: "missing measurements",
			measure: func() *AudioMeasurements {
				return nil
			},
		},
		{
			name: "no noise profile",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.NoiseProfile = nil
				return m
			},
		},
		{
			name: "empty duration",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.NoiseProfile.Duration = 0
				return m
			},
		},
		{
			name: "voice activated",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Noise.VoiceActivated = true
				return m
			},
		},
		{
			name: "zero noise floor",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Noise.Floor = 0
				return m
			},
		},
		{
			name: "low gate separation",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.GateSeparationDB = afftdnCustomMinSeparationDB - 0.1
				return m
			},
		},
		{
			name: "low flatness",
			measure: func() *AudioMeasurements {
				m := noiseBandEligibleMeasurementsForTest()
				m.Regions.NoiseProfile.Spectral.Flatness = afftdnCustomMinFlatness - 0.01
				return m
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.measure()
			var reported int64

			measureNoiseBands(context.Background(), "unused-input.flac", m, func() {
				atomic.AddInt64(&reported, 1)
			}, nil)

			if int(reported) != len(afftdnBandCentresHz) {
				t.Fatalf("reported = %d, want %d", reported, len(afftdnBandCentresHz))
			}
			if m == nil || m.Regions.NoiseProfile == nil {
				return
			}
			if m.Regions.NoiseProfile.BandNoise != nil {
				t.Fatalf("BandNoise = %v, want nil", m.Regions.NoiseProfile.BandNoise)
			}
			if m.Regions.NoiseProfile.BandsMeasured {
				t.Fatal("BandsMeasured = true, want false")
			}
		})
	}
}

func noiseBandEligibleMeasurementsForTest() *AudioMeasurements {
	return &AudioMeasurements{
		Noise: NoiseMetrics{Floor: -58.0},
		Regions: RegionMetrics{
			GateSeparationDB: afftdnCustomMinSeparationDB + 1.0,
			NoiseProfile: &NoiseProfile{
				Duration:      time.Second,
				Spectral:      SpectralMetrics{Flatness: afftdnCustomMinFlatness + 0.01},
				BandNoise:     []float64{-61.0, -60.0, -59.0},
				BandsMeasured: true,
			},
		},
	}
}
