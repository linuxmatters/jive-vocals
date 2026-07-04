// Package processor handles audio analysis and processing
package processor

// This file holds the Pass-3/4 limiter and pre-gain derivation: the limiter
// ceiling constants, the ceiling/pre-gain maths, the filter-string builders, and
// the limiter plan and diagnostics carried into NormalisationResult.

import (
	"fmt"
	"strings"
)

// Limiter ceiling constants used by deriveLimiterAndPreGain for the ceiling
// decision and the pre-gain deficit calculation.
const (
	// minLimiterCeilingDB is the practical minimum for FFmpeg's alimiter ceiling
	// (dBTP). Engine floor, not a tuning constant.
	// See docs/Normalisation-Tuning.md for the corpus derivation.
	minLimiterCeilingDB = -24.0 // dBTP

	// brickwallTruePeakHeadroomDB is the inter-sample allowance (dB) subtracted
	// from loudnorm's TargetTP to set the brickwall's sample-peak ceiling.
	// See docs/Normalisation-Tuning.md for the corpus derivation.
	brickwallTruePeakHeadroomDB = 0.9 // dB

	// measurementCushionDB is the fixed measurement-disagreement cushion (dB) added
	// to loudnorm's internal true-peak target (loudnormInternalTargetTP). It is the
	// only static margin left in loudnorm's internal targeting.
	// See docs/Normalisation-Tuning.md for the corpus derivation.
	measurementCushionDB = 0.2 // dB

	// linearSafetyMargin keeps loudnorm inside linear-mode bounds (dB) in the
	// calculateLinearModeTarget guard. loudnormInternalTargetTP folds the same value
	// into the per-file internal TP so the cap is inert by construction.
	// See docs/Normalisation-Tuning.md for the corpus derivation.
	linearSafetyMargin = 0.1 // dB
)

// limiterDerivation carries both results of deriveLimiterAndPreGain: the limiter
// ceiling decision and the pre-gain deficit that raises a clamped-ceiling signal
// so loudnorm can still apply full linear gain.
type limiterDerivation struct {
	ceiling          float64 // limiter ceiling in dBTP (clamped to minLimiterCeilingDB if needed)
	needed           bool    // true if limiting is required (projected TP exceeds target)
	clamped          bool    // true if ceiling was clamped to minimum (loudnorm may need to adjust target)
	preGainDB        float64 // pre-gain amount in dB (positive when clamped, 0.0 otherwise)
	reDerivedCeiling float64 // limiter ceiling re-derived from post-gain values (0.0 when not clamped)
}

// deriveLimiterAndPreGain derives the Pass-4 pre-limiter ceiling and the pre-gain
// deficit applied when that ceiling is clamped, so loudnorm can apply full linear
// gain without exceeding target TP. The ceiling places the post-limiter sample
// peak the full crest budget (target_TP - target_I) above the pre-limiter
// loudness: ceiling = target_TP - gainRequired. The downstream brickwall limiter
// (buildBrickwallLimiter, pinned to target_TP) catches any post-adeclick overshoot.
//
// Ceilings below alimiter's engine floor (minLimiterCeilingDB) are clamped; on a
// clamp, preGainDB raises the signal before limiting and reDerivedCeiling is the
// ceiling re-derived from the post-gain values. Both are 0.0 when not clamped,
// and the whole derivation is zero when limiting is not needed (so diagnostics
// never report a pre-gain alongside a disabled limiter).
// See docs/Normalisation-Tuning.md for the minLimiterCeilingDB derivation.
//
// Parameters:
//   - measuredI: Measured integrated loudness from Pass 3 (LUFS)
//   - measuredTP: Measured true peak from Pass 3 (dBTP)
//   - targetI: Target integrated loudness (LUFS), typically -16.0
//   - targetTP: Target true peak (dBTP), typically -1.0
func deriveLimiterAndPreGain(measuredI, measuredTP, targetI, targetTP float64) limiterDerivation {
	gainRequired := targetI - measuredI

	// Pre-gain: computed independently of the limiting decision (the former
	// calculatePreGain took no measuredTP). When the ideal ceiling sits below
	// alimiter's minimum, the deficit raises the signal before limiting so
	// loudnorm can apply full linear gain, and the ceiling is re-derived from the
	// post-gain values. Otherwise both stay 0.0.
	idealCeiling := targetTP - gainRequired
	var d limiterDerivation
	if idealCeiling < minLimiterCeilingDB {
		d.preGainDB = minLimiterCeilingDB - idealCeiling

		postGainI := measuredI + d.preGainDB
		newGainRequired := targetI - postGainI
		d.reDerivedCeiling = targetTP - newGainRequired
	}

	// No limiting needed if linear mode already possible. Return a zero
	// derivation: any pre-gain computed above is moot without a limiter, and
	// leaving it set would mislead diagnostics (PreGainDB > 0 with
	// LimiterEnabled false).
	projectedTP := measuredTP + gainRequired
	if projectedTP <= targetTP {
		return limiterDerivation{}
	}

	// Derived ceiling: targetTP - gainRequired (= filtered_I + B, the full crest budget).
	d.ceiling = targetTP - gainRequired
	d.needed = true

	// Clamp to alimiter's minimum supported ceiling
	if d.ceiling < minLimiterCeilingDB {
		d.ceiling = minLimiterCeilingDB
		d.clamped = true
	}

	return d
}

