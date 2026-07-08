package usecase

import "testing"

func TestDeriveBMICategory(t *testing.T) {
	cases := []struct {
		bmi  float64
		want string
	}{
		{17.0, "underweight"},
		{18.4, "underweight"},
		{18.5, "normal"},
		{22.9, "normal"},
		{23.0, "overweight"},
		{24.9, "overweight"},
		{25.0, "obese"},
		{31.2, "obese"},
	}
	for _, c := range cases {
		if got := deriveBMICategory(c.bmi); got != c.want {
			t.Errorf("deriveBMICategory(%.1f) = %q; want %q", c.bmi, got, c.want)
		}
	}
}

func TestDeriveCentralObesity(t *testing.T) {
	cases := []struct {
		waist float64
		sex   string
		want  bool
	}{
		{89.9, "male", false},
		{90.0, "male", true},
		{79.9, "female", false},
		{80.0, "female", true},
		{85.0, "male", false},  // di bawah ambang pria
		{85.0, "female", true}, // di atas ambang wanita
	}
	for _, c := range cases {
		if got := deriveCentralObesity(c.waist, c.sex); got != c.want {
			t.Errorf("deriveCentralObesity(%.1f, %q) = %v; want %v", c.waist, c.sex, got, c.want)
		}
	}
}

func TestDeriveHypertensionStatus(t *testing.T) {
	cases := []struct {
		sys, dia int
		want     string
	}{
		{110, 70, "normal"},
		{119, 79, "normal"},
		{120, 78, "elevated"},
		{129, 79, "elevated"},
		{130, 78, "stage_1"},
		{118, 82, "stage_1"}, // diastolik memicu stage_1
		{140, 80, "stage_2"},
		{120, 92, "stage_2"}, // diastolik memicu stage_2
	}
	for _, c := range cases {
		if got := deriveHypertensionStatus(c.sys, c.dia); got != c.want {
			t.Errorf("deriveHypertensionStatus(%d,%d) = %q; want %q", c.sys, c.dia, got, c.want)
		}
	}
}

func TestDeriveDiabetesStatus(t *testing.T) {
	cases := []struct {
		hba1c, fpg float64
		want       string
	}{
		{5.4, 90, "normal"},
		{5.7, 90, "prediabetes"},
		{5.4, 100, "prediabetes"}, // gula puasa memicu prediabetes
		{6.4, 120, "prediabetes"},
		{6.5, 120, "diabetes"},
		{5.4, 126, "diabetes"}, // gula puasa memicu diabetes
	}
	for _, c := range cases {
		if got := deriveDiabetesStatus(c.hba1c, c.fpg); got != c.want {
			t.Errorf("deriveDiabetesStatus(%.1f,%.0f) = %q; want %q", c.hba1c, c.fpg, got, c.want)
		}
	}
}

func TestDeriveDiagnosis(t *testing.T) {
	komplikasi := "komplikasi"
	cases := []struct {
		diseaseType string
		override    *string
		wantID      int
		wantLabel   string
	}{
		{"diabetes_t2", nil, 1, "Diabetes"},
		{"hypertension", nil, 2, "Hipertensi"},
		{"both", nil, 3, "Komplikasi"},
		{"diabetes_t2", &komplikasi, 3, "Komplikasi"}, // override menang
		{"unknown", nil, 0, ""},
	}
	for _, c := range cases {
		gotID, gotLabel := deriveDiagnosis(c.diseaseType, c.override)
		if gotID != c.wantID || gotLabel != c.wantLabel {
			t.Errorf("deriveDiagnosis(%q, %v) = (%d,%q); want (%d,%q)",
				c.diseaseType, c.override, gotID, gotLabel, c.wantID, c.wantLabel)
		}
	}
}
