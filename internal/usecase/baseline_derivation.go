package usecase

// Derivasi field baseline dari nilai mentah. Faskes hanya menginput angka ukur/lab; kategori
// klinis (bmi_category, central_obesity, hypertension_status, diabetes_status) dan pemetaan
// diagnosis→cluster dihitung di sini agar tidak perlu diketik/di-OCR. Nilai string kanonik
// bebas didefinisikan di sini — hanya disimpan & ditampilkan, tak ada consumer yang mem-parse
// nilai spesifiknya (lihat docs/erd.md: baseline = konteks klinis + fitur ML).

// deriveBMICategory memetakan BMI ke enum baseline (underweight/normal/overweight/obese)
// memakai cutoff WHO Asia-Pasifik yang lebih ketat dari cutoff global.
func deriveBMICategory(bmi float64) string {
	switch {
	case bmi < 18.5:
		return "underweight"
	case bmi < 23.0:
		return "normal"
	case bmi < 25.0:
		return "overweight"
	default:
		return "obese"
	}
}

// deriveCentralObesity menerapkan ambang lingkar pinggang IDF Asia
// (pria ≥ 90 cm, wanita ≥ 80 cm).
func deriveCentralObesity(waistCm float64, sex string) bool {
	if sex == "female" {
		return waistCm >= 80.0
	}
	return waistCm >= 90.0
}

// deriveHypertensionStatus mengklasifikasi tekanan darah memakai kategori ACC/AHA 2017;
// kategori tertinggi antara sistolik/diastolik yang menang.
func deriveHypertensionStatus(systolic, diastolic int) string {
	switch {
	case systolic >= 140 || diastolic >= 90:
		return "stage_2"
	case systolic >= 130 || diastolic >= 80:
		return "stage_1"
	case systolic >= 120:
		return "elevated"
	default:
		return "normal"
	}
}

// deriveDiabetesStatus mengklasifikasi status diabetes dari HbA1c (fallback gula darah puasa)
// memakai ambang ADA: normal / prediabetes / diabetes.
func deriveDiabetesStatus(hba1cPct, fastingGlucoseMgdl float64) string {
	switch {
	case hba1cPct >= 6.5 || fastingGlucoseMgdl >= 126:
		return "diabetes"
	case hba1cPct >= 5.7 || fastingGlucoseMgdl >= 100:
		return "prediabetes"
	default:
		return "normal"
	}
}

// deriveDiagnosis me-resolve dropdown Diagnosis tunggal menjadi kolom cluster_id +
// diagnosis_cluster. override (dari request) menang; bila kosong, default dari disease_type
// pasien. Mengembalikan (clusterID, label); (0, "") bila tak dikenali.
func deriveDiagnosis(diseaseType string, override *string) (int, string) {
	key := diseaseType
	if override != nil && *override != "" {
		key = *override
	}
	switch key {
	case "diabetes", "diabetes_t2":
		return 1, "Diabetes"
	case "hipertensi", "hypertension":
		return 2, "Hipertensi"
	case "komplikasi", "both":
		return 3, "Komplikasi"
	default:
		return 0, ""
	}
}
