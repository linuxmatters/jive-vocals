package processor

import (
	"encoding/json"
	"strconv"
	"time"
)

// This file holds the §8.4 unit-honesty conversions applied at record assembly
// (representation only). The source domain structs are never mutated: durations
// stay time.Duration (other code consumes them) and LoudnormStats keeps its
// FFmpeg string-parse shape. The record-facing types here present seconds (_s
// float) and §8.4 numeric loudnorm fields instead.

// noiseProfileRecord wraps the elected room-tone NoiseProfile for the record,
// presenting its time bounds as _s floats (§8.4). The source NoiseProfile is
// untouched.
type noiseProfileRecord struct {
	src *NoiseProfile
}

// MarshalJSON emits the record-facing room-tone profile shape directly.
func (p noiseProfileRecord) MarshalJSON() ([]byte, error) {
	if p.src == nil {
		return []byte("null"), nil
	}
	return json.Marshal(newNoiseProfileSecondsJSON(p.src))
}

func (p *noiseProfileRecord) UnmarshalJSON(data []byte) error {
	var decoded *noiseProfileSecondsJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded == nil {
		p.src = nil
		return nil
	}
	p.src = noiseProfileFromSecondsJSON(*decoded)
	return nil
}

// Profile exposes the wrapped NoiseProfile for read-only consumers (off
// rec.Regions.RoomTone.Elected). Returns nil when no profile is wrapped.
func (p *noiseProfileRecord) Profile() *NoiseProfile {
	if p == nil {
		return nil
	}
	return p.src
}

// noiseProfileJSON is the flat JSON contract for NoiseProfile: the embedded
// SpectralMetrics is unpacked into the historical spectral_* tags (distinct from
// SpectralMetrics's own mean/centroid_hz/entropy tags) so the schema is unchanged
// after the embed. Field order and tags mirror the former flat struct exactly.
type noiseProfileJSON struct {
	Start    time.Duration `json:"start"`
	Duration time.Duration `json:"duration"`
	// MeasuredNoiseFloor mirrors NoiseProfile.MeasuredNoiseFloor: seeded as astats
	// RMS, then overwritten by detectVoiceActivity with the momentary-LUFS percentile
	// floor. The measured_floor_dbfs key names dBFS but the elected value is on the
	// momentary-LUFS axis.
	MeasuredNoiseFloor jsonFloat `json:"measured_floor_dbfs"`
	PeakLevel          jsonFloat `json:"peak_level_dbfs"`
	CrestFactor        jsonFloat `json:"crest_factor_db"`
	Entropy            jsonFloat `json:"entropy"`
	ExtractionWarning  string    `json:"extraction_warning,omitempty"`

	SpectralMean     jsonFloat `json:"spectral_mean"`
	SpectralVariance jsonFloat `json:"spectral_variance"`
	SpectralCentroid jsonFloat `json:"spectral_centroid_hz"`
	SpectralSpread   jsonFloat `json:"spectral_spread_hz"`
	SpectralSkewness jsonFloat `json:"spectral_skewness"`
	SpectralKurtosis jsonFloat `json:"spectral_kurtosis"`
	SpectralEntropy  jsonFloat `json:"spectral_entropy"`
	SpectralFlatness jsonFloat `json:"spectral_flatness"`
	SpectralCrest    jsonFloat `json:"spectral_crest"`
	SpectralFlux     jsonFloat `json:"spectral_flux"`
	SpectralSlope    jsonFloat `json:"spectral_slope"`
	SpectralDecrease jsonFloat `json:"spectral_decrease"`
	SpectralRolloff  jsonFloat `json:"spectral_rolloff_hz"`

	BandNoise     []jsonFloat `json:"band_noise_dbfs,omitempty"`
	BandsMeasured bool        `json:"band_noise_measured,omitempty"`
}

