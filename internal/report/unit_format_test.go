package report

import "testing"

// TestUnitMetricFormat pins the unit-to-formatter routing in unitMetricFormat.
// Each routed unit class is exercised through a real catalogued metric key so a
// new key on an unrouted unit cannot silently mis-format, and the default branch
// panic contract is asserted so a future unrouted unit fails loudly.
func TestUnitMetricFormat(t *testing.T) {
	cases := []struct {
		name         string
		key          string // a catalogued key whose Unit is the routed class
		wantFormat   metricFormat
		wantDecimals int
	}{
		{"dBFS", "rms_level_dbfs", fmtDB, 2},
		{"dBTP", "true_peak_dbtp", fmtPeakDB, 2},
		{"LUFS", "momentary_lufs", fmtLUFS, 2},
		{"Hz", "centroid_hz", fmtSpectral, 2},
		{"unit-less", "flatness", fmtSpectral, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			format, decimals := unitMetricFormat(tc.key)
			if format != tc.wantFormat {
				t.Errorf("unitMetricFormat(%q) format = %d, want %d", tc.key, format, tc.wantFormat)
			}
			if decimals != tc.wantDecimals {
				t.Errorf("unitMetricFormat(%q) decimals = %d, want %d", tc.key, decimals, tc.wantDecimals)
			}
		})
	}
}

// TestUnitMetricFormatUnroutedPanics pins the default-branch panic contract: a
// catalogued key on an unrouted unit (here "count") must panic so it cannot reach
// a silent mis-format. "interval_count" has Unit "count", which the switch does
// not route (it is rendered as an integer via formatInt, not through this path).
func TestUnitMetricFormatUnroutedPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("unitMetricFormat on an unrouted unit did not panic")
		}
	}()
	unitMetricFormat("interval_count")
}

// TestUnitMetricFormatUncataloguedPanics pins the no-definition panic: a key with
// no catalogue entry must fail loudly rather than mis-format on a zero-value Unit.
func TestUnitMetricFormatUncataloguedPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("unitMetricFormat on an uncatalogued key did not panic")
		}
	}()
	unitMetricFormat("not_a_real_key")
}

// TestMetricLabelUncataloguedPanics pins the metricLabel no-definition panic: a
// key with no catalogue entry must fail loudly rather than return the raw key.
func TestMetricLabelUncataloguedPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("metricLabel on an uncatalogued key did not panic")
		}
	}()
	metricLabel("not_a_real_key")
}
