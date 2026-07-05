package processor

import (
	"encoding/json"
	"math"
	"strconv"
)

type jsonFloat float64

func (f jsonFloat) MarshalJSON() ([]byte, error) {
	v := float64(f)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return []byte("null"), nil
	}
	return strconv.AppendFloat(nil, v, 'f', -1, 64), nil
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func jsonFloatPtr(v *float64) *jsonFloat {
	if v == nil {
		return nil
	}
	out := jsonFloat(*v)
	return &out
}

func jsonFloatSlice(values []float64) []jsonFloat {
	if len(values) == 0 {
		return nil
	}
	out := make([]jsonFloat, len(values))
	for i, v := range values {
		out[i] = jsonFloat(v)
	}
	return out
}

type runRecordJSON struct {
	SchemaVersion   int                  `json:"schema_version"`
	Run             runProvenanceJSON    `json:"run"`
	Loudness        loudnessDomainJSON   `json:"loudness"`
	Dynamics        dynamicsDomainJSON   `json:"dynamics"`
	Spectral        spectralDomainJSON   `json:"spectral"`
	Noise           *noiseMetricsJSON    `json:"noise,omitempty"`
	Regions         *regionsBlockJSON    `json:"regions,omitempty"`
	Filters         *filtersBlockJSON    `json:"filters,omitempty"`
	Normalisation   *normalisationRecord `json:"normalisation,omitempty"`
	IntervalSummary *intervalSummaryJSON `json:"interval_summary,omitempty"`
	Spectrograms    []SpectrogramImage   `json:"spectrograms,omitempty"`
}

func newRunRecordJSON(r *RunRecord) *runRecordJSON {
	if r == nil {
		return nil
	}
	return &runRecordJSON{
		SchemaVersion:   r.SchemaVersion,
		Run:             newRunProvenanceJSON(r.Run),
		Loudness:        newLoudnessDomainJSON(r.Loudness),
		Dynamics:        newDynamicsDomainJSON(r.Dynamics),
		Spectral:        newSpectralDomainJSON(r.Spectral),
		Noise:           newNoiseMetricsJSON(r.Noise),
		Regions:         newRegionsBlockJSON(r.Regions),
		Filters:         newFiltersBlockJSON(r.Filters),
		Normalisation:   r.Normalisation,
		IntervalSummary: newIntervalSummaryJSON(r.IntervalSummary),
		Spectrograms:    r.Spectrograms,
	}
}

type runProvenanceJSON struct {
	InputFile    string    `json:"input_file"`
	Version      string    `json:"version"`
	Executable   string    `json:"executable"`
	ProcessedAt  string    `json:"processed_at"`
	DurationS    jsonFloat `json:"duration_s"`
	SampleRateHz int       `json:"sample_rate_hz"`
	Channels     int       `json:"channels"`
}

func newRunProvenanceJSON(r RunProvenance) runProvenanceJSON {
	return runProvenanceJSON{
		InputFile:    r.InputFile,
		Version:      r.Version,
		Executable:   r.Executable,
		ProcessedAt:  r.ProcessedAt,
		DurationS:    jsonFloat(r.DurationS),
		SampleRateHz: r.SampleRateHz,
		Channels:     r.Channels,
	}
}

type loudnessDomainJSON struct {
	TargetILUFS jsonFloat          `json:"target_i_lufs"`
	Stages      loudnessStagesJSON `json:"stages"`
}

type loudnessStagesJSON struct {
	Input    *inputLoudnessMetricsJSON  `json:"input,omitempty"`
	Filtered *outputLoudnessMetricsJSON `json:"filtered,omitempty"`
	Final    *outputLoudnessMetricsJSON `json:"final,omitempty"`
}

func newLoudnessDomainJSON(d LoudnessDomain) loudnessDomainJSON {
	return loudnessDomainJSON{
		TargetILUFS: jsonFloat(d.TargetILUFS),
		Stages: loudnessStagesJSON{
			Input:    newInputLoudnessMetricsJSON(d.Stages.Input),
			Filtered: newOutputLoudnessMetricsJSON(d.Stages.Filtered),
			Final:    newOutputLoudnessMetricsJSON(d.Stages.Final),
		},
	}
}

