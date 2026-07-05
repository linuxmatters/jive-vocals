package processor

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	ffmpeg "github.com/linuxmatters/ffmpeg-statigo"
)

const spectralTestEpsilon = 1e-9

var staleSpectralPrimitiveFields = func() []string {
	fields := make([]string, 0, len(spectralMetricDescriptors))
	for _, descriptor := range spectralMetricDescriptors {
		fields = append(fields, "Spectral"+descriptor.name)
	}
	return fields
}()

func spectralMetricsFromValues(values ...float64) SpectralMetrics {
	metrics := SpectralMetrics{Found: true}
	for index, descriptor := range spectralMetricDescriptors {
		if index < len(values) {
			*descriptor.metricField(&metrics) = values[index]
		}
	}
	return metrics
}

func assertSpectralMetricsApprox(t *testing.T, got SpectralMetrics, want SpectralMetrics) {
	t.Helper()

	for _, descriptor := range spectralMetricDescriptors {
		gotValue := *descriptor.metricField(&got)
		wantValue := *descriptor.metricField(&want)
		if math.Abs(gotValue-wantValue) > spectralTestEpsilon {
			t.Errorf("%s: got %v, want %v", descriptor.name, gotValue, wantValue)
		}
	}
}

func TestFinalizeSpectral_ZeroFrameCount(t *testing.T) {
	acc := &baseMetadataAccumulators{}
	result := acc.finalizeSpectral()

	if result != (SpectralMetrics{}) {
		t.Errorf("expected zero-value SpectralMetrics, got %+v", result)
	}
}

func TestFinalizeSpectral_AveragesCorrectly(t *testing.T) {
	acc := &baseMetadataAccumulators{}
	acc.accumulateSpectral(spectralMetricsFromValues(2.0, 4.0, 1000.0, 200.0, 1.0, 2.0, 0.25, 0.10, 1.0, 0.5, -0.005, 0.1, 2000.0))
	acc.accumulateSpectral(spectralMetricsFromValues(8.0, 16.0, 2000.0, 400.0, 3.0, 6.0, 1.25, 0.40, 5.0, 1.5, -0.015, 0.3, 6000.0))

	result := acc.finalizeSpectral()

	assertSpectralMetricsApprox(t, result, spectralMetricsFromValues(5.0, 10.0, 1500.0, 300.0, 2.0, 4.0, 0.75, 0.25, 3.0, 1.0, -0.01, 0.2, 4000.0))
}

func TestFinalizeSpectral_AssignsBaseSpectral(t *testing.T) {
	acc := &baseMetadataAccumulators{}
	for range 3 {
		acc.accumulateSpectral(spectralMetricsFromValues(10.0, 20.0, 3000.0, 500.0, 2.0, 4.0, 0.7, 0.3, 5.0, 1.0, -0.02, 0.4, 8000.0))
	}

	spectral := acc.finalizeSpectral()

	assertSpectralMetricsApprox(t, spectral, spectralMetricsFromValues(10.0, 20.0, 3000.0, 500.0, 2.0, 4.0, 0.7, 0.3, 5.0, 1.0, -0.02, 0.4, 8000.0))
}

func TestSpectralAccumulator_ZeroFrameCount(t *testing.T) {
	var acc SpectralAccumulator

	if acc.Found() {
		t.Fatal("expected Found to be false before adding spectral metrics")
	}
	if got := acc.Average(); got != (SpectralMetrics{}) {
		t.Fatalf("Average() = %+v, want zero-value SpectralMetrics", got)
	}
}

func TestSpectralAccumulator_MixedFoundAndUnfound(t *testing.T) {
	var acc SpectralAccumulator

	acc.Add(SpectralMetrics{
		Mean:     100.0,
		Variance: 200.0,
		Found:    false,
	})
	acc.Add(SpectralMetrics{
		Mean:     10.0,
		Variance: 20.0,
		Found:    true,
	})

	if !acc.Found() {
		t.Fatal("expected Found to be true after adding found spectral metrics")
	}

	average := acc.Average()
	if !average.Found {
		t.Fatal("expected averaged SpectralMetrics to preserve Found")
	}
	if average.Mean != 10.0 {
		t.Errorf("Mean = %v, want 10.0", average.Mean)
	}
	if average.Variance != 20.0 {
		t.Errorf("Variance = %v, want 20.0", average.Variance)
	}
}

