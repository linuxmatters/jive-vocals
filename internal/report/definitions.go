package report

// Objective metric catalogue for the Markdown report.
//
// Source of record: docs/Spectral-Metrics-Reference.md (objective definitions,
// units, computation, and source filter for every metric Jive Vocals emits).
// Glosses here are transcribed from that reference and cross-checked against the
// audio-metrics skill SKILL.md for the precise ffmpeg computation. Map keys are
// the exact RunRecord JSON field names (AUDIO-MEASUREMENTS.md §8.4 unit-suffix
// convention); confirm key strings against an emitted record before editing.
//
// Risk: definition drift. If a metric's computation, unit, or key changes in the
// reference or the record schema, update the matching entry here. The
// required-key test (definitions_test.go) fails when a renderer-needed key has no
// definition, but it cannot detect a stale gloss. Keep this file aligned with
// the reference by hand.
//
// Glosses are OBJECTIVE: what the metric is and, in brief, how it is computed. No
// interpretation, no quality verdict, no range-to-meaning mapping.

// Definition describes one metric for the report: a human-readable label, the
// unit string rendered alongside the value, and a one-line objective gloss.
type Definition struct {
	Label string
	Unit  string
	Gloss string
}

type metricKey string

type metricDescriptor struct {
	key        metricKey
	definition Definition
	format     metricFormat
	decimals   int
}

func metric(key metricKey, definition Definition, format metricFormat, decimals int) metricDescriptor {
	return metricDescriptor{key: key, definition: definition, format: format, decimals: decimals}
}

func (d metricDescriptor) withFormat(format metricFormat, decimals int) metricDescriptor {
	d.format = format
	d.decimals = decimals
	return d
}

const (
	keyIntegratedLUFS            metricKey = "integrated_lufs"
	keyTruePeakDBTP              metricKey = "true_peak_dbtp"
	keyLRALU                     metricKey = "lra_lu"
	keySamplePeakDBFS            metricKey = "sample_peak_dbfs"
	keyMomentaryLUFS             metricKey = "momentary_lufs"
	keyShortTermLUFS             metricKey = "short_term_lufs"
	keyThreshLUFS                metricKey = "thresh_lufs"
	keyTargetOffsetDB            metricKey = "target_offset_db"
	keyRMSLevelDBFS              metricKey = "rms_level_dbfs"
	keyPeakLevelDBFS             metricKey = "peak_level_dbfs"
	keyCrestFactorAstatsDB       metricKey = "crest_factor_astats_db"
	keyCrestFactorDB             metricKey = "crest_factor_db"
	keyDynamicRangeDB            metricKey = "dynamic_range_db"
	keyMinLevelDBFS              metricKey = "min_level_dbfs"
	keyMaxLevelDBFS              metricKey = "max_level_dbfs"
	keyRMSPeakDBFS               metricKey = "rms_peak_dbfs"
	keyRMSTroughDBFS             metricKey = "rms_trough_dbfs"
	keyFlatFactor                metricKey = "flat_factor"
	keyDCOffset                  metricKey = "dc_offset"
	keyZeroCrossingsRate         metricKey = "zero_crossings_rate"
	keyBitDepth                  metricKey = "bit_depth"
	keyEntropy                   metricKey = "entropy"
	keyMean                      metricKey = "mean"
	keyVariance                  metricKey = "variance"
	keyCentroidHz                metricKey = "centroid_hz"
	keySpreadHz                  metricKey = "spread_hz"
	keySkewness                  metricKey = "skewness"
	keyKurtosis                  metricKey = "kurtosis"
	keyFlatness                  metricKey = "flatness"
	keyCrest                     metricKey = "crest"
	keyFlux                      metricKey = "flux"
	keySlope                     metricKey = "slope"
	keyDecrease                  metricKey = "decrease"
	keyRolloffHz                 metricKey = "rolloff_hz"
	keyFloorDBFS                 metricKey = "floor_dbfs"
	keyFloorSource               metricKey = "floor_source"
	keyFloorPrescanDBFS          metricKey = "floor_prescan_dbfs"
	keyFloorAstatsDBFS           metricKey = "floor_astats_dbfs"
	keyRoomToneDetectLevelDBFS   metricKey = "room_tone_detect_level_dbfs"
	keyVoiceActivated            metricKey = "voice_activated"
	keyFlooredFraction           metricKey = "floored_fraction"
	keyReductionHeadroomDB       metricKey = "reduction_headroom_db"
	keyVoicedLowPercentileDBFS   metricKey = "voiced_low_percentile_dbfs"
	keyNoiseHighPercentileDBFS   metricKey = "noise_high_percentile_dbfs"
	keyGateSeparationDB          metricKey = "gate_separation_db"
	keyMeasuredFloorDBFS         metricKey = "measured_floor_dbfs"
	keyStartS                    metricKey = "start_s"
	keyDurationS                 metricKey = "duration_s"
	keySpectralCentroidHz        metricKey = "spectral_centroid_hz"
	keySpectralFlatness          metricKey = "spectral_flatness"
	keySpectralKurtosis          metricKey = "spectral_kurtosis"
	keyVoicingDensity            metricKey = "voicing_density"
	keySpeechBandBodyRMSDBFS     metricKey = "speech_band_body_rms_dbfs"
	keySpeechBandSibilantRMSDBFS metricKey = "speech_band_sib_rms_dbfs"
	keyScore                     metricKey = "score"
	keyIntervalCount             metricKey = "interval_count"
	keyLargestGapDB              metricKey = "largest_gap_db"
	keyRMSDistributionMinDBFS    metricKey = "rms_dist_min_dbfs"
	keyRMSDistributionP10DBFS    metricKey = "rms_dist_p10_dbfs"
	keyRMSDistributionP25DBFS    metricKey = "rms_dist_p25_dbfs"
	keyRMSDistributionP50DBFS    metricKey = "rms_dist_p50_dbfs"
	keyRMSDistributionP75DBFS    metricKey = "rms_dist_p75_dbfs"
	keyRMSDistributionP90DBFS    metricKey = "rms_dist_p90_dbfs"
	keyRMSDistributionMaxDBFS    metricKey = "rms_dist_max_dbfs"
)