type noiseProfileSecondsJSON struct {
	StartS    jsonFloat `json:"start_s"`
	DurationS jsonFloat `json:"duration_s"`

	MeasuredNoiseFloor jsonFloat `json:"measured_floor_dbfs"`
	PeakLevel          jsonFloat `json:"peak_level_dbfs"`
	CrestFactor        jsonFloat `json:"crest_factor_db"`
	Entropy            jsonFloat `json:"entropy"`
	ExtractionWarning  string    `json:"extraction_warning,omitempty"`

	SpectralMean     jsonFloat `json:"spectral_mean"`
	SpectralVariance jsonFloat `json:"spectral_variance"`
	SpectralCentroid jsonFloat `json:"spectral_centroid_hz"`
	SpectralSpread   jsonFloat `json:"spectral_spread_hz"`
	SpectralSkewness jsonFloat `json:"spectral_skewness"`
	SpectralKurtosis jsonFloat `json:"spectral_kurtosis"`
	SpectralEntropy  jsonFloat `json:"spectral_entropy"`
	SpectralFlatness jsonFloat `json:"spectral_flatness"`
	SpectralCrest    jsonFloat `json:"spectral_crest"`
	SpectralFlux     jsonFloat `json:"spectral_flux"`
	SpectralSlope    jsonFloat `json:"spectral_slope"`
	SpectralDecrease jsonFloat `json:"spectral_decrease"`
	SpectralRolloff  jsonFloat `json:"spectral_rolloff_hz"`

	BandNoise     []jsonFloat `json:"band_noise_dbfs,omitempty"`
	BandsMeasured bool        `json:"band_noise_measured,omitempty"`
}

func newNoiseProfileSecondsJSON(p *NoiseProfile) noiseProfileSecondsJSON {
	return noiseProfileSecondsJSON{
		StartS:             jsonFloat(p.Start.Seconds()),
		DurationS:          jsonFloat(p.Duration.Seconds()),
		MeasuredNoiseFloor: jsonFloat(p.MeasuredNoiseFloor),
		PeakLevel:          jsonFloat(p.PeakLevel),
		CrestFactor:        jsonFloat(p.CrestFactor),
		Entropy:            jsonFloat(p.Entropy),
		ExtractionWarning:  p.ExtractionWarning,
		SpectralMean:       jsonFloat(p.Spectral.Mean),
		SpectralVariance:   jsonFloat(p.Spectral.Variance),
		SpectralCentroid:   jsonFloat(p.Spectral.Centroid),
		SpectralSpread:     jsonFloat(p.Spectral.Spread),
		SpectralSkewness:   jsonFloat(p.Spectral.Skewness),
		SpectralKurtosis:   jsonFloat(p.Spectral.Kurtosis),
		SpectralEntropy:    jsonFloat(p.Spectral.Entropy),
		SpectralFlatness:   jsonFloat(p.Spectral.Flatness),
		SpectralCrest:      jsonFloat(p.Spectral.Crest),
		SpectralFlux:       jsonFloat(p.Spectral.Flux),
		SpectralSlope:      jsonFloat(p.Spectral.Slope),
		SpectralDecrease:   jsonFloat(p.Spectral.Decrease),
		SpectralRolloff:    jsonFloat(p.Spectral.Rolloff),
		BandNoise:          jsonFloatSlice(p.BandNoise),
		BandsMeasured:      p.BandsMeasured,
	}
}

func noiseProfileFromSecondsJSON(p noiseProfileSecondsJSON) *NoiseProfile {
	return &NoiseProfile{
		Start:              durationFromSeconds(p.StartS),
		Duration:           durationFromSeconds(p.DurationS),
		MeasuredNoiseFloor: float64(p.MeasuredNoiseFloor),
		PeakLevel:          float64(p.PeakLevel),
		CrestFactor:        float64(p.CrestFactor),
		Entropy:            float64(p.Entropy),
		ExtractionWarning:  p.ExtractionWarning,
		Spectral: SpectralMetrics{
			Mean:     float64(p.SpectralMean),
			Variance: float64(p.SpectralVariance),
			Centroid: float64(p.SpectralCentroid),
			Spread:   float64(p.SpectralSpread),
			Skewness: float64(p.SpectralSkewness),
			Kurtosis: float64(p.SpectralKurtosis),
			Entropy:  float64(p.SpectralEntropy),
			Flatness: float64(p.SpectralFlatness),
			Crest:    float64(p.SpectralCrest),
			Flux:     float64(p.SpectralFlux),
			Slope:    float64(p.SpectralSlope),
			Decrease: float64(p.SpectralDecrease),
			Rolloff:  float64(p.SpectralRolloff),
			Found:    true,
		},
		BandNoise:     floatSliceFromJSON(p.BandNoise),
		BandsMeasured: p.BandsMeasured,
	}
}