type loudnessMetricsJSON struct {
	MomentaryLoudness jsonFloat `json:"momentary_lufs"`
	ShortTermLoudness jsonFloat `json:"short_term_lufs"`
	SamplePeak        jsonFloat `json:"sample_peak_dbfs"`
}

func newLoudnessMetricsJSON(m LoudnessMetrics) loudnessMetricsJSON {
	return loudnessMetricsJSON{
		MomentaryLoudness: jsonFloat(m.MomentaryLoudness),
		ShortTermLoudness: jsonFloat(m.ShortTermLoudness),
		SamplePeak:        jsonFloat(m.SamplePeak),
	}
}

type inputLoudnessMetricsJSON struct {
	LoudnessMetrics loudnessMetricsJSON
	InputI          jsonFloat `json:"integrated_lufs"`
	InputTP         jsonFloat `json:"true_peak_dbtp"`
	InputLRA        jsonFloat `json:"lra_lu"`
	InputThresh     jsonFloat `json:"thresh_lufs"`
	TargetOffset    jsonFloat `json:"target_offset_db"`
}

func newInputLoudnessMetricsJSON(m *InputLoudnessMetrics) *inputLoudnessMetricsJSON {
	if m == nil {
		return nil
	}
	return &inputLoudnessMetricsJSON{
		LoudnessMetrics: newLoudnessMetricsJSON(m.LoudnessMetrics),
		InputI:          jsonFloat(m.InputI),
		InputTP:         jsonFloat(m.InputTP),
		InputLRA:        jsonFloat(m.InputLRA),
		InputThresh:     jsonFloat(m.InputThresh),
		TargetOffset:    jsonFloat(m.TargetOffset),
	}
}

func (m inputLoudnessMetricsJSON) MarshalJSON() ([]byte, error) {
	type flat struct {
		MomentaryLoudness jsonFloat `json:"momentary_lufs"`
		ShortTermLoudness jsonFloat `json:"short_term_lufs"`
		SamplePeak        jsonFloat `json:"sample_peak_dbfs"`
		InputI            jsonFloat `json:"integrated_lufs"`
		InputTP           jsonFloat `json:"true_peak_dbtp"`
		InputLRA          jsonFloat `json:"lra_lu"`
		InputThresh       jsonFloat `json:"thresh_lufs"`
		TargetOffset      jsonFloat `json:"target_offset_db"`
	}
	return marshalJSON(flat{
		MomentaryLoudness: m.LoudnessMetrics.MomentaryLoudness,
		ShortTermLoudness: m.LoudnessMetrics.ShortTermLoudness,
		SamplePeak:        m.LoudnessMetrics.SamplePeak,
		InputI:            m.InputI,
		InputTP:           m.InputTP,
		InputLRA:          m.InputLRA,
		InputThresh:       m.InputThresh,
		TargetOffset:      m.TargetOffset,
	})
}

type outputLoudnessMetricsJSON struct {
	LoudnessMetrics loudnessMetricsJSON
	OutputI         jsonFloat `json:"integrated_lufs"`
	OutputTP        jsonFloat `json:"true_peak_dbtp"`
	OutputLRA       jsonFloat `json:"lra_lu"`
	OutputThresh    jsonFloat `json:"thresh_lufs"`
	TargetOffset    jsonFloat `json:"target_offset_db"`
}

func newOutputLoudnessMetricsJSON(m *OutputLoudnessMetrics) *outputLoudnessMetricsJSON {
	if m == nil {
		return nil
	}
	return &outputLoudnessMetricsJSON{
		LoudnessMetrics: newLoudnessMetricsJSON(m.LoudnessMetrics),
		OutputI:         jsonFloat(m.OutputI),
		OutputTP:        jsonFloat(m.OutputTP),
		OutputLRA:       jsonFloat(m.OutputLRA),
		OutputThresh:    jsonFloat(m.OutputThresh),
		TargetOffset:    jsonFloat(m.TargetOffset),
	}
}