var (
	// Loudness (ebur128 + loudnorm)
	integratedLUFSMetric = metric(keyIntegratedLUFS, Definition{
		Label: "Integrated loudness",
		Unit:  "LUFS",
		Gloss: "Gated programme loudness over the whole input, BS.1770 K-weighted mean-square with two-stage gating.",
	}, fmtLUFS, 2)
	truePeakDBTPMetric = metric(keyTruePeakDBTP, Definition{
		Label: "True peak",
		Unit:  "dBTP",
		Gloss: "Inter-sample peak of the libswresample-oversampled signal.",
	}, fmtPeakDB, 2)
	lraLUMetric = metric(keyLRALU, Definition{
		Label: "Loudness range",
		Unit:  "LU",
		Gloss: "Statistical spread of the 3 s short-term loudness distribution (lra_high minus lra_low).",
	}, fmtLU, 2)
	samplePeakDBFSMetric = metric(keySamplePeakDBFS, Definition{
		Label: "Sample peak",
		Unit:  "dBFS",
		Gloss: "Largest digital sample without oversampling, 20*log10(sample_peak).",
	}, fmtDB, 2)
	momentaryLUFSMetric = metric(keyMomentaryLUFS, Definition{
		Label: "Momentary loudness",
		Unit:  "LUFS",
		Gloss: "BS.1770 loudness over a 400 ms sliding window.",
	}, fmtLUFS, 2)
	shortTermLUFSMetric = metric(keyShortTermLUFS, Definition{
		Label: "Short-term loudness",
		Unit:  "LUFS",
		Gloss: "BS.1770 loudness over a 3 s sliding window.",
	}, fmtLUFS, 2)
	threshLUFSMetric = metric(keyThreshLUFS, Definition{
		Label: "Gating threshold",
		Unit:  "LUFS",
		Gloss: "Relative gating threshold, -10 LU below the absolute-gated loudness mean.",
	}, fmtLUFS, 2)
	targetOffsetDBMetric = metric(keyTargetOffsetDB, Definition{
		Label: "Target offset",
		Unit:  "LU",
		Gloss: "Residual gap to the target integrated loudness, target_i minus output_i.",
	}, fmtSigned, 2)

	// Dynamics (astats, time domain)
	rmsLevelDBFSMetric = metric(keyRMSLevelDBFS, Definition{
		Label: "RMS level",
		Unit:  "dBFS",
		Gloss: "RMS amplitude of the samples, 20*log10(sqrt(sum(x^2)/N)).",
	}, fmtDB, 2)
	peakLevelDBFSMetric = metric(keyPeakLevelDBFS, Definition{
		Label: "Peak level",
		Unit:  "dBFS",
		Gloss: "Largest absolute sample, 20*log10(max(|min|,|max|)).",
	}, fmtDB, 2)
	crestFactorAstatsDBMetric = metric(keyCrestFactorAstatsDB, Definition{
		Label: "Crest factor",
		Unit:  "dB",
		Gloss: "Time-domain peak-to-RMS ratio (peak/RMS), stored converted linear to dB.",
	}, fmtSpectral, 4)
	crestFactorDBMetric = metric(keyCrestFactorDB, Definition{
		Label: "Crest factor",
		Unit:  "dB",
		Gloss: "Region-scoped time-domain peak-to-RMS ratio (peak/RMS), stored converted linear to dB.",
	}, fmtSpectral, 2)
	dynamicRangeDBMetric = metric(keyDynamicRangeDB, Definition{
		Label: "Dynamic range",
		Unit:  "dB",
		Gloss: "Span between loudest and quietest non-zero sample, 20*log10(2*max(|min|,|max|)/min_nonzero).",
	}, fmtSpectral, 4)
	minLevelDBFSMetric = metric(keyMinLevelDBFS, Definition{
		Label: "Min level",
		Unit:  "dBFS",
		Gloss: "Smallest signed sample, converted to dBFS.",
	}, fmtDB, 2)
	maxLevelDBFSMetric = metric(keyMaxLevelDBFS, Definition{
		Label: "Max level",
		Unit:  "dBFS",
		Gloss: "Largest signed sample, converted to dBFS.",
	}, fmtDB, 2)
	rmsPeakDBFSMetric = metric(keyRMSPeakDBFS, Definition{
		Label: "RMS peak",
		Unit:  "dBFS",
		Gloss: "Maximum per-window RMS over the short measurement window.",
	}, fmtDB, 2)
	rmsTroughDBFSMetric = metric(keyRMSTroughDBFS, Definition{
		Label: "RMS trough",
		Unit:  "dBFS",
		Gloss: "Minimum per-window RMS over the short measurement window.",
	}, fmtDB, 2)
	flatFactorMetric = metric(keyFlatFactor, Definition{
		Label: "Flat factor",
		Unit:  "",
		Gloss: "Run-length flatness at the min/max levels, 20*log10((min_runs+max_runs)/(min_count+max_count)).",
	}, fmtSpectral, 4)
	dcOffsetMetric = metric(keyDCOffset, Definition{
		Label: "DC offset",
		Unit:  "",
		Gloss: "Mean sample amplitude, sum(x)/N.",
	}, fmtSpectral, 4)
	zeroCrossingsRateMetric = metric(keyZeroCrossingsRate, Definition{
		Label: "Zero-crossings rate",
		Unit:  "",
		Gloss: "Fraction of sample pairs that change sign, zero_crossings/N.",
	}, fmtSpectral, 4)
	bitDepthMetric = metric(keyBitDepth, Definition{
		Label: "Bit depth",
		Unit:  "bits",
		Gloss: "Effective bit depth estimated from the sample data.",
	}, fmtSpectral, 4)
	entropyMetric = metric(keyEntropy, Definition{
		Label: "Entropy",
		Unit:  "",
		Gloss: "Magnitude-weighted spectral entropy, -sum(mag*ln(mag+eps))/ln(N); for astats stages, the sample-value distribution entropy.",
	}, fmtSpectral, 4)

	// Spectral (aspectralstats, the 13 fields)
	meanMetric = metric(keyMean, Definition{
		Label: "Spectral mean",
		Unit:  "",
		Gloss: "Arithmetic mean of the magnitude bins, sum(mag[n])/size.",
	}, fmtSpectral, 4)
	varianceMetric = metric(keyVariance, Definition{
		Label: "Spectral variance",
		Unit:  "",
		Gloss: "Population variance of the magnitudes about the mean, sum((mag[n]-mean)^2)/size.",
	}, fmtSpectral, 4)
	centroidHzMetric = metric(keyCentroidHz, Definition{
		Label: "Spectral centroid",
		Unit:  "Hz",
		Gloss: "Magnitude-weighted mean frequency of the spectrum, sum(mag[n]*f[n])/sum(mag[n]).",
	}, fmtSpectral, 4)
	spreadHzMetric = metric(keySpreadHz, Definition{
		Label: "Spectral spread",
		Unit:  "Hz",
		Gloss: "Magnitude-weighted standard deviation of frequency about the centroid.",
	}, fmtSpectral, 4)
	skewnessMetric = metric(keySkewness, Definition{
		Label: "Spectral skewness",
		Unit:  "",
		Gloss: "Third standardised spectral moment about the centroid.",
	}, fmtSpectral, 4)
	kurtosisMetric = metric(keyKurtosis, Definition{
		Label: "Spectral kurtosis",
		Unit:  "",
		Gloss: "Fourth standardised (Pearson) spectral moment about the centroid; not excess kurtosis.",
	}, fmtSpectral, 4)
	flatnessMetric = metric(keyFlatness, Definition{
		Label: "Spectral flatness",
		Unit:  "",
		Gloss: "Geometric mean over arithmetic mean of the magnitudes, a 0-1 linear ratio.",
	}, fmtSpectral, 4)
	crestMetric = metric(keyCrest, Definition{
		Label: "Spectral crest",
		Unit:  "",
		Gloss: "Peak magnitude bin over mean magnitude bin, max(mag[n])/mean(mag[n]).",
	}, fmtSpectral, 4)
	fluxMetric = metric(keyFlux, Definition{
		Label: "Spectral flux",
		Unit:  "",
		Gloss: "L2 distance between this frame's and the previous frame's magnitude spectrum.",
	}, fmtSpectral, 4)
	slopeMetric = metric(keySlope, Definition{
		Label: "Spectral slope",
		Unit:  "",
		Gloss: "Linear-regression slope of magnitude against normalised bin index.",
	}, fmtSpectral, 4)
	decreaseMetric = metric(keyDecrease, Definition{
		Label: "Spectral decrease",
		Unit:  "",
		Gloss: "Relative spectral decrease from the first bin, sum((mag[n]-mag[0])/n)/sum(mag[n]).",
	}, fmtSpectral, 4)
	rolloffHzMetric = metric(keyRolloffHz, Definition{
		Label: "Spectral roll-off",
		Unit:  "Hz",
		Gloss: "Frequency below which 85% of the cumulative magnitude lies.",
	}, fmtSpectral, 4)

	// Noise (input-only noise domain)
	floorDBFSMetric = metric(keyFloorDBFS, Definition{
		Label: "Noise floor",
		Unit:  "dBFS",
		Gloss: "Input VAD noise floor on the K-weighted momentary-LUFS axis (the afftdn seed); a low percentile of the per-interval level set. A different axis and quantity from the room-tone RMS floor (measured_floor_dbfs).",
	}, fmtDB, 2)
	floorSourceMetric = metric(keyFloorSource, Definition{
		Label: "Floor source",
		Unit:  "",
		Gloss: "Origin of the elected floor: astats, rms_estimate, ebur128_estimate, or vad_percentile.",
	}, fmtSpectral, 4)
	floorPrescanDBFSMetric = metric(keyFloorPrescanDBFS, Definition{
		Label: "Pre-scan floor",
		Unit:  "dBFS",
		Gloss: "Noise floor estimated from the per-interval data, feeding room-tone detection.",
	}, fmtDB, 2)
	floorAstatsDBFSMetric = metric(keyFloorAstatsDBFS, Definition{
		Label: "astats floor",
		Unit:  "dBFS",
		Gloss: "FFmpeg astats noise-floor estimate, the minimum local peak over the sliding window.",
	}, fmtDB, 2)
	roomToneDetectLevelDBFSMetric = metric(keyRoomToneDetectLevelDBFS, Definition{
		Label: "Room-tone detect level",
		Unit:  "dBFS",
		Gloss: "Adaptive threshold below which an interval is treated as a room-tone candidate.",
	}, fmtDB, 2)
	voiceActivatedMetric = metric(keyVoiceActivated, Definition{
		Label: "Voice activated",
		Unit:  "",
		Gloss: "True when the floored (digital-silence) interval fraction is high, the platform-gated capture signature.",
	}, fmtSpectral, 4)
	flooredFractionMetric = metric(keyFlooredFraction, Definition{
		Label: "Floored fraction",
		Unit:  "",
		Gloss: "Fraction (0..1) of intervals sitting at the digital-silence floor; the detection margin behind voice_activated, which trips at or above the fixed threshold.",
	}, fmtSpectral, 4)
	reductionHeadroomDBMetric = metric(keyReductionHeadroomDB, Definition{
		Label: "Reduction headroom",
		Unit:  "dB",
		Gloss: "Gap in dB between the noise floor and quiet speech.",
	}, fmtSpectral, 2)

	// Regions: elected profile bounds and election-only fields
	voicedLowPercentileDBFSMetric = metric(keyVoicedLowPercentileDBFS, Definition{
		Label: "Voiced low percentile",
		Unit:  "dBFS",
		Gloss: "10th percentile of voiced-speech momentary loudness over the elected region: the quiet edge of speech.",
	}, fmtDB, 2)
	noiseHighPercentileDBFSMetric = metric(keyNoiseHighPercentileDBFS, Definition{
		Label: "Noise high percentile",
		Unit:  "dBFS",
		Gloss: "95th percentile of below-split momentary loudness: the loud edge of the noise.",
	}, fmtDB, 2)
	gateSeparationDBMetric = metric(keyGateSeparationDB, Definition{
		Label: "Gate separation",
		Unit:  "dB",
		Gloss: "Difference between the voiced low percentile and the noise high percentile.",
	}, fmtSpectral, 2)
	measuredFloorDBFSMetric = metric(keyMeasuredFloorDBFS, Definition{
		Label: "Measured floor",
		Unit:  "dBFS",
		Gloss: "Input room-tone RMS (dBFS), the RMS level of the elected room-tone region on the astats RMS axis. A different axis and quantity from the VAD noise floor (floor_dbfs).",
	}, fmtDB, 2)
	startSMetric = metric(keyStartS, Definition{
		Label: "Start",
		Unit:  "s",
		Gloss: "Start time of the elected region from the input origin.",
	}, fmtRaw, 2)
	durationSMetric = metric(keyDurationS, Definition{
		Label: "Duration",
		Unit:  "s",
		Gloss: "Length of the elected region.",
	}, fmtRaw, 2)
	spectralCentroidHzMetric = metric(keySpectralCentroidHz, Definition{
		Label: "Spectral centroid",
		Unit:  "Hz",
		Gloss: "Magnitude-weighted mean frequency of the elected region's spectrum.",
	}, fmtSpectral, 2)
	spectralFlatnessMetric = metric(keySpectralFlatness, Definition{
		Label: "Spectral flatness",
		Unit:  "",
		Gloss: "Geometric over arithmetic mean of the elected region's magnitudes, a 0-1 ratio.",
	}, fmtSpectral, 4)
	spectralKurtosisMetric = metric(keySpectralKurtosis, Definition{
		Label: "Spectral kurtosis",
		Unit:  "",
		Gloss: "Fourth standardised spectral moment of the elected region.",
	}, fmtSpectral, 4)
	voicingDensityMetric = metric(keyVoicingDensity, Definition{
		Label: "Voicing density",
		Unit:  "",
		Gloss: "Proportion of voiced intervals over the elected speech region, 0-1.",
	}, fmtSpectral, 4)
	speechBandBodyRMSDBFSMetric = metric(keySpeechBandBodyRMSDBFS, Definition{
		Label: "Body-band RMS",
		Unit:  "dBFS",
		Gloss: "RMS over the 1-3 kHz vocal-presence band of the elected speech region.",
	}, fmtDB, 2)
	speechBandSibilantRMSDBFSMetric = metric(keySpeechBandSibilantRMSDBFS, Definition{
		Label: "Sibilant-band RMS",
		Unit:  "dBFS",
		Gloss: "RMS over the 6-9 kHz sibilant band of the elected speech region.",
	}, fmtDB, 2)
	scoreMetric = metric(keyScore, Definition{
		Label: "Score",
		Unit:  "",
		Gloss: "Composite candidate-ranking score of the elected region.",
	}, fmtSpectral, 4)

	// Interval summary (per-250ms RMS distribution + gap)
	intervalCountMetric = metric(keyIntervalCount, Definition{
		Label: "Interval count",
		Unit:  "count",
		Gloss: "Number of 250 ms intervals sampled over the input.",
	}, fmtRaw, 0)
	largestGapDBMetric = metric(keyLargestGapDB, Definition{
		Label: "Largest gap",
		Unit:  "dB",
		Gloss: "Biggest jump between adjacent sorted interval RMS values, the room-tone/speech boundary signal.",
	}, fmtSpectral, 2)
	rmsDistributionMinDBFSMetric = metric(keyRMSDistributionMinDBFS, Definition{
		Label: "RMS min",
		Unit:  "dBFS",
		Gloss: "Lowest interval RMS above digital silence.",
	}, fmtDB, 2)
	rmsDistributionP10DBFSMetric = metric(keyRMSDistributionP10DBFS, Definition{
		Label: "RMS p10",
		Unit:  "dBFS",
		Gloss: "10th-percentile interval RMS above digital silence.",
	}, fmtDB, 2)
	rmsDistributionP25DBFSMetric = metric(keyRMSDistributionP25DBFS, Definition{
		Label: "RMS p25",
		Unit:  "dBFS",
		Gloss: "25th-percentile interval RMS above digital silence.",
	}, fmtDB, 2)
	rmsDistributionP50DBFSMetric = metric(keyRMSDistributionP50DBFS, Definition{
		Label: "RMS p50",
		Unit:  "dBFS",
		Gloss: "Median interval RMS above digital silence.",
	}, fmtDB, 2)
	rmsDistributionP75DBFSMetric = metric(keyRMSDistributionP75DBFS, Definition{
		Label: "RMS p75",
		Unit:  "dBFS",
		Gloss: "75th-percentile interval RMS above digital silence.",
	}, fmtDB, 2)
	rmsDistributionP90DBFSMetric = metric(keyRMSDistributionP90DBFS, Definition{
		Label: "RMS p90",
		Unit:  "dBFS",
		Gloss: "90th-percentile interval RMS above digital silence.",
	}, fmtDB, 2)
	rmsDistributionMaxDBFSMetric = metric(keyRMSDistributionMaxDBFS, Definition{
		Label: "RMS max",
		Unit:  "dBFS",
		Gloss: "Highest interval RMS above digital silence.",
	}, fmtDB, 2)

	regionSampleCrestFactorDBMetric = crestFactorDBMetric.withFormat(fmtSpectral, 4)
)