func TestSpectralAccumulator_AveragesAllFields(t *testing.T) {
	var acc SpectralAccumulator
	acc.Add(spectralMetricsFromValues(2.0, 4.0, 1000.0, 200.0, 1.0, 2.0, 0.2, 0.4, 6.0, 0.02, -0.10, 0.06, 5000.0))
	acc.Add(spectralMetricsFromValues(6.0, 12.0, 3000.0, 600.0, 3.0, 6.0, 0.6, 0.8, 10.0, 0.06, -0.30, 0.18, 9000.0))

	result := acc.Average()

	assertSpectralMetricsApprox(t, result, spectralMetricsFromValues(4.0, 8.0, 2000.0, 400.0, 2.0, 4.0, 0.4, 0.6, 8.0, 0.04, -0.20, 0.12, 7000.0))
}

func TestBaseMetadataAccumulators_UsesSingleSpectralAccumulator(t *testing.T) {
	accType := reflect.TypeFor[baseMetadataAccumulators]()
	var spectralFields []reflect.StructField
	for field := range accType.Fields() {
		if strings.HasPrefix(field.Name, "spectral") {
			spectralFields = append(spectralFields, field)
		}
	}

	if len(spectralFields) != 1 {
		t.Fatalf("baseMetadataAccumulators spectral field count = %d, want 1", len(spectralFields))
	}
	if spectralFields[0].Type != reflect.TypeFor[SpectralAccumulator]() {
		t.Fatalf("spectral field type = %v, want SpectralAccumulator", spectralFields[0].Type)
	}
}

func TestIntervalSample_UsesSingleSpectralMetricsField(t *testing.T) {
	sampleType := reflect.TypeFor[IntervalSample]()
	var spectralFields []reflect.StructField
	for field := range sampleType.Fields() {
		if strings.HasPrefix(field.Name, "Spectral") {
			spectralFields = append(spectralFields, field)
		}
	}

	if len(spectralFields) != 1 {
		t.Fatalf("IntervalSample spectral field count = %d, want 1", len(spectralFields))
	}
	if spectralFields[0].Name != "Spectral" {
		t.Fatalf("spectral field name = %s, want Spectral", spectralFields[0].Name)
	}
	if spectralFields[0].Type != reflect.TypeFor[SpectralMetrics]() {
		t.Fatalf("spectral field type = %v, want SpectralMetrics", spectralFields[0].Type)
	}
}

func TestIntervalSample_HasNoFlatSpectralPrimitiveFields(t *testing.T) {
	assertNoStaleSpectralPrimitiveFields[IntervalSample](t)
}

func TestIntervalFrameMetrics_UsesSingleSpectralMetricsField(t *testing.T) {
	metricsType := reflect.TypeFor[intervalFrameMetrics]()
	var spectralFields []reflect.StructField
	for field := range metricsType.Fields() {
		if strings.HasPrefix(field.Name, "Spectral") {
			spectralFields = append(spectralFields, field)
		}
	}

	if len(spectralFields) != 1 {
		t.Fatalf("intervalFrameMetrics spectral field count = %d, want 1", len(spectralFields))
	}
	if spectralFields[0].Name != "Spectral" {
		t.Fatalf("spectral field name = %s, want Spectral", spectralFields[0].Name)
	}
	if spectralFields[0].Type != reflect.TypeFor[SpectralMetrics]() {
		t.Fatalf("spectral field type = %v, want SpectralMetrics", spectralFields[0].Type)
	}
}

// newMetadataDict builds an *ffmpeg.AVDictionary from key/value string pairs and
// registers its cleanup. Keys absent from the map are absent from the dict, so a
// caller can model a frame that "misses" a key. The values are raw decimal text,
// exactly as FFmpeg's astats/ebur128 filters emit them into frame metadata.
func newMetadataDict(t *testing.T, pairs map[string]string) *ffmpeg.AVDictionary {
	t.Helper()

	var dict *ffmpeg.AVDictionary
	for k, v := range pairs {
		key := ffmpeg.ToCStr(k)
		value := ffmpeg.ToCStr(v)
		if _, err := ffmpeg.AVDictSet(&dict, key, value, 0); err != nil {
			key.Free()
			value.Free()
			ffmpeg.AVDictFree(&dict)
			t.Fatalf("AVDictSet(%q) error = %v", k, err)
		}
		key.Free()
		value.Free()
	}
	t.Cleanup(func() { ffmpeg.AVDictFree(&dict) })
	return dict
}