func (m outputLoudnessMetricsJSON) MarshalJSON() ([]byte, error) {
	type flat struct {
		MomentaryLoudness jsonFloat `json:"momentary_lufs"`
		ShortTermLoudness jsonFloat `json:"short_term_lufs"`
		SamplePeak        jsonFloat `json:"sample_peak_dbfs"`
		OutputI           jsonFloat `json:"integrated_lufs"`
		OutputTP          jsonFloat `json:"true_peak_dbtp"`
		OutputLRA         jsonFloat `json:"lra_lu"`
		OutputThresh      jsonFloat `json:"thresh_lufs"`
		TargetOffset      jsonFloat `json:"target_offset_db"`
	}
	return marshalJSON(flat{
		MomentaryLoudness: m.LoudnessMetrics.MomentaryLoudness,
		ShortTermLoudness: m.LoudnessMetrics.ShortTermLoudness,
		SamplePeak:        m.LoudnessMetrics.SamplePeak,
		OutputI:           m.OutputI,
		OutputTP:          m.OutputTP,
		OutputLRA:         m.OutputLRA,
		OutputThresh:      m.OutputThresh,
		TargetOffset:      m.TargetOffset,
	})
}

type dynamicsDomainJSON struct {
	Stages dynamicsStagesJSON `json:"stages"`
}

type dynamicsStagesJSON struct {
	Input    *dynamicsMetricsJSON `json:"input,omitempty"`
	Filtered *dynamicsMetricsJSON `json:"filtered,omitempty"`
	Final    *dynamicsMetricsJSON `json:"final,omitempty"`
}

func newDynamicsDomainJSON(d DynamicsDomain) dynamicsDomainJSON {
	return dynamicsDomainJSON{
		Stages: dynamicsStagesJSON{
			Input:    newDynamicsMetricsJSON(d.Stages.Input),
			Filtered: newDynamicsMetricsJSON(d.Stages.Filtered),
			Final:    newDynamicsMetricsJSON(d.Stages.Final),
		},
	}
}

type dynamicsMetricsJSON struct {
	DynamicRange      jsonFloat `json:"dynamic_range_db"`
	RMSLevel          jsonFloat `json:"rms_level_dbfs"`
	PeakLevel         jsonFloat `json:"peak_level_dbfs"`
	RMSTrough         jsonFloat `json:"rms_trough_dbfs"`
	RMSPeak           jsonFloat `json:"rms_peak_dbfs"`
	DCOffset          jsonFloat `json:"dc_offset"`
	FlatFactor        jsonFloat `json:"flat_factor"`
	CrestFactor       jsonFloat `json:"crest_factor_astats_db"`
	ZeroCrossingsRate jsonFloat `json:"zero_crossings_rate"`
	ZeroCrossings     jsonFloat `json:"zero_crossings_count"`
	MaxDifference     jsonFloat `json:"max_difference"`
	MinDifference     jsonFloat `json:"min_difference"`
	MeanDifference    jsonFloat `json:"mean_difference"`
	RMSDifference     jsonFloat `json:"rms_difference"`
	Entropy           jsonFloat `json:"entropy"`
	MinLevel          jsonFloat `json:"min_level_dbfs"`
	MaxLevel          jsonFloat `json:"max_level_dbfs"`
	NoiseFloorCount   jsonFloat `json:"noise_floor_count"`
	BitDepth          jsonFloat `json:"bit_depth"`
	NumberOfSamples   jsonFloat `json:"number_of_samples"`
}