var loudnessMetricDescriptors = []metricDescriptor{
	integratedLUFSMetric,
	truePeakDBTPMetric,
	lraLUMetric,
	threshLUFSMetric,
	momentaryLUFSMetric,
	shortTermLUFSMetric,
	samplePeakDBFSMetric,
	targetOffsetDBMetric,
}

var dynamicsMetricDescriptors = []metricDescriptor{
	rmsLevelDBFSMetric,
	peakLevelDBFSMetric,
	crestFactorAstatsDBMetric,
	dynamicRangeDBMetric,
	minLevelDBFSMetric,
	maxLevelDBFSMetric,
	rmsPeakDBFSMetric,
	rmsTroughDBFSMetric,
	flatFactorMetric,
	dcOffsetMetric,
	zeroCrossingsRateMetric,
	bitDepthMetric,
	entropyMetric,
}

var spectralMetricDescriptors = []metricDescriptor{
	meanMetric,
	varianceMetric,
	centroidHzMetric,
	spreadHzMetric,
	skewnessMetric,
	kurtosisMetric,
	entropyMetric,
	flatnessMetric,
	crestMetric,
	fluxMetric,
	slopeMetric,
	decreaseMetric,
	rolloffHzMetric,
}

var noiseMetricDescriptors = []metricDescriptor{
	floorDBFSMetric,
	floorSourceMetric,
	floorPrescanDBFSMetric,
	floorAstatsDBFSMetric,
	roomToneDetectLevelDBFSMetric,
	voiceActivatedMetric,
	flooredFractionMetric,
	reductionHeadroomDBMetric,
}