// buildPreLimiterPrefix constructs the Pass-3/4 pre-limiter prefix: a
// comma-separated fragment of volume (when pre-gain is active) and the levelling
// alimiter (when limiting is needed) that creates true-peak headroom so loudnorm
// stays in linear mode. Returns "" when no limiting is needed.
//
// The alimiter runs a gentle 5 ms attack / 100 ms release for transparent peak
// limiting at unity gain with lookahead. asc=1:asc_level=0.8 is a program-dependent
// release shaper kept as a safety net: it stays dormant on typical material and
// engages only under heavy sustained limiting.
//
// Parameters:
//   - preGainDB: Pre-gain amount in dB (positive when clamped, 0.0 otherwise)
//   - ceiling: Limiter ceiling in dBTP
//   - needsLimiting: True if limiting is required
//
// Returns the filter string fragment or "" when no limiting needed.
func buildPreLimiterPrefix(preGainDB, ceiling float64, needsLimiting bool) string {
	if !needsLimiting {
		return ""
	}

	var parts []string

	if preGainDB > 0 {
		parts = append(parts, fmt.Sprintf("volume=%.1fdB", preGainDB))
	}

	limiterCeilingLinear := Decibels(ceiling).LinearAmplitude().Float64()
	levellingLimiterFilter := fmt.Sprintf(
		"alimiter=limit=%.6f:attack=5:release=100:level_in=1:level_out=1:level=0:latency=1:asc=1:asc_level=0.8",
		limiterCeilingLinear,
	)
	parts = append(parts, levellingLimiterFilter)

	return strings.Join(parts, ",")
}

// buildBrickwallLimiter builds the final-stage source-rate brickwall limiter (the
// peakLimiter: it owns true-peak delivery). alimiter limits SAMPLE peak, so
// ceilingDBTP is the sample-peak ceiling: the caller sets it below the loudnorm
// true-peak target by the inter-sample allowance (brickwallTruePeakHeadroomDB) so
// oversampled true peak still lands under the target. This helper is a pure
// dBTP→string converter and applies no headroom itself.
// See docs/Normalisation-Tuning.md for the brickwallTruePeakHeadroomDB derivation.
func buildBrickwallLimiter(ceilingDBTP float64) string {
	limit := Decibels(ceilingDBTP).LinearAmplitude().Float64()
	return fmt.Sprintf(
		"alimiter=limit=%.6f:attack=1:release=50:level_in=1:level_out=1:level=0:latency=1:asc=1:asc_level=0.8",
		limit,
	)
}

type limiterPlan struct {
	preGainDB   float64
	ceilingDB   float64
	needed      bool
	clamped     bool
	gainDB      float64
	pass3Prefix string
	filteredTP  float64 // Pass-2 filtered true peak (dBTP) the limiter acts on
}

// diagnostics projects the plan's six limiter values into the exported
// LimiterDiagnostics carried by NormalisationResult, so the result assigns them
// in one step instead of copying six fields by hand.
func (p limiterPlan) diagnostics() LimiterDiagnostics {
	return LimiterDiagnostics{
		LimiterEnabled:    p.needed,
		LimiterCeiling:    p.ceilingDB,
		LimiterGain:       p.gainDB,
		LimiterFilteredTP: p.filteredTP,
		PreGainDB:         p.preGainDB,
		LimiterClamped:    p.clamped,
	}
}

func planLimiterForLoudnorm(output *OutputMeasurements, config *EffectiveFilterConfig) limiterPlan {
	loudnorm := config.Loudnorm
	d := deriveLimiterAndPreGain(
		output.Loudness.OutputI, output.Loudness.OutputTP,
		loudnorm.TargetI, loudnorm.TargetTP,
	)
	ceilingDB := d.ceiling
	if d.clamped {
		ceilingDB = d.reDerivedCeiling
	}

	return limiterPlan{
		preGainDB:   d.preGainDB,
		ceilingDB:   ceilingDB,
		needed:      d.needed,
		clamped:     d.clamped,
		gainDB:      loudnorm.TargetI - output.Loudness.OutputI,
		pass3Prefix: buildPreLimiterPrefix(d.preGainDB, ceilingDB, d.needed),
		filteredTP:  output.Loudness.OutputTP,
	}
}

// LimiterDiagnostics holds the Pass-4 pre-limiting values shared between the
// internal limiterPlan and the exported NormalisationResult. It is embedded into
// NormalisationResult (anonymous, no json tag) so the JSON object stays flat: the
// six fields marshal under the same keys as before. limiterPlan.diagnostics()
// produces it so the result fills these in one assignment.
type LimiterDiagnostics struct {
	LimiterEnabled    bool    `json:"limiter_enabled"` // True if pre-limiting was applied
	LimiterCeiling    float64 `json:"ceiling_dbtp"`    // Ceiling in dBTP (only valid if LimiterEnabled)
	LimiterGain       float64 `json:"gain_db"`         // Gain required that triggered limiting (dB)
	LimiterFilteredTP float64 `json:"filtered_dbtp"`   // Pass-2 filtered true peak (dBTP) the limiter acts on
	PreGainDB         float64 `json:"pre_gain_db"`     // Pre-gain amount in dB (0.0 when no pre-gain applied)
	LimiterClamped    bool    `json:"limiter_clamped"` // True when deriveLimiterAndPreGain clamped ceiling to minimum
}