func newDynamicsMetricsJSON(m *DynamicsMetrics) *dynamicsMetricsJSON {
	if m == nil {
		return nil
	}
	return &dynamicsMetricsJSON{
		DynamicRange:      jsonFloat(m.DynamicRange),
		RMSLevel:          jsonFloat(m.RMSLevel),
		PeakLevel:         jsonFloat(m.PeakLevel),
		RMSTrough:         jsonFloat(m.RMSTrough),
		RMSPeak:           jsonFloat(m.RMSPeak),
		DCOffset:          jsonFloat(m.DCOffset),
		FlatFactor:        jsonFloat(m.FlatFactor),
		CrestFactor:       jsonFloat(m.CrestFactor),
		ZeroCrossingsRate: jsonFloat(m.ZeroCrossingsRate),
		ZeroCrossings:     jsonFloat(m.ZeroCrossings),
		MaxDifference:     jsonFloat(m.MaxDifference),
		MinDifference:     jsonFloat(m.MinDifference),
		MeanDifference:    jsonFloat(m.MeanDifference),
		RMSDifference:     jsonFloat(m.RMSDifference),
		Entropy:           jsonFloat(m.Entropy),
		MinLevel:          jsonFloat(m.MinLevel),
		MaxLevel:          jsonFloat(m.MaxLevel),
		NoiseFloorCount:   jsonFloat(m.NoiseFloorCount),
		BitDepth:          jsonFloat(m.BitDepth),
		NumberOfSamples:   jsonFloat(m.NumberOfSamples),
	}
}

type spectralDomainJSON struct {
	Stages spectralStagesJSON `json:"stages"`
}

type spectralStagesJSON struct {
	Input    *spectralMetricsJSON `json:"input,omitempty"`
	Filtered *spectralMetricsJSON `json:"filtered,omitempty"`
	Final    *spectralMetricsJSON `json:"final,omitempty"`
}

func newSpectralDomainJSON(s SpectralDomain) spectralDomainJSON {
	return spectralDomainJSON{
		Stages: spectralStagesJSON{
			Input:    newSpectralMetricsJSON(s.Stages.Input),
			Filtered: newSpectralMetricsJSON(s.Stages.Filtered),
			Final:    newSpectralMetricsJSON(s.Stages.Final),
		},
	}
}

type spectralMetricsJSON struct {
	Mean     jsonFloat `json:"mean"`
	Variance jsonFloat `json:"variance"`
	Centroid jsonFloat `json:"centroid_hz"`
	Spread   jsonFloat `json:"spread_hz"`
	Skewness jsonFloat `json:"skewness"`
	Kurtosis jsonFloat `json:"kurtosis"`
	Entropy  jsonFloat `json:"entropy"`
	Flatness jsonFloat `json:"flatness"`
	Crest    jsonFloat `json:"crest"`
	Flux     jsonFloat `json:"flux"`
	Slope    jsonFloat `json:"slope"`
	Decrease jsonFloat `json:"decrease"`
	Rolloff  jsonFloat `json:"rolloff_hz"`
}

func newSpectralMetricsJSON(m *SpectralMetrics) *spectralMetricsJSON {
	if m == nil {
		return nil
	}
	return &spectralMetricsJSON{
		Mean:     jsonFloat(m.Mean),
		Variance: jsonFloat(m.Variance),
		Centroid: jsonFloat(m.Centroid),
		Spread:   jsonFloat(m.Spread),
		Skewness: jsonFloat(m.Skewness),
		Kurtosis: jsonFloat(m.Kurtosis),
		Entropy:  jsonFloat(m.Entropy),
		Flatness: jsonFloat(m.Flatness),
		Crest:    jsonFloat(m.Crest),
		Flux:     jsonFloat(m.Flux),
		Slope:    jsonFloat(m.Slope),
		Decrease: jsonFloat(m.Decrease),
		Rolloff:  jsonFloat(m.Rolloff),
	}
}

type noiseMetricsJSON struct {
	Floor               jsonFloat `json:"floor_dbfs"`
	FloorSource         string    `json:"floor_source"`
	FloorPrescan        jsonFloat `json:"floor_prescan_dbfs"`
	FloorAstats         jsonFloat `json:"floor_astats_dbfs"`
	RoomToneDetectLevel jsonFloat `json:"room_tone_detect_level_dbfs"`
	VoiceActivated      bool      `json:"voice_activated"`
	FlooredFraction     jsonFloat `json:"floored_fraction"`
	ReductionHeadroom   jsonFloat `json:"reduction_headroom_db"`
}