// MarshalJSON preserves the flat spectral_* JSON contract while the Go model
// carries the room-tone spectral data as an embedded SpectralMetrics value. The
// embedded value flattens into the historical spectral_* tags rather than
// SpectralMetrics's own mean/centroid_hz/entropy tags, so the run-record JSON and
// the default-marshalled noise_profile key stay byte-identical. Non-finite float
// fields serialise to null through jsonFloat.
func (p NoiseProfile) MarshalJSON() ([]byte, error) {
	flat := noiseProfileJSON{
		Start:              p.Start,
		Duration:           p.Duration,
		MeasuredNoiseFloor: jsonFloat(p.MeasuredNoiseFloor),
		PeakLevel:          jsonFloat(p.PeakLevel),
		CrestFactor:        jsonFloat(p.CrestFactor),
		Entropy:            jsonFloat(p.Entropy),
		ExtractionWarning:  p.ExtractionWarning,

		SpectralMean:     jsonFloat(p.Spectral.Mean),
		SpectralVariance: jsonFloat(p.Spectral.Variance),
		SpectralCentroid: jsonFloat(p.Spectral.Centroid),
		SpectralSpread:   jsonFloat(p.Spectral.Spread),
		SpectralSkewness: jsonFloat(p.Spectral.Skewness),
		SpectralKurtosis: jsonFloat(p.Spectral.Kurtosis),
		SpectralEntropy:  jsonFloat(p.Spectral.Entropy),
		SpectralFlatness: jsonFloat(p.Spectral.Flatness),
		SpectralCrest:    jsonFloat(p.Spectral.Crest),
		SpectralFlux:     jsonFloat(p.Spectral.Flux),
		SpectralSlope:    jsonFloat(p.Spectral.Slope),
		SpectralDecrease: jsonFloat(p.Spectral.Decrease),
		SpectralRolloff:  jsonFloat(p.Spectral.Rolloff),

		BandNoise:     jsonFloatSlice(p.BandNoise),
		BandsMeasured: p.BandsMeasured,
	}
	return json.Marshal(flat)
}

// speechProfileRecord wraps the elected speech candidate for the record. Its
// nested region (start/end/duration) and refinement bounds become _s floats. The
// source SpeechCandidateMetrics is untouched.
type speechProfileRecord struct {
	src *SpeechCandidateMetrics
}

// MarshalJSON emits the record-facing speech profile shape directly.
func (s speechProfileRecord) MarshalJSON() ([]byte, error) {
	if s.src == nil {
		return []byte("null"), nil
	}
	return json.Marshal(newSpeechProfileJSON(s.src))
}

func (s *speechProfileRecord) UnmarshalJSON(data []byte) error {
	var decoded *speechProfileJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded == nil {
		s.src = nil
		return nil
	}
	s.src = speechProfileFromJSON(*decoded)
	return nil
}

// Profile exposes the wrapped SpeechCandidateMetrics for read-only consumers (off
// rec.Regions.Speech.Elected). Returns nil when no profile is wrapped.
func (s *speechProfileRecord) Profile() *SpeechCandidateMetrics {
	if s == nil {
		return nil
	}
	return s.src
}

type speechRegionSecondsJSON struct {
	StartS    jsonFloat `json:"start_s"`
	EndS      jsonFloat `json:"end_s"`
	DurationS jsonFloat `json:"duration_s"`
}

func newSpeechRegionSecondsJSON(r SpeechRegion) speechRegionSecondsJSON {
	return speechRegionSecondsJSON{
		StartS:    jsonFloat(r.Start.Seconds()),
		EndS:      jsonFloat(r.End.Seconds()),
		DurationS: jsonFloat(r.Duration.Seconds()),
	}
}

type speechProfileJSON struct {
	Region speechRegionSecondsJSON `json:"region"`
	regionSampleJSON
	VoicingDensity    jsonFloat `json:"voicing_density,omitempty"`
	BodyBandRMS       jsonFloat `json:"speech_band_body_rms_dbfs,omitempty"`
	SibBandRMS        jsonFloat `json:"speech_band_sib_rms_dbfs,omitempty"`
	BandsMeasured     bool      `json:"speech_bands_measured,omitempty"`
	Score             jsonFloat `json:"score"`
	OriginalStartS    jsonFloat `json:"original_start_s,omitempty"`
	OriginalDurationS jsonFloat `json:"original_duration_s,omitempty"`
	WasRefined        bool      `json:"was_refined,omitempty"`
}

