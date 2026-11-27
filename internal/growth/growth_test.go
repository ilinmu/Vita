package growth

import (
	"math"
	"testing"

	"github.com/ilinmu/vita/internal/model"
)

func TestPercentileRoundtripAndOrdering(t *testing.T) {
	stds := []Standard{StandardChina, StandardWHO}
	genders := []model.Gender{model.GenderMale, model.GenderFemale}
	metrics := []Metric{MetricHeight, MetricWeight, MetricHeadCirc, MetricBMI}
	ages := []float64{0, 6, 12, 24, 36, 60}

	for _, std := range stds {
		for _, g := range genders {
			for _, m := range metrics {
				for _, age := range ages {
					lms, ok := InterpolateLMS(std, g, m, age)
					if !ok {
						t.Fatalf("missing LMS for %s %s %s at %.1fm", std, g, m, age)
					}
					p3 := ValueAtZ(lms, zP3)
					p15 := ValueAtZ(lms, zP15)
					p50 := ValueAtZ(lms, zP50)
					p85 := ValueAtZ(lms, zP85)
					p97 := ValueAtZ(lms, zP97)
					if !(p3 < p15 && p15 < p50 && p50 < p85 && p85 < p97) {
						t.Fatalf("ordering failed (%s %s %s %.1fm): %v %v %v %v %v", std, g, m, age, p3, p15, p50, p85, p97)
					}
					pct, z, ok := Percentile(std, g, m, age, p50)
					if !ok || math.Abs(z) > 1e-4 || math.Abs(pct-50) > 1e-2 {
						t.Fatalf("median roundtrip failed: pct=%.4f z=%.6f ok=%v", pct, z, ok)
					}
					_, z85, ok := Percentile(std, g, m, age, p85)
					if !ok || math.Abs(z85-zP85) > 1e-4 {
						t.Fatalf("p85 roundtrip failed: got z=%.6f want=%.6f", z85, zP85)
					}
				}
			}
		}
	}
}

func TestCurveSeriesMonotonicGrowth(t *testing.T) {
	cs := CurveSeries(StandardChina, model.GenderMale, MetricHeight, 0, 144, 49)
	if len(cs.Ages) != 49 {
		t.Fatalf("expected 49 points, got %d", len(cs.Ages))
	}
	for i := 1; i < len(cs.P50); i++ {
		if cs.P50[i] <= cs.P50[i-1] {
			t.Fatalf("height P50 not strictly increasing at step %d: %.2f -> %.2f", i, cs.P50[i-1], cs.P50[i])
		}
	}
}

func TestBenchmarksSanity(t *testing.T) {
	// 12-month boy height in China standard should be ~76.5 cm at P50
	lms, ok := InterpolateLMS(StandardChina, model.GenderMale, MetricHeight, 12)
	if !ok {
		t.Fatal("missing 12m China boy height")
	}
	if math.Abs(lms.M-76.5) > 0.5 {
		t.Fatalf("unexpected 12m height M: %.2f", lms.M)
	}
	// 0-month WHO girl weight should be ~3.23 kg at P50
	lmsW, ok := InterpolateLMS(StandardWHO, model.GenderFemale, MetricWeight, 0)
	if !ok {
		t.Fatal("missing 0m WHO girl weight")
	}
	if math.Abs(lmsW.M-3.23) > 0.1 {
		t.Fatalf("unexpected 0m WHO girl weight M: %.2f", lmsW.M)
	}
}