func newNoiseMetricsJSON(n *NoiseMetrics) *noiseMetricsJSON {
	if n == nil {
		return nil
	}
	return &noiseMetricsJSON{
		Floor:               jsonFloat(n.Floor),
		FloorSource:         n.FloorSource,
		FloorPrescan:        jsonFloat(n.FloorPrescan),
		FloorAstats:         jsonFloat(n.FloorAstats),
		RoomToneDetectLevel: jsonFloat(n.RoomToneDetectLevel),
		VoiceActivated:      n.VoiceActivated,
		FlooredFraction:     jsonFloat(n.FlooredFraction),
		ReductionHeadroom:   jsonFloat(n.ReductionHeadroom),
	}
}

type regionsBlockJSON struct {
	RoomTone       roomToneRegionRecordJSON `json:"room_tone"`
	Speech         speechRegionRecordJSON   `json:"speech"`
	GateStatistics *gateStatisticsJSON      `json:"gate_statistics,omitempty"`
}

func newRegionsBlockJSON(r *RegionsBlock) *regionsBlockJSON {
	if r == nil {
		return nil
	}
	return &regionsBlockJSON{
		RoomTone: roomToneRegionRecordJSON{
			Elected:           r.RoomTone.Elected,
			CandidatesSummary: newCandidatesSummaryJSON(r.RoomTone.CandidatesSummary),
			Samples:           newRegionSamplesJSON(r.RoomTone.Samples),
		},
		Speech: speechRegionRecordJSON{
			Elected:           r.Speech.Elected,
			CandidatesSummary: newCandidatesSummaryJSON(r.Speech.CandidatesSummary),
			Samples:           newRegionSamplesJSON(r.Speech.Samples),
		},
		GateStatistics: newGateStatisticsJSON(r.GateStatistics),
	}
}

type roomToneRegionRecordJSON struct {
	Elected           *noiseProfileRecord    `json:"elected,omitempty"`
	CandidatesSummary *candidatesSummaryJSON `json:"candidates_summary,omitempty"`
	Samples           regionSamplesJSON      `json:"samples"`
}

type speechRegionRecordJSON struct {
	Elected           *speechProfileRecord   `json:"elected,omitempty"`
	CandidatesSummary *candidatesSummaryJSON `json:"candidates_summary,omitempty"`
	Samples           regionSamplesJSON      `json:"samples"`
}

type candidatesSummaryJSON struct {
	EvaluatedCount int        `json:"evaluated_count"`
	ElectedScore   *jsonFloat `json:"elected_score,omitempty"`
}

func newCandidatesSummaryJSON(s *CandidatesSummary) *candidatesSummaryJSON {
	if s == nil {
		return nil
	}
	return &candidatesSummaryJSON{EvaluatedCount: s.EvaluatedCount, ElectedScore: jsonFloatPtr(s.ElectedScore)}
}

type gateStatisticsJSON struct {
	VoicedLowPercentile jsonFloat `json:"voiced_low_percentile_dbfs"`
	NoiseHighPercentile jsonFloat `json:"noise_high_percentile_dbfs"`
	SeparationDB        jsonFloat `json:"gate_separation_db"`
}

func newGateStatisticsJSON(g *GateStatistics) *gateStatisticsJSON {
	if g == nil {
		return nil
	}
	return &gateStatisticsJSON{
		VoicedLowPercentile: jsonFloat(g.VoicedLowPercentile),
		NoiseHighPercentile: jsonFloat(g.NoiseHighPercentile),
		SeparationDB:        jsonFloat(g.SeparationDB),
	}
}

type regionSamplesJSON struct {
	Input    *regionSampleJSON `json:"input,omitempty"`
	Filtered *regionSampleJSON `json:"filtered,omitempty"`
	Final    *regionSampleJSON `json:"final,omitempty"`
}

func newRegionSamplesJSON(samples RegionSamples) regionSamplesJSON {
	return regionSamplesJSON{
		Input:    newRegionSampleJSON(samples.Input),
		Filtered: newRegionSampleJSON(samples.Filtered),
		Final:    newRegionSampleJSON(samples.Final),
	}
}

