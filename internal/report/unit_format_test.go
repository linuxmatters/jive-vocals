package report

import "testing"

// TestUnitMetricFormat pins the unit-to-formatter routing in unitMetricFormat.
// Each routed unit class is exercised through a real catalogued metric key so a
// new key on an unrouted unit cannot silently mis-format, and the default branch
// panic contract is asserted so a future unrouted unit fails loudly.
func TestUnitMetricFormat(t *testing.T) {
	cases := []struct {
		name         string
		metric       metricDescriptor
		wantFormat   metricFormat
		wantDecimals int
	}{
		{"dBFS", rmsLevelDBFSMetric, fmtDB, 2},
		{"dBTP", truePeakDBTPMetric, fmtPeakDB, 2},
		{"LUFS", momentaryLUFSMetric, fmtLUFS, 2},
		{"Hz", spectralCentroidHzMetric, fmtSpectral, 2},
		{"unit-less", flatnessMetric, fmtSpectral, 4},
		{"count", intervalCountMetric, fmtRaw, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			format, decimals := unitMetricFormat(tc.metric)
			if format != tc.wantFormat {
				t.Errorf("unitMetricFormat(%q) format = %d, want %d", tc.metric.key, format, tc.wantFormat)
			}
			if decimals != tc.wantDecimals {
				t.Errorf("unitMetricFormat(%q) decimals = %d, want %d", tc.metric.key, decimals, tc.wantDecimals)
			}
		})
	}
}

// TestUnitMetricFormatUncataloguedPanics pins the no-definition panic: a key with
// no catalogue entry must fail loudly rather than mis-format on a zero-value Unit.
func TestUnitMetricFormatUncataloguedPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("unitMetricFormat on an uncatalogued key did not panic")
		}
	}()
	unitMetricFormat(metricDescriptor{key: metricKey("not_a_real_key")})
}

// TestMetricLabelUncataloguedPanics pins the metricLabel no-definition panic: a
// key with no catalogue entry must fail loudly rather than return the raw key.
func TestMetricLabelUncataloguedPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("metricLabel on an uncatalogued key did not panic")
		}
	}()
	metricLabel(metricDescriptor{key: metricKey("not_a_real_key")})
}