func TestGetFloatMetadata_ParsesValueFromCBytes(t *testing.T) {
	dict := newMetadataDict(t, map[string]string{
		"lavfi.astats.1.Dynamic_range": "42.5",
		"lavfi.astats.1.RMS_level":     "-23.456789",
		"lavfi.astats.1.Min_level":     "-0.5",
	})

	cases := []struct {
		name string
		key  *ffmpeg.CStr
		want float64
	}{
		{"Dynamic_range", metaKeyDynamicRange, 42.5},
		{"RMS_level", metaKeyRMSLevel, -23.456789},
		{"Min_level", metaKeyMinLevel, -0.5},
	}
	for _, c := range cases {
		got, ok := getFloatMetadata(dict, c.key)
		if !ok {
			t.Errorf("%s: ok = false, want true", c.name)
			continue
		}
		// Bit-identical to the strconv.ParseFloat the previous String() path fed.
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGetFloatMetadata_MissingKeyReportsNotFound(t *testing.T) {
	dict := newMetadataDict(t, map[string]string{
		"lavfi.astats.1.RMS_level": "-20.0",
	})

	if value, ok := getFloatMetadata(dict, metaKeyDynamicRange); ok {
		t.Errorf("missing key: got (%v, true), want (_, false)", value)
	}
}

func TestGetFloatMetadata_UnparseableValueReportsNotFound(t *testing.T) {
	dict := newMetadataDict(t, map[string]string{
		"lavfi.astats.1.Dynamic_range": "not-a-number",
	})

	if value, ok := getFloatMetadata(dict, metaKeyDynamicRange); ok {
		t.Errorf("unparseable value: got (%v, true), want (_, false)", value)
	}
}

func TestCStringNoCopyStopsAtNUL(t *testing.T) {
	raw := []byte{'-', '2', '3', '.', '5', 0, '9', '9'}

	got := cStringNoCopy(unsafe.Pointer(&raw[0]))

	if got != "-23.5" {
		t.Fatalf("cStringNoCopy() = %q, want %q", got, "-23.5")
	}
}

func TestCStringNoCopyCapsScan(t *testing.T) {
	raw := make([]byte, 300)
	for i := range raw {
		raw[i] = '7'
	}

	got := cStringNoCopy(unsafe.Pointer(&raw[0]))

	if len(got) != 256 {
		t.Fatalf("cStringNoCopy() length = %d, want 256", len(got))
	}
	if strings.Trim(got, "7") != "" {
		t.Fatalf("cStringNoCopy() = %q, want only 7 bytes", got)
	}
}

// TestExtractAstatsMetadata_LatestFoundPerKeyWins is the core semantic guard for
// task 1.2: astats values are cumulative ("latest non-missing wins"), so a late
// frame that misses a key must NOT clobber the value an earlier frame supplied,
// and the per-key found flag (astatsFound) must reflect any frame that supplied
// Dynamic_range.
func TestExtractAstatsMetadata_LatestFoundPerKeyWins(t *testing.T) {
	var acc baseMetadataAccumulators

	// Frame 1: supplies Dynamic_range and RMS_level.
	acc.extractAstatsMetadata(newMetadataDict(t, map[string]string{
		"lavfi.astats.1.Dynamic_range": "40.0",
		"lavfi.astats.1.RMS_level":     "-25.0",
		"lavfi.astats.1.RMS_trough":    "-60.0",
	}), optionalFloat{})

	// Frame 2 (later): supplies a newer RMS_level but MISSES Dynamic_range and
	// RMS_trough. The earlier values for the missed keys must be retained, and
	// the newer RMS_level must win.
	acc.extractAstatsMetadata(newMetadataDict(t, map[string]string{
		"lavfi.astats.1.RMS_level": "-22.0",
	}), optionalFloat{})

	if acc.astatsDynamicRange != 40.0 {
		t.Errorf("astatsDynamicRange = %v, want 40.0 (earlier frame retained)", acc.astatsDynamicRange)
	}
	if acc.astatsRMSLevel != -22.0 {
		t.Errorf("astatsRMSLevel = %v, want -22.0 (later frame wins)", acc.astatsRMSLevel)
	}
	if acc.astatsRMSTrough != -60.0 {
		t.Errorf("astatsRMSTrough = %v, want -60.0 (earlier frame retained)", acc.astatsRMSTrough)
	}
	if !acc.astatsFound {
		t.Error("astatsFound = false, want true (Dynamic_range was supplied)")
	}
}

// TestExtractAstatsMetadata_FoundFlagStaysFalseWithoutDynamicRange confirms the
// found flag tracks Dynamic_range specifically, matching the pre-change rule.
func TestExtractAstatsMetadata_FoundFlagStaysFalseWithoutDynamicRange(t *testing.T) {
	var acc baseMetadataAccumulators

	acc.extractAstatsMetadata(newMetadataDict(t, map[string]string{
		"lavfi.astats.1.RMS_level": "-20.0",
	}), optionalFloat{})

	if acc.astatsFound {
		t.Error("astatsFound = true, want false (no Dynamic_range supplied)")
	}
}

// TestExtractAstatsMetadata_AppliesConversions guards the dB conversions that sit
// on top of the raw parse (Crest_factor, Min_level, Max_level), so the no-copy
// parse change cannot silently shift a converted field.
func TestExtractAstatsMetadata_AppliesConversions(t *testing.T) {
	var acc baseMetadataAccumulators

	acc.extractAstatsMetadata(newMetadataDict(t, map[string]string{
		"lavfi.astats.1.Crest_factor": "10.0",
		"lavfi.astats.1.Min_level":    "-0.5",
		"lavfi.astats.1.Max_level":    "0.5",
	}), optionalFloat{})

	if want := linearRatioToDB(10.0); acc.astatsCrestFactor != want {
		t.Errorf("astatsCrestFactor = %v, want %v", acc.astatsCrestFactor, want)
	}
	if want := linearSampleToDBFS(-0.5); acc.astatsMinLevel != want {
		t.Errorf("astatsMinLevel = %v, want %v", acc.astatsMinLevel, want)
	}
	if want := linearSampleToDBFS(0.5); acc.astatsMaxLevel != want {
		t.Errorf("astatsMaxLevel = %v, want %v", acc.astatsMaxLevel, want)
	}
}

// BenchmarkGetFloatMetadata exercises the per-key parse on the hot Pass 1 path.
// The no-copy CStr view should report fewer allocs/op than the prior String()
// path. Run: go test -bench=BenchmarkGetFloatMetadata -benchmem.
func BenchmarkGetFloatMetadata(b *testing.B) {
	var dict *ffmpeg.AVDictionary
	key := ffmpeg.ToCStr("lavfi.astats.1.RMS_level")
	value := ffmpeg.ToCStr("-23.456789")
	if _, err := ffmpeg.AVDictSet(&dict, key, value, 0); err != nil {
		key.Free()
		value.Free()
		b.Fatalf("AVDictSet() error = %v", err)
	}
	key.Free()
	value.Free()
	b.Cleanup(func() { ffmpeg.AVDictFree(&dict) })

	b.ReportAllocs()
	var sink float64
	for b.Loop() {
		v, _ := getFloatMetadata(dict, metaKeyRMSLevel)
		sink += v
	}
	_ = sink
}

func assertNoStaleSpectralPrimitiveFields[T any](t *testing.T) {
	t.Helper()

	targetType := reflect.TypeFor[T]()
	for _, fieldName := range staleSpectralPrimitiveFields {
		if field, ok := targetType.FieldByName(fieldName); ok {
			t.Errorf("%s has stale flat spectral field %s with type %v", targetType.Name(), field.Name, field.Type)
		}
	}
}

func TestIntervalAccumulatorFinalize_WritesAveragedSpectralMetrics(t *testing.T) {
	acc := &intervalAccumulator{}
	acc.add(intervalFrameMetrics{
		Spectral: spectralMetricsFromValues(2.0, 4.0, 1000.0, 200.0, 1.0, 2.0, 0.2, 0.4, 6.0, 0.02, -0.10, 0.06, 5000.0),
	})
	acc.add(intervalFrameMetrics{
		Spectral: spectralMetricsFromValues(6.0, 12.0, 3000.0, 600.0, 3.0, 6.0, 0.6, 0.8, 10.0, 0.06, -0.30, 0.18, 9000.0),
	})

	result := acc.finalize(time.Second)

	if !result.Spectral.Found {
		t.Fatal("expected averaged interval spectral metrics to preserve Found")
	}
	assertSpectralMetricsApprox(t, result.Spectral, spectralMetricsFromValues(4.0, 8.0, 2000.0, 400.0, 2.0, 4.0, 0.4, 0.6, 8.0, 0.04, -0.20, 0.12, 7000.0))
}