func newSpeechProfileJSON(s *SpeechCandidateMetrics) speechProfileJSON {
	sample := newRegionSampleJSON(&s.RegionSample)
	out := speechProfileJSON{
		Region:            newSpeechRegionSecondsJSON(s.Region),
		VoicingDensity:    jsonFloat(s.VoicingDensity),
		BodyBandRMS:       jsonFloat(s.BodyBandRMS),
		SibBandRMS:        jsonFloat(s.SibBandRMS),
		BandsMeasured:     s.BandsMeasured,
		Score:             jsonFloat(s.Score),
		OriginalStartS:    jsonFloat(s.OriginalStart.Seconds()),
		OriginalDurationS: jsonFloat(s.OriginalDuration.Seconds()),
		WasRefined:        s.WasRefined,
	}
	if sample != nil {
		out.regionSampleJSON = *sample
	}
	return out
}

func speechProfileFromJSON(s speechProfileJSON) *SpeechCandidateMetrics {
	return &SpeechCandidateMetrics{
		Region: SpeechRegion{
			Start:    durationFromSeconds(s.Region.StartS),
			End:      durationFromSeconds(s.Region.EndS),
			Duration: durationFromSeconds(s.Region.DurationS),
		},
		RegionSample:     regionSampleFromJSON(s.regionSampleJSON),
		VoicingDensity:   float64(s.VoicingDensity),
		BodyBandRMS:      float64(s.BodyBandRMS),
		SibBandRMS:       float64(s.SibBandRMS),
		BandsMeasured:    s.BandsMeasured,
		Score:            float64(s.Score),
		OriginalStart:    durationFromSeconds(s.OriginalStartS),
		OriginalDuration: durationFromSeconds(s.OriginalDurationS),
		WasRefined:       s.WasRefined,
	}
}

// normalisationRecord wraps NormalisationResult for the record. It presents the
// region-measurement duration as region_measurement_s (float seconds, §8.4) and
// converts loudnorm_measured from FFmpeg's raw string keys to the §8.4 numeric
// sub-block. The wrapped snapshot and LoudnormStats are read-only here.
type normalisationRecord struct {
	src *NormalisationResult
}

// MarshalJSON emits the record-facing normalisation shape directly.
func (n normalisationRecord) MarshalJSON() ([]byte, error) {
	if n.src == nil {
		return []byte("null"), nil
	}
	return json.Marshal(newNormalisationJSON(n.src))
}

func (n *normalisationRecord) UnmarshalJSON(data []byte) error {
	var decoded *normalisationJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded == nil {
		n.src = nil
		return nil
	}
	n.src = normalisationFromJSON(*decoded)
	return nil
}

// Result exposes the wrapped NormalisationResult for read-only consumers (off
// rec.Normalisation). Returns nil when no result is wrapped.
func (n *normalisationRecord) Result() *NormalisationResult {
	if n == nil {
		return nil
	}
	return n.src
}

type normalisationJSON struct {
	InputLUFS             jsonFloat             `json:"input_lufs"`
	InputTP               jsonFloat             `json:"input_dbtp"`
	OutputLUFS            jsonFloat             `json:"output_lufs"`
	OutputTP              jsonFloat             `json:"output_dbtp"`
	GainApplied           jsonFloat             `json:"gain_applied_db"`
	WithinTarget          bool                  `json:"within_target"`
	Skipped               bool                  `json:"skipped"`
	LoudnormMeasured      *loudnormMeasuredJSON `json:"loudnorm_measured"`
	RequestedTargetI      jsonFloat             `json:"requested_target_lufs"`
	EffectiveTargetI      jsonFloat             `json:"effective_target_lufs"`
	LinearModeForced      bool                  `json:"linear_mode_forced"`
	ActualNormDynamic     bool                  `json:"actual_norm_dynamic"`
	LimiterEnabled        bool                  `json:"limiter_enabled"`
	LimiterCeiling        jsonFloat             `json:"ceiling_dbtp"`
	LimiterGain           jsonFloat             `json:"gain_db"`
	LimiterFilteredTP     jsonFloat             `json:"filtered_dbtp"`
	PreGainDB             jsonFloat             `json:"pre_gain_db"`
	LimiterClamped        bool                  `json:"limiter_clamped"`
	Pass3FilterPrefix     string                `json:"pass3_filter_prefix"`
	RegionMeasurementTime jsonFloat             `json:"region_measurement_s"`
}