var gateStatisticMetricDescriptors = []metricDescriptor{
	voicedLowPercentileDBFSMetric,
	noiseHighPercentileDBFSMetric,
	gateSeparationDBMetric,
}

var roomToneElectedMetricDescriptors = []metricDescriptor{
	startSMetric,
	durationSMetric,
	measuredFloorDBFSMetric,
	peakLevelDBFSMetric,
	crestFactorDBMetric,
	entropyMetric,
	spectralCentroidHzMetric,
	spectralFlatnessMetric,
	spectralKurtosisMetric,
}

var speechElectedMetricDescriptors = []metricDescriptor{
	durationSMetric,
	rmsLevelDBFSMetric,
	peakLevelDBFSMetric,
	crestFactorDBMetric,
	momentaryLUFSMetric,
	shortTermLUFSMetric,
	truePeakDBTPMetric,
	samplePeakDBFSMetric,
	speechBandBodyRMSDBFSMetric,
	speechBandSibilantRMSDBFSMetric,
	voicingDensityMetric,
	scoreMetric,
}

var regionSampleMetricDescriptors = appendMetricDescriptors(
	[]metricDescriptor{
		rmsLevelDBFSMetric,
		peakLevelDBFSMetric,
		regionSampleCrestFactorDBMetric,
		momentaryLUFSMetric,
		shortTermLUFSMetric,
		truePeakDBTPMetric,
		samplePeakDBFSMetric,
	},
	spectralMetricDescriptors,
)