type regionSampleJSON struct {
	RMSLevel      jsonFloat            `json:"rms_level_dbfs"`
	PeakLevel     jsonFloat            `json:"peak_level_dbfs"`
	CrestFactor   jsonFloat            `json:"crest_factor_db"`
	Spectral      *spectralMetricsJSON `json:"spectral"`
	MomentaryLUFS jsonFloat            `json:"momentary_lufs"`
	ShortTermLUFS jsonFloat            `json:"short_term_lufs"`
	TruePeak      jsonFloat            `json:"true_peak_dbtp"`
	SamplePeak    jsonFloat            `json:"sample_peak_dbfs"`
}

func newRegionSampleJSON(s *RegionSample) *regionSampleJSON {
	if s == nil {
		return nil
	}
	return &regionSampleJSON{
		RMSLevel:      jsonFloat(s.RMSLevel),
		PeakLevel:     jsonFloat(s.PeakLevel),
		CrestFactor:   jsonFloat(s.CrestFactor),
		Spectral:      newSpectralMetricsJSON(&s.Spectral),
		MomentaryLUFS: jsonFloat(s.MomentaryLUFS),
		ShortTermLUFS: jsonFloat(s.ShortTermLUFS),
		TruePeak:      jsonFloat(s.TruePeak),
		SamplePeak:    jsonFloat(s.SamplePeak),
	}
}

type filtersBlockJSON struct {
	RumbleHighPass      biquadFilterConfigJSON   `json:"rumble_highpass"`
	BandlimitLowPass    biquadFilterConfigJSON   `json:"bandlimit_lowpass"`
	NoiseReduction      noiseReductionConfigJSON `json:"noise_reduction"`
	SpeechGate          speechGateConfigJSON     `json:"speech_gate"`
	LevellingCompressor compressorConfigJSON     `json:"levelling_compressor"`
	Deesser             deesserConfigJSON        `json:"deesser"`
	Diagnostics         *adaptiveDiagnosticsJSON `json:"diagnostics,omitempty"`
}

func newFiltersBlockJSON(f *FiltersBlock) *filtersBlockJSON {
	if f == nil {
		return nil
	}
	return &filtersBlockJSON{
		RumbleHighPass:      newBiquadFilterConfigJSON(f.RumbleHighPass),
		BandlimitLowPass:    newBiquadFilterConfigJSON(f.BandlimitLowPass),
		NoiseReduction:      newNoiseReductionConfigJSON(f.NoiseReduction),
		SpeechGate:          newSpeechGateConfigJSON(f.SpeechGate),
		LevellingCompressor: newCompressorConfigJSON(f.LevellingCompressor),
		Deesser:             newDeesserConfigJSON(f.Deesser),
		Diagnostics:         newAdaptiveDiagnosticsJSON(f.Diagnostics),
	}
}

type biquadFilterConfigJSON struct {
	Enabled   bool      `json:"enabled"`
	Frequency jsonFloat `json:"frequency_hz"`
	Poles     int       `json:"poles_count"`
	Width     jsonFloat `json:"width"`
	Mix       jsonFloat `json:"mix"`
	Transform string    `json:"transform"`
}

func newBiquadFilterConfigJSON(c BiquadFilterConfig) biquadFilterConfigJSON {
	return biquadFilterConfigJSON{
		Enabled:   c.Enabled,
		Frequency: jsonFloat(c.Frequency),
		Poles:     c.Poles,
		Width:     jsonFloat(c.Width),
		Mix:       jsonFloat(c.Mix),
		Transform: c.Transform,
	}
}

