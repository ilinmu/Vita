// Package growth provides China (WS/T 423-2022 / WS/T 612) and WHO
// pediatric LMS growth reference standards, Z-score / percentile calculation,
// and reference percentile curve generation.
package growth

import (
	"math"

	"github.com/ilinmu/vita/internal/model"
)

// Standard identifies a reference growth standard.
type Standard string

const (
	StandardChina Standard = "china"
	StandardWHO   Standard = "who"
)

// Metric identifies a built-in anthropometric metric.
type Metric string

const (
	MetricHeight   Metric = "height"
	MetricWeight   Metric = "weight"
	MetricHeadCirc Metric = "head_circ"
	MetricBMI      Metric = "bmi"
)

// LMSPoint holds Box-Cox LMS parameters at a given age in months.
type LMSPoint struct {
	AgeMonths float64
	L         float64
	M         float64
	S         float64
}

// PercentileCurves holds reference curve series across sampled age points.
type PercentileCurves struct {
	Ages []float64
	P3   []float64
	P15  []float64
	P50  []float64
	P85  []float64
	P97  []float64
}

const (
	zP3  = -1.8807936
	zP15 = -1.0364334
	zP50 = 0.0
	zP85 = 1.0364334
	zP97 = 1.8807936
)

func selectTable(std Standard, gender model.Gender, metric Metric) []LMSPoint {
	male := gender != model.GenderFemale
	switch std {
	case StandardWHO:
		switch metric {
		case MetricHeight:
			if male {
				return whoBoysHeight
			}
			return whoGirlsHeight
		case MetricWeight:
			if male {
				return whoBoysWeight
			}
			return whoGirlsWeight
		case MetricHeadCirc:
			if male {
				return whoBoysHead
			}
			return whoGirlsHead
		case MetricBMI:
			if male {
				return whoBoysBMI
			}
			return whoGirlsBMI
		}
	default: // StandardChina
		switch metric {
		case MetricHeight:
			if male {
				return chinaBoysHeight
			}
			return chinaGirlsHeight
		case MetricWeight:
			if male {
				return chinaBoysWeight
			}
			return chinaGirlsWeight
		case MetricHeadCirc:
			if male {
				return chinaBoysHead
			}
			return chinaGirlsHead
		case MetricBMI:
			if male {
				return chinaBoysBMI
			}
			return chinaGirlsBMI
		}
	}
	return nil
}

// MaxAgeMonths returns the upper age bound (in months) supported by the standard & metric.
func MaxAgeMonths(std Standard, gender model.Gender, metric Metric) float64 {
	tbl := selectTable(std, gender, metric)
	if len(tbl) == 0 {
		return 0
	}
	return tbl[len(tbl)-1].AgeMonths
}

// InterpolateLMS returns linearly interpolated LMS parameters at ageMonths.
func InterpolateLMS(std Standard, gender model.Gender, metric Metric, ageMonths float64) (LMSPoint, bool) {
	tbl := selectTable(std, gender, metric)
	if len(tbl) == 0 || ageMonths < tbl[0].AgeMonths || ageMonths > tbl[len(tbl)-1].AgeMonths {
		return LMSPoint{}, false
	}
	if ageMonths == tbl[0].AgeMonths {
		return tbl[0], true
	}
	last := tbl[len(tbl)-1]
	if ageMonths == last.AgeMonths {
		return last, true
	}
	for i := 1; i < len(tbl); i++ {
		if ageMonths <= tbl[i].AgeMonths {
			a, b := tbl[i-1], tbl[i]
			span := b.AgeMonths - a.AgeMonths
			if span <= 0 {
				return a, true
			}
			t := (ageMonths - a.AgeMonths) / span
			return LMSPoint{
				AgeMonths: ageMonths,
				L:         a.L + (b.L-a.L)*t,
				M:         a.M + (b.M-a.M)*t,
				S:         a.S + (b.S-a.S)*t,
			}, true
		}
	}
	return LMSPoint{}, false
}

// ValueAtZ converts a Z-score into a physical value using the LMS formula.
func ValueAtZ(lms LMSPoint, z float64) float64 {
	if math.Abs(lms.L) < 1e-6 {
		return lms.M * math.Exp(lms.S*z)
	}
	base := 1.0 + lms.L*lms.S*z
	if base <= 0 {
		return 0
	}
	return lms.M * math.Pow(base, 1.0/lms.L)
}

// ZScore converts a physical measurement into a standard normal Z-score via LMS.
func ZScore(lms LMSPoint, value float64) float64 {
	if value <= 0 || lms.M <= 0 || lms.S <= 0 {
		return 0
	}
	if math.Abs(lms.L) < 1e-6 {
		return math.Log(value/lms.M) / lms.S
	}
	return (math.Pow(value/lms.M, lms.L) - 1.0) / (lms.L * lms.S)
}

// Percentile computes the reference percentile (0..100) and Z-score for value at ageMonths.
func Percentile(std Standard, gender model.Gender, metric Metric, ageMonths, value float64) (pct float64, z float64, ok bool) {
	if value <= 0 {
		return 0, 0, false
	}
	lms, found := InterpolateLMS(std, gender, metric, ageMonths)
	if !found {
		return 0, 0, false
	}
	z = ZScore(lms, value)
	pct = 0.5 * (1.0 + math.Erf(z/math.Sqrt2)) * 100.0
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, z, true
}

// CurveSeries samples P3, P15, P50, P85, and P97 curves over [minMonth, maxMonth].
func CurveSeries(std Standard, gender model.Gender, metric Metric, minMonth, maxMonth float64, steps int) PercentileCurves {
	tbl := selectTable(std, gender, metric)
	if len(tbl) == 0 || steps < 2 {
		return PercentileCurves{}
	}
	lo := math.Max(minMonth, tbl[0].AgeMonths)
	hi := math.Min(maxMonth, tbl[len(tbl)-1].AgeMonths)
	if hi <= lo {
		return PercentileCurves{}
	}
	out := PercentileCurves{
		Ages: make([]float64, 0, steps),
		P3:   make([]float64, 0, steps),
		P15:  make([]float64, 0, steps),
		P50:  make([]float64, 0, steps),
		P85:  make([]float64, 0, steps),
		P97:  make([]float64, 0, steps),
	}
	for i := 0; i < steps; i++ {
		age := lo + (hi-lo)*float64(i)/float64(steps-1)
		lms, ok := InterpolateLMS(std, gender, metric, age)
		if !ok {
			continue
		}
		out.Ages = append(out.Ages, age)
		out.P3 = append(out.P3, ValueAtZ(lms, zP3))
		out.P15 = append(out.P15, ValueAtZ(lms, zP15))
		out.P50 = append(out.P50, ValueAtZ(lms, zP50))
		out.P85 = append(out.P85, ValueAtZ(lms, zP85))
		out.P97 = append(out.P97, ValueAtZ(lms, zP97))
	}
	return out
}