var intervalMetricDescriptors = []metricDescriptor{
	intervalCountMetric,
	largestGapDBMetric,
	rmsDistributionMinDBFSMetric,
	rmsDistributionP10DBFSMetric,
	rmsDistributionP25DBFSMetric,
	rmsDistributionP50DBFSMetric,
	rmsDistributionP75DBFSMetric,
	rmsDistributionP90DBFSMetric,
	rmsDistributionMaxDBFSMetric,
}

var allMetricDescriptors = appendMetricDescriptors(
	loudnessMetricDescriptors,
	dynamicsMetricDescriptors,
	spectralMetricDescriptors,
	noiseMetricDescriptors,
	gateStatisticMetricDescriptors,
	roomToneElectedMetricDescriptors,
	speechElectedMetricDescriptors,
	regionSampleMetricDescriptors,
	intervalMetricDescriptors,
)

// Definitions maps a RunRecord JSON field name to its objective definition. The
// descriptor groups above are the source of record for emitted report metrics.
var Definitions = definitionsByKey(allMetricDescriptors)

// requiredKeys is derived from the descriptors the report sections emit.
var requiredKeys = metricKeys(allMetricDescriptors)

func appendMetricDescriptors(groups ...[]metricDescriptor) []metricDescriptor {
	size := 0
	for _, group := range groups {
		size += len(group)
	}
	descriptors := make([]metricDescriptor, 0, size)
	for _, group := range groups {
		descriptors = append(descriptors, group...)
	}
	return descriptors
}

func definitionsByKey(descriptors []metricDescriptor) map[string]Definition {
	definitions := make(map[string]Definition, len(descriptors))
	for _, descriptor := range descriptors {
		key := string(descriptor.key)
		if existing, ok := definitions[key]; ok {
			if existing != descriptor.definition {
				panic("report: conflicting definition for key " + key)
			}
			continue
		}
		definitions[key] = descriptor.definition
	}
	return definitions
}

func metricKeys(descriptors []metricDescriptor) []metricKey {
	seen := make(map[metricKey]bool, len(descriptors))
	keys := make([]metricKey, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if seen[descriptor.key] {
			continue
		}
		seen[descriptor.key] = true
		keys = append(keys, descriptor.key)
	}
	return keys
}

// DefinitionFor returns the objective definition for a RunRecord field name and
// whether one exists. Renderers use this to pair each value cell with its label,
// unit, and gloss.
func DefinitionFor(key string) (Definition, bool) {
	d, ok := Definitions[key]
	return d, ok
}