func newNormalisationJSON(n *NormalisationResult) normalisationJSON {
	return normalisationJSON{
		InputLUFS:             jsonFloat(n.InputLUFS),
		InputTP:               jsonFloat(n.InputTP),
		OutputLUFS:            jsonFloat(n.OutputLUFS),
		OutputTP:              jsonFloat(n.OutputTP),
		GainApplied:           jsonFloat(n.GainApplied),
		WithinTarget:          n.WithinTarget,
		Skipped:               n.Skipped,
		LoudnormMeasured:      newLoudnormMeasuredJSON(n.LoudnormParsed),
		RequestedTargetI:      jsonFloat(n.RequestedTargetI),
		EffectiveTargetI:      jsonFloat(n.EffectiveTargetI),
		LinearModeForced:      n.LinearModeForced,
		ActualNormDynamic:     n.ActualNormDynamic,
		LimiterEnabled:        n.LimiterEnabled,
		LimiterCeiling:        jsonFloat(n.LimiterCeiling),
		LimiterGain:           jsonFloat(n.LimiterGain),
		LimiterFilteredTP:     jsonFloat(n.LimiterFilteredTP),
		PreGainDB:             jsonFloat(n.PreGainDB),
		LimiterClamped:        n.LimiterClamped,
		Pass3FilterPrefix:     n.Pass3FilterPrefix,
		RegionMeasurementTime: jsonFloat(n.RegionMeasurementTime.Seconds()),
	}
}

func normalisationFromJSON(n normalisationJSON) *NormalisationResult {
	out := &NormalisationResult{
		InputLUFS:         float64(n.InputLUFS),
		InputTP:           float64(n.InputTP),
		OutputLUFS:        float64(n.OutputLUFS),
		OutputTP:          float64(n.OutputTP),
		GainApplied:       float64(n.GainApplied),
		WithinTarget:      n.WithinTarget,
		Skipped:           n.Skipped,
		RequestedTargetI:  float64(n.RequestedTargetI),
		EffectiveTargetI:  float64(n.EffectiveTargetI),
		LinearModeForced:  n.LinearModeForced,
		ActualNormDynamic: n.ActualNormDynamic,
		LimiterDiagnostics: LimiterDiagnostics{
			LimiterEnabled:    n.LimiterEnabled,
			LimiterCeiling:    float64(n.LimiterCeiling),
			LimiterGain:       float64(n.LimiterGain),
			LimiterFilteredTP: float64(n.LimiterFilteredTP),
			PreGainDB:         float64(n.PreGainDB),
			LimiterClamped:    n.LimiterClamped,
		},
		Pass3FilterPrefix:     n.Pass3FilterPrefix,
		RegionMeasurementTime: durationFromSeconds(n.RegionMeasurementTime),
	}
	out.LoudnormParsed = loudnormMeasuredFromJSON(n.LoudnormMeasured, out.EffectiveTargetI)
	out.LoudnormStats = loudnormStatsFromJSON(n.LoudnormMeasured)
	return out
}

type loudnormMeasuredJSON struct {
	InputI            *jsonFloat `json:"input_integrated_lufs,omitempty"`
	InputTP           *jsonFloat `json:"input_true_peak_dbtp,omitempty"`
	InputLRA          *jsonFloat `json:"input_lra_lu,omitempty"`
	InputThresh       *jsonFloat `json:"input_thresh_lufs,omitempty"`
	OutputI           *jsonFloat `json:"output_integrated_lufs,omitempty"`
	OutputTP          *jsonFloat `json:"output_true_peak_dbtp,omitempty"`
	OutputLRA         *jsonFloat `json:"output_lra_lu,omitempty"`
	OutputThresh      *jsonFloat `json:"output_thresh_lufs,omitempty"`
	TargetOffset      *jsonFloat `json:"target_offset_db,omitempty"`
	NormalizationType string     `json:"normalization_type,omitempty"`
}

func newLoudnormMeasuredJSON(measured *LoudnormMeasured) *loudnormMeasuredJSON {
	if measured == nil {
		return nil
	}
	return &loudnormMeasuredJSON{
		InputI:            loudnormValueJSON(measured.InputI),
		InputTP:           loudnormValueJSON(measured.InputTP),
		InputLRA:          loudnormValueJSON(measured.InputLRA),
		InputThresh:       loudnormValueJSON(measured.InputThresh),
		OutputI:           loudnormValueJSON(measured.OutputI),
		OutputTP:          loudnormValueJSON(measured.OutputTP),
		OutputLRA:         loudnormValueJSON(measured.OutputLRA),
		OutputThresh:      loudnormValueJSON(measured.OutputThresh),
		TargetOffset:      loudnormValueJSON(measured.TargetOffset),
		NormalizationType: measured.NormalizationType,
	}
}