type noiseReductionConfigJSON struct {
	Enabled              bool      `json:"enabled"`
	Strength             jsonFloat `json:"strength"`
	PatchSec             jsonFloat `json:"patch_s"`
	ResearchSec          jsonFloat `json:"research_s"`
	Smooth               jsonFloat `json:"smooth"`
	AfftdnEnabled        bool      `json:"afftdn_enabled"`
	AfftdnNoiseReduction jsonFloat `json:"afftdn_noise_reduction_db"`
	AfftdnNoiseType      string    `json:"afftdn_noise_type"`
	AfftdnTrackNoise     bool      `json:"afftdn_track_noise"`
	AfftdnNoiseFloor     jsonFloat `json:"afftdn_noise_floor_db"`
	AfftdnBandNoise      string    `json:"afftdn_band_noise,omitempty"`
}

func newNoiseReductionConfigJSON(c NoiseReductionConfig) noiseReductionConfigJSON {
	return noiseReductionConfigJSON{
		Enabled:              c.Enabled,
		Strength:             jsonFloat(c.Strength),
		PatchSec:             jsonFloat(c.PatchSec),
		ResearchSec:          jsonFloat(c.ResearchSec),
		Smooth:               jsonFloat(c.Smooth),
		AfftdnEnabled:        c.AfftdnEnabled,
		AfftdnNoiseReduction: jsonFloat(c.AfftdnNoiseReduction),
		AfftdnNoiseType:      c.AfftdnNoiseType,
		AfftdnTrackNoise:     c.AfftdnTrackNoise,
		AfftdnNoiseFloor:     jsonFloat(c.AfftdnNoiseFloor),
		AfftdnBandNoise:      c.AfftdnBandNoise,
	}
}

type speechGateConfigJSON struct {
	Enabled   bool      `json:"enabled"`
	Threshold jsonFloat `json:"threshold_db"`
	Ratio     jsonFloat `json:"ratio"`
	Attack    jsonFloat `json:"attack_ms"`
	Release   jsonFloat `json:"release_ms"`
	Range     jsonFloat `json:"range_db"`
	Knee      jsonFloat `json:"knee"`
	Makeup    jsonFloat `json:"makeup"`
	Detection string    `json:"detection"`
}

func newSpeechGateConfigJSON(c SpeechGateConfig) speechGateConfigJSON {
	return speechGateConfigJSON{
		Enabled:   c.Enabled,
		Threshold: jsonFloat(c.Threshold),
		Ratio:     jsonFloat(c.Ratio),
		Attack:    jsonFloat(c.Attack),
		Release:   jsonFloat(c.Release),
		Range:     jsonFloat(c.Range),
		Knee:      jsonFloat(c.Knee),
		Makeup:    jsonFloat(c.Makeup),
		Detection: c.Detection,
	}
}

type compressorConfigJSON struct {
	Enabled   bool      `json:"enabled"`
	Threshold jsonFloat `json:"threshold_db"`
	Ratio     jsonFloat `json:"ratio"`
	Attack    jsonFloat `json:"attack_ms"`
	Release   jsonFloat `json:"release_ms"`
	Makeup    jsonFloat `json:"makeup_db"`
	Knee      jsonFloat `json:"knee"`
	Mix       jsonFloat `json:"mix"`
}

func newCompressorConfigJSON(c LevellingCompressorConfig) compressorConfigJSON {
	return compressorConfigJSON{
		Enabled:   c.Enabled,
		Threshold: jsonFloat(c.Threshold),
		Ratio:     jsonFloat(c.Ratio),
		Attack:    jsonFloat(c.Attack),
		Release:   jsonFloat(c.Release),
		Makeup:    jsonFloat(c.Makeup),
		Knee:      jsonFloat(c.Knee),
		Mix:       jsonFloat(c.Mix),
	}
}

type deesserConfigJSON struct {
	Enabled   bool      `json:"enabled"`
	Intensity jsonFloat `json:"intensity"`
	Amount    jsonFloat `json:"amount"`
	Frequency jsonFloat `json:"frequency"`
}

func newDeesserConfigJSON(c DeesserConfig) deesserConfigJSON {
	return deesserConfigJSON{
		Enabled:   c.Enabled,
		Intensity: jsonFloat(c.Intensity),
		Amount:    jsonFloat(c.Amount),
		Frequency: jsonFloat(c.Frequency),
	}
}

