package report

import (
	"strings"
	"testing"
)

// TestRequiredKeysHaveDefinitions pins the loudness + dynamics + spectral key set
// the renderer emits: every required key MUST resolve to a definition, so a
// missing or renamed entry fails here rather than producing a blank report cell.
func TestRequiredKeysHaveDefinitions(t *testing.T) {
	for _, key := range requiredKeys {
		if _, ok := Definitions[string(key)]; !ok {
			t.Errorf("required key %q has no definition", key)
		}
	}
}

// TestSpectralThirteenCovered asserts all 13 aspectralstats fields are defined,
// guarding the spectral section against a dropped metric.
func TestSpectralThirteenCovered(t *testing.T) {
	if len(spectralMetricDescriptors) != 13 {
		t.Fatalf("spectral set has %d keys, want 13", len(spectralMetricDescriptors))
	}
	for _, metric := range spectralMetricDescriptors {
		if _, ok := Definitions[string(metric.key)]; !ok {
			t.Errorf("spectral key %q has no definition", metric.key)
		}
	}
}

// TestDefinitionsNonEmptyLabelAndGloss asserts every catalogue entry carries a
// label and gloss. Unit may be empty (dimensionless ratios), so it is not pinned
// non-empty.
func TestDefinitionsNonEmptyLabelAndGloss(t *testing.T) {
	for key, def := range Definitions {
		if strings.TrimSpace(def.Label) == "" {
			t.Errorf("definition %q has empty label", key)
		}
		if strings.TrimSpace(def.Gloss) == "" {
			t.Errorf("definition %q has empty gloss", key)
		}
	}
}

// TestRequiredKeysCarryUnitWhereDimensioned asserts the dimensioned required
// metrics carry a non-empty unit. Dimensionless ratios (flat_factor, dc_offset,
// zero_crossings_rate, entropy, and the bare spectral moments) carry no unit by
// design, so they are excluded.
func TestRequiredKeysCarryUnitWhereDimensioned(t *testing.T) {
	dimensionless := map[metricKey]bool{
		flatFactorMetric.key:        true,
		dcOffsetMetric.key:          true,
		zeroCrossingsRateMetric.key: true,
		entropyMetric.key:           true,
		meanMetric.key:              true,
		varianceMetric.key:          true,
		skewnessMetric.key:          true,
		kurtosisMetric.key:          true,
		flatnessMetric.key:          true,
		crestMetric.key:             true,
		fluxMetric.key:              true,
		slopeMetric.key:             true,
		decreaseMetric.key:          true,
		floorSourceMetric.key:       true,
		voiceActivatedMetric.key:    true,
		flooredFractionMetric.key:   true,
		spectralFlatnessMetric.key:  true,
		spectralKurtosisMetric.key:  true,
		voicingDensityMetric.key:    true,
		scoreMetric.key:             true,
	}
	for _, key := range requiredKeys {
		if dimensionless[key] {
			continue
		}
		def := Definitions[string(key)]
		if strings.TrimSpace(def.Unit) == "" {
			t.Errorf("required dimensioned key %q has empty unit", key)
		}
	}
}

// TestNoRangeToMeaningTokens grep-asserts no gloss leaks a quality or
// range-to-meaning verdict. The catalogue is objective by mandate: definitions
// only, never interpretation.
func TestNoRangeToMeaningTokens(t *testing.T) {
	banned := []string{
		"warm", "bright", "good", "tonal", "broadband",
		"clean", "damaged", "harsh",
	}
	for key, def := range Definitions {
		lower := strings.ToLower(def.Gloss + " " + def.Label)
		for _, token := range banned {
			if strings.Contains(lower, token) {
				t.Errorf("definition %q contains banned range-to-meaning token %q: %q",
					key, token, def.Gloss)
			}
		}
	}
}