func loudnormMeasuredFromJSON(m *loudnormMeasuredJSON, effectiveTargetI float64) *LoudnormMeasured {
	if m == nil {
		return nil
	}
	out := &LoudnormMeasured{
		InputI:            loudnormValueFromJSON(m.InputI),
		InputTP:           loudnormValueFromJSON(m.InputTP),
		InputLRA:          loudnormValueFromJSON(m.InputLRA),
		InputThresh:       loudnormValueFromJSON(m.InputThresh),
		OutputI:           loudnormValueFromJSON(m.OutputI),
		OutputTP:          loudnormValueFromJSON(m.OutputTP),
		OutputLRA:         loudnormValueFromJSON(m.OutputLRA),
		OutputThresh:      loudnormValueFromJSON(m.OutputThresh),
		TargetOffset:      loudnormValueFromJSON(m.TargetOffset),
		NormalizationType: m.NormalizationType,
	}
	if out.OutputI.OK {
		out.TargetDeviation = LoudnormValue{Value: out.OutputI.Value - effectiveTargetI, OK: true}
	}
	return out
}

func loudnormStatsFromJSON(m *loudnormMeasuredJSON) *LoudnormStats {
	if m == nil {
		return nil
	}
	return &LoudnormStats{
		InputI:            loudnormStringFromJSON(m.InputI),
		InputTP:           loudnormStringFromJSON(m.InputTP),
		InputLRA:          loudnormStringFromJSON(m.InputLRA),
		InputThresh:       loudnormStringFromJSON(m.InputThresh),
		OutputI:           loudnormStringFromJSON(m.OutputI),
		OutputTP:          loudnormStringFromJSON(m.OutputTP),
		OutputLRA:         loudnormStringFromJSON(m.OutputLRA),
		OutputThresh:      loudnormStringFromJSON(m.OutputThresh),
		TargetOffset:      loudnormStringFromJSON(m.TargetOffset),
		NormalizationType: m.NormalizationType,
	}
}

func loudnormValueJSON(value LoudnormValue) *jsonFloat {
	if !value.OK {
		return nil
	}
	out := jsonFloat(value.Value)
	return &out
}

func durationFromSeconds(seconds jsonFloat) time.Duration {
	return time.Duration(float64(seconds) * float64(time.Second))
}

func floatSliceFromJSON(values []jsonFloat) []float64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = float64(v)
	}
	return out
}

func loudnormValueFromJSON(value *jsonFloat) LoudnormValue {
	if value == nil {
		return LoudnormValue{}
	}
	return LoudnormValue{Value: float64(*value), OK: true}
}

func loudnormStringFromJSON(value *jsonFloat) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(float64(*value), 'f', -1, 64)
}

func regionSampleFromJSON(s regionSampleJSON) RegionSample {
	return RegionSample{
		RMSLevel:      float64(s.RMSLevel),
		PeakLevel:     float64(s.PeakLevel),
		CrestFactor:   float64(s.CrestFactor),
		Spectral:      spectralMetricsFromJSON(s.Spectral),
		MomentaryLUFS: float64(s.MomentaryLUFS),
		ShortTermLUFS: float64(s.ShortTermLUFS),
		TruePeak:      float64(s.TruePeak),
		SamplePeak:    float64(s.SamplePeak),
	}
}

func spectralMetricsFromJSON(s *spectralMetricsJSON) SpectralMetrics {
	if s == nil {
		return SpectralMetrics{}
	}
	return SpectralMetrics{
		Mean:     float64(s.Mean),
		Variance: float64(s.Variance),
		Centroid: float64(s.Centroid),
		Spread:   float64(s.Spread),
		Skewness: float64(s.Skewness),
		Kurtosis: float64(s.Kurtosis),
		Entropy:  float64(s.Entropy),
		Flatness: float64(s.Flatness),
		Crest:    float64(s.Crest),
		Flux:     float64(s.Flux),
		Slope:    float64(s.Slope),
		Decrease: float64(s.Decrease),
		Rolloff:  float64(s.Rolloff),
		Found:    true,
	}
}