type adaptiveDiagnosticsJSON struct {
	DynamicRangeDB                jsonFloat `json:"dynamic_range_db"`
	BandlimitLPReason             string    `json:"bandlimit_lowpass_reason"`
	SpeechGateQuietSpeechEstimate jsonFloat `json:"quiet_speech_estimate_dbfs"`
	SpeechGateSpeechSeparation    jsonFloat `json:"separation_db"`
	SpeechGateSpeechHeadroom      jsonFloat `json:"speech_headroom_db"`
	SpeechGateThresholdUnclamped  jsonFloat `json:"threshold_unclamped_db"`
	SpeechGateClampReason         string    `json:"clamp_reason"`
	SpeechGateDepthDB             jsonFloat `json:"speech_gate_depth_db"`
	SpeechGateNarrowGap           bool      `json:"narrow_gap"`
	AfftdnEnabled                 bool      `json:"afftdn_enabled"`
	AfftdnNoiseFloorDB            jsonFloat `json:"afftdn_noise_floor_db"`
	AfftdnDisableReason           string    `json:"afftdn_disable_reason"`
	AfftdnNoiseType               string    `json:"afftdn_noise_type"`
}

func newAdaptiveDiagnosticsJSON(d *AdaptiveDiagnostics) *adaptiveDiagnosticsJSON {
	if d == nil {
		return nil
	}
	return &adaptiveDiagnosticsJSON{
		DynamicRangeDB:                jsonFloat(0),
		BandlimitLPReason:             d.BandlimitLPReason,
		SpeechGateQuietSpeechEstimate: jsonFloat(d.SpeechGateQuietSpeechEstimate),
		SpeechGateSpeechSeparation:    jsonFloat(d.SpeechGateSpeechSeparation),
		SpeechGateSpeechHeadroom:      jsonFloat(d.SpeechGateSpeechHeadroom),
		SpeechGateThresholdUnclamped:  jsonFloat(d.SpeechGateThresholdUnclamped),
		SpeechGateClampReason:         d.SpeechGateClampReason,
		SpeechGateDepthDB:             jsonFloat(d.SpeechGateDepthDB),
		SpeechGateNarrowGap:           d.SpeechGateNarrowGap,
		AfftdnEnabled:                 d.AfftdnEnabled,
		AfftdnNoiseFloorDB:            jsonFloat(d.AfftdnNoiseFloorDB),
		AfftdnDisableReason:           d.AfftdnDisableReason,
		AfftdnNoiseType:               d.AfftdnNoiseType,
	}
}

type intervalSummaryJSON struct {
	Count        int                  `json:"count"`
	RMS          *rmsDistributionJSON `json:"rms_distribution,omitempty"`
	LargestGapDB *jsonFloat           `json:"largest_gap_db,omitempty"`
}

type rmsDistributionJSON struct {
	Min jsonFloat `json:"min_dbfs"`
	P10 jsonFloat `json:"p10_dbfs"`
	P25 jsonFloat `json:"p25_dbfs"`
	P50 jsonFloat `json:"p50_dbfs"`
	P75 jsonFloat `json:"p75_dbfs"`
	P90 jsonFloat `json:"p90_dbfs"`
	Max jsonFloat `json:"max_dbfs"`
}

func newIntervalSummaryJSON(s *IntervalSummary) *intervalSummaryJSON {
	if s == nil {
		return nil
	}
	return &intervalSummaryJSON{
		Count:        s.Count,
		RMS:          newRMSDistributionJSON(s.RMS),
		LargestGapDB: jsonFloatPtr(s.LargestGapDB),
	}
}

func newRMSDistributionJSON(r *RMSDistribution) *rmsDistributionJSON {
	if r == nil {
		return nil
	}
	return &rmsDistributionJSON{
		Min: jsonFloat(r.Min),
		P10: jsonFloat(r.P10),
		P25: jsonFloat(r.P25),
		P50: jsonFloat(r.P50),
		P75: jsonFloat(r.P75),
		P90: jsonFloat(r.P90),
		Max: jsonFloat(r.Max),
	}
}
