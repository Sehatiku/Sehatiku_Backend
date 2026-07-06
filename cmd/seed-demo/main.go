// cmd/seed-demo/main.go
//
// Seed data realistis untuk video demo Sehatiku.
//
// Yang di-seed:
//   - 1 faskes (Puskesmas Cempaka Putih)
//   - 2 nakes  (1 dokter, 1 kader)
//   - 6 pasien  (mix diabetes / hipertensi / both, berbagai profil risiko)
//   - patient_clinical_baselines per pasien
//   - health_logs 14 hari terakhir (glukosa, tekanan darah, aktivitas, tidur, stres)
//   - lab_results (HbA1c, LDL, eGFR)
//   - daily_features (7 hari terakhir)
//   - risk_scores  (berkorelasi dengan daily_features)
//   - escalations  (pasien risiko bahaya / waswas)
//
// Idempoten: jalankan berulang kali aman (ON CONFLICT DO NOTHING pada user;
// health_logs/risk_scores/escalations selalu fresh setelah truncate).
//
// Jalankan dari root repo:
//
//	go run ./cmd/seed-demo

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"sehatiku-backend/internal/config"
	"sehatiku-backend/internal/entity"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

// ── helpers ─────────────────────────────────────────────────────────────────

func ptr[T any](v T) *T { return &v }

func hashPassword(plain string) string {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), 10)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func daysAgo(n int) time.Time {
	return time.Now().AddDate(0, 0, -n).Truncate(24 * time.Hour)
}

func daysAgoAt(n, hour, min int) time.Time {
	base := daysAgo(n)
	return time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, base.Location())
}

func topFactors(factors []string) json.RawMessage {
	b, _ := json.Marshal(factors)
	return b
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	cfg := config.NewViper()
	log := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, log)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║         SEHATIKU — Seed Demo Data                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// ── 0. RESET TRANSACTIONAL DATA ───────────────────────────────────────────
	fmt.Println("▶ [0/8] Resetting operational & ML data...")
	tablesToTruncate := []string{
		"patient_notifications",
		"consultations",
		"notifications",
		"device_push_tokens",
		"escalations",
		"risk_scores",
		"daily_features",
		"patient_clinical_baselines",
		"lab_results",
		"health_logs",
		"model_versions",
		"patients",
		"nakes",
	}

	mainDB := db
	tx := mainDB.Begin()
	if tx.Error != nil {
		log.Fatal("failed to begin transaction", zap.Error(tx.Error))
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	db = tx // Reassign db to the transaction instance so all subsequent code uses it

	for _, table := range tablesToTruncate {
		sql := fmt.Sprintf(`TRUNCATE TABLE "%s" RESTART IDENTITY CASCADE`, table)
		if err := db.Exec(sql).Error; err != nil {
			db.Rollback()
			log.Fatal("reset table failed", zap.String("table", table), zap.Error(err))
		}
	}

	// ── 1. FASKES ─────────────────────────────────────────────────────────────
	fmt.Println("▶ [1/8] Seeding faskes...")
	faskes := entity.Faskes{
		ID:           "11111111-0000-0000-0000-000000000001",
		Name:         "Puskesmas Cempaka Putih",
		Type:         "puskesmas",
		Address:      "Jl. Cempaka Putih Tengah No. 1, Jakarta Pusat",
		Region:       "Jakarta Pusat",
		Status:       "active",
		Username:     "puskesmas.cempaka",
		PasswordHash: hashPassword("asdasdasd"),
		PhoneNumber:  "6281234560001",
	}
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&faskes).Error; err != nil {
		log.Fatal("seed faskes", zap.Error(err))
	}

	// ── 2. NAKES ──────────────────────────────────────────────────────────────
	fmt.Println("▶ [2/8] Seeding nakes...")
	dokter1ID := "22222222-0000-0000-0000-000000000001"
	dokter2ID := "22222222-0000-0000-0000-000000000002"
	dokter3ID := "22222222-0000-0000-0000-000000000003"

	nakesList := []entity.Nakes{
		{
			ID:           dokter1ID,
			FaskesID:     faskes.ID,
			Username:     "dr.rani",
			PasswordHash: hashPassword("asdasdasd"),
			FullName:     "dr. Rani Kusuma Dewi",
			Role:         "dokter",
			NIK:          "3171010101850001",
			Alamat:       "Jl. Mawar No. 12, Jakarta Pusat",
			PhoneNumber:  "6281234560010",
			Status:       "active",
		},
		{
			ID:           dokter2ID,
			FaskesID:     faskes.ID,
			Username:     "dr.budi",
			PasswordHash: hashPassword("asdasdasd"),
			FullName:     "dr. Budi Santoso",
			Role:         "dokter",
			NIK:          "3171010201900002",
			Alamat:       "Jl. Melati No. 5, Jakarta Pusat",
			PhoneNumber:  "6281234560011",
			Status:       "active",
		},
		{
			ID:           dokter3ID,
			FaskesID:     faskes.ID,
			Username:     "dr.siti",
			PasswordHash: hashPassword("asdasdasd"),
			FullName:     "dr. Siti Aminah",
			Role:         "dokter",
			NIK:          "3171010301880003",
			Alamat:       "Jl. Dahlia No. 8, Jakarta Pusat",
			PhoneNumber:  "6281234560012",
			Status:       "active",
		},
	}
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&nakesList).Error; err != nil {
		log.Fatal("seed nakes", zap.Error(err))
	}

	// ── 3. PASIEN ─────────────────────────────────────────────────────────────
	fmt.Println("▶ [3/8] Seeding patients...")

	type patientSeed struct {
		entity.Patient
		plainPass string
	}

	dob := func(year, month, day int) *time.Time {
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		return &t
	}

	patients := []patientSeed{
		// 1 — Diabetes T2, risiko BAHAYA (glukosa tidak terkontrol)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000001", FaskesID: faskes.ID,
				AssignedNakesID: dokter1ID, Username: "pasien.suharto",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Suharto Wibowo",
				NIK: "3171011001580001", Alamat: "Jl. Kenanga No. 3, Jakarta Pusat",
				PhoneNumber: "6281100001001", CompanionName: "Dewi Wibowo",
				CompanionPhone: "6281100001002", DateOfBirth: dob(1958, 10, 10),
				Sex: "male", DiseaseType: "diabetes_t2", Status: "active",
				EnrolledAt: daysAgo(60),
			},
			plainPass: "asdasdasd",
		},
		// 2 — Hipertensi, risiko BAHAYA (sistolik konsisten tinggi)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000002", FaskesID: faskes.ID,
				AssignedNakesID: dokter1ID, Username: "pasien.hartini",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Hartini Susanto",
				NIK: "3171015505620002", Alamat: "Jl. Anggrek No. 7, Jakarta Pusat",
				PhoneNumber: "6281100001003", CompanionName: "Eko Susanto",
				CompanionPhone: "6281100001004", DateOfBirth: dob(1962, 5, 15),
				Sex: "female", DiseaseType: "hypertension", Status: "active",
				EnrolledAt: daysAgo(45),
			},
			plainPass: "asdasdasd",
		},
		// 3 — Both (DM+HT), risiko WASWAS (sedang, tren memburuk)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000003", FaskesID: faskes.ID,
				AssignedNakesID: dokter1ID, Username: "pasien.agus",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Agus Priyono",
				NIK: "3171012303650003", Alamat: "Jl. Dahlia No. 11, Jakarta Pusat",
				PhoneNumber: "6281100001005", CompanionName: "Siti Priyono",
				CompanionPhone: "6281100001006", DateOfBirth: dob(1965, 3, 23),
				Sex: "male", DiseaseType: "both", Status: "active",
				EnrolledAt: daysAgo(30),
			},
			plainPass: "asdasdasd",
		},
		// 4 — Diabetes T2, risiko AMAN (terkontrol baik)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000004", FaskesID: faskes.ID,
				AssignedNakesID: dokter1ID, Username: "pasien.yuli",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Yuliana Putri",
				NIK: "3171012806750004", Alamat: "Jl. Tulip No. 2, Jakarta Pusat",
				PhoneNumber: "6281100001007", CompanionName: "Hendra Putri",
				CompanionPhone: "6281100001008", DateOfBirth: dob(1975, 6, 28),
				Sex: "female", DiseaseType: "diabetes_t2", Status: "active",
				EnrolledAt: daysAgo(90),
			},
			plainPass: "asdasdasd",
		},
		// 5 — Hipertensi, risiko AMAN (patuh obat, gaya hidup baik)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000005", FaskesID: faskes.ID,
				AssignedNakesID: dokter2ID, Username: "pasien.bambang",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Bambang Irianto",
				NIK: "3171011109720005", Alamat: "Jl. Seruni No. 8, Jakarta Pusat",
				PhoneNumber: "6281100001009", CompanionName: "Ratna Irianto",
				CompanionPhone: "6281100001010", DateOfBirth: dob(1972, 9, 11),
				Sex: "male", DiseaseType: "hypertension", Status: "active",
				EnrolledAt: daysAgo(120),
			},
			plainPass: "asdasdasd",
		},
		// 6 — Both, risiko WASWAS (baru enroll, data terbatas)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000006", FaskesID: faskes.ID,
				AssignedNakesID: dokter2ID, Username: "pasien.mirna",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Mirnawati Halim",
				NIK: "3171012108800006", Alamat: "Jl. Flamboyan No. 14, Jakarta Pusat",
				PhoneNumber: "6281100001011", CompanionName: "Halim Halim",
				CompanionPhone: "6281100001012", DateOfBirth: dob(1980, 8, 21),
				Sex: "female", DiseaseType: "both", Status: "active",
				EnrolledAt: daysAgo(7),
			},
			plainPass: "asdasdasd",
		},
		// 7 — Diabetes T2, risiko AMAN (patuh diet, olah raga)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000007", FaskesID: faskes.ID,
				AssignedNakesID: dokter2ID, Username: "pasien.lukman",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Lukman Hakim",
				NIK: "3171011204700007", Alamat: "Jl. Dahlia No. 15, Jakarta Pusat",
				PhoneNumber: "6281100001013", CompanionName: "Indah Hakim",
				CompanionPhone: "6281100001014", DateOfBirth: dob(1970, 4, 12),
				Sex: "male", DiseaseType: "diabetes_t2", Status: "active",
				EnrolledAt: daysAgo(40),
			},
			plainPass: "asdasdasd",
		},
		// 8 — Hipertensi, risiko WASWAS (tekanan darah naik turun)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000008", FaskesID: faskes.ID,
				AssignedNakesID: dokter3ID, Username: "pasien.sitirahma",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Siti Rahma",
				NIK: "3171016511730008", Alamat: "Jl. Melati No. 8, Jakarta Pusat",
				PhoneNumber: "6281100001015", CompanionName: "Ali Rahma",
				CompanionPhone: "6281100001016", DateOfBirth: dob(1973, 11, 25),
				Sex: "female", DiseaseType: "hypertension", Status: "active",
				EnrolledAt: daysAgo(25),
			},
			plainPass: "asdasdasd",
		},
		// 9 — Both, risiko BAHAYA (kombinasi DM & HT tidak terkontrol)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000009", FaskesID: faskes.ID,
				AssignedNakesID: dokter3ID, Username: "pasien.ahmad",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Ahmad Wijaya",
				NIK: "3171011802600009", Alamat: "Jl. Kamboja No. 9, Jakarta Pusat",
				PhoneNumber: "6281100001017", CompanionName: "Siti Wijaya",
				CompanionPhone: "6281100001018", DateOfBirth: dob(1960, 2, 18),
				Sex: "male", DiseaseType: "both", Status: "active",
				EnrolledAt: daysAgo(50),
			},
			plainPass: "asdasdasd",
		},
		// 10 — Diabetes T2, risiko AMAN (terkendali)
		{
			Patient: entity.Patient{
				ID: "33333333-0000-0000-0000-000000000010", FaskesID: faskes.ID,
				AssignedNakesID: dokter3ID, Username: "pasien.rina",
				PasswordHash: hashPassword("asdasdasd"), FullName: "Rina Kartika",
				NIK: "3171017008780010", Alamat: "Jl. Flamboyan No. 20, Jakarta Pusat",
				PhoneNumber: "6281100001019", CompanionName: "Hendra Kartika",
				CompanionPhone: "6281100001020", DateOfBirth: dob(1978, 8, 30),
				Sex: "female", DiseaseType: "diabetes_t2", Status: "active",
				EnrolledAt: daysAgo(35),
			},
			plainPass: "asdasdasd",
		},
	}

	for _, p := range patients {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&p.Patient).Error; err != nil {
			log.Fatal("seed patient", zap.String("id", p.Patient.ID), zap.Error(err))
		}
	}

	// Kumpulkan ID untuk kemudahan referensi
	p1ID := patients[0].Patient.ID
	p2ID := patients[1].Patient.ID
	p3ID := patients[2].Patient.ID
	p4ID := patients[3].Patient.ID
	p5ID := patients[4].Patient.ID
	p6ID := patients[5].Patient.ID
	p7ID := patients[6].Patient.ID
	p8ID := patients[7].Patient.ID
	p9ID := patients[8].Patient.ID
	p10ID := patients[9].Patient.ID

	// ── 4. CLINICAL BASELINES ─────────────────────────────────────────────────
	fmt.Println("▶ [4/8] Seeding patient_clinical_baselines...")

	clusterDM := ptr(0)
	clusterHT := ptr(1)
	clusterBoth := ptr(2)
	diagDM := ptr("high_glucose_group")
	diagHT := ptr("hypertension_group")
	diagBoth := ptr("mixed_cardiometabolic")
	groupHigh := ptr("high_risk")
	groupMod := ptr("moderate_risk")
	groupLow := ptr("low_risk")

	baselines := []entity.PatientClinicalBaseline{
		// Pasien 1 — Suharto: DM T2 berat, BMI obese
		{
			PatientID: p1ID, RecordedAt: daysAgo(60),
			AgeYears: 67, Sex: "male",
			BMI: 29.8, BMICategory: "overweight", WaistCircumferenceCm: 98, CentralObesity: true,
			SmokingStatus: "former_smoker", AlcoholUse: false, PhysicalActivity: "sedentary",
			FamilyHistoryDiabetes: true, FamilyHistoryCVD: true,
			SystolicBPMmhg: 145, DiastolicBPMmhg: 90, HypertensionStatus: "stage_1",
			FastingGlucoseMgdl: 210, HbA1cPct: 9.2, DiabetesStatus: "uncontrolled",
			TotalCholesterolMgdl: 230, HDLMgdl: 38, LDLMgdl: 155, TriglyceidesMgdl: 185,
			CVDRisk10YrPct: 98.0, CVDRiskCategory: "very_high",
			OnAntihypertensive: true, OnAntidiabetic: true, OnStatin: true,
			TargetRisk: "high", EGFR: 68, UACR: 45,
			ClusterID: clusterDM, DiagnosisCluster: diagDM, ClinicalGroup: groupHigh,
		},
		// Pasien 2 — Hartini: HT berat, wanita lansia
		{
			PatientID: p2ID, RecordedAt: daysAgo(45),
			AgeYears: 63, Sex: "female",
			BMI: 27.4, BMICategory: "overweight", WaistCircumferenceCm: 90, CentralObesity: true,
			SmokingStatus: "never", AlcoholUse: false, PhysicalActivity: "light",
			FamilyHistoryDiabetes: false, FamilyHistoryCVD: true,
			SystolicBPMmhg: 168, DiastolicBPMmhg: 102, HypertensionStatus: "stage_2",
			FastingGlucoseMgdl: 105, HbA1cPct: 5.8, DiabetesStatus: "normal",
			TotalCholesterolMgdl: 215, HDLMgdl: 48, LDLMgdl: 135, TriglyceidesMgdl: 160,
			CVDRisk10YrPct: 84.0, CVDRiskCategory: "very_high",
			OnAntihypertensive: true, OnAntidiabetic: false, OnStatin: false,
			TargetRisk: "high", EGFR: 75, UACR: 30,
			ClusterID: clusterHT, DiagnosisCluster: diagHT, ClinicalGroup: groupHigh,
		},
		// Pasien 3 — Agus: Both, sedang
		{
			PatientID: p3ID, RecordedAt: daysAgo(30),
			AgeYears: 60, Sex: "male",
			BMI: 26.1, BMICategory: "overweight", WaistCircumferenceCm: 93, CentralObesity: true,
			SmokingStatus: "current_smoker", AlcoholUse: false, PhysicalActivity: "light",
			FamilyHistoryDiabetes: true, FamilyHistoryCVD: false,
			SystolicBPMmhg: 152, DiastolicBPMmhg: 94, HypertensionStatus: "stage_1",
			FastingGlucoseMgdl: 162, HbA1cPct: 7.8, DiabetesStatus: "uncontrolled",
			TotalCholesterolMgdl: 205, HDLMgdl: 40, LDLMgdl: 130, TriglyceidesMgdl: 175,
			CVDRisk10YrPct: 51.0, CVDRiskCategory: "high",
			OnAntihypertensive: true, OnAntidiabetic: true, OnStatin: false,
			TargetRisk: "moderate", EGFR: 82, UACR: 20,
			ClusterID: clusterBoth, DiagnosisCluster: diagBoth, ClinicalGroup: groupMod,
		},
		// Pasien 4 — Yuliana: DM T2 terkontrol
		{
			PatientID: p4ID, RecordedAt: daysAgo(90),
			AgeYears: 50, Sex: "female",
			BMI: 23.5, BMICategory: "normal", WaistCircumferenceCm: 80, CentralObesity: false,
			SmokingStatus: "never", AlcoholUse: false, PhysicalActivity: "moderate",
			FamilyHistoryDiabetes: true, FamilyHistoryCVD: false,
			SystolicBPMmhg: 122, DiastolicBPMmhg: 78, HypertensionStatus: "normal",
			FastingGlucoseMgdl: 118, HbA1cPct: 6.8, DiabetesStatus: "controlled",
			TotalCholesterolMgdl: 185, HDLMgdl: 55, LDLMgdl: 105, TriglyceidesMgdl: 125,
			CVDRisk10YrPct: 21.0, CVDRiskCategory: "low",
			OnAntihypertensive: false, OnAntidiabetic: true, OnStatin: false,
			TargetRisk: "low", EGFR: 95, UACR: 8,
			ClusterID: clusterDM, DiagnosisCluster: diagDM, ClinicalGroup: groupLow,
		},
		// Pasien 5 — Bambang: HT aman, patuh
		{
			PatientID: p5ID, RecordedAt: daysAgo(120),
			AgeYears: 53, Sex: "male",
			BMI: 24.8, BMICategory: "normal", WaistCircumferenceCm: 88, CentralObesity: false,
			SmokingStatus: "never", AlcoholUse: false, PhysicalActivity: "moderate",
			FamilyHistoryDiabetes: false, FamilyHistoryCVD: false,
			SystolicBPMmhg: 130, DiastolicBPMmhg: 82, HypertensionStatus: "prehypertension",
			FastingGlucoseMgdl: 95, HbA1cPct: 5.4, DiabetesStatus: "normal",
			TotalCholesterolMgdl: 178, HDLMgdl: 52, LDLMgdl: 100, TriglyceidesMgdl: 130,
			CVDRisk10YrPct: 14.0, CVDRiskCategory: "moderate",
			OnAntihypertensive: true, OnAntidiabetic: false, OnStatin: false,
			TargetRisk: "low", EGFR: 90, UACR: 5,
			ClusterID: clusterHT, DiagnosisCluster: diagHT, ClinicalGroup: groupLow,
		},
		// Pasien 6 — Mirnawati: Both, baru enroll
		{
			PatientID: p6ID, RecordedAt: daysAgo(7),
			AgeYears: 45, Sex: "female",
			BMI: 28.2, BMICategory: "overweight", WaistCircumferenceCm: 92, CentralObesity: true,
			SmokingStatus: "never", AlcoholUse: false, PhysicalActivity: "sedentary",
			FamilyHistoryDiabetes: true, FamilyHistoryCVD: true,
			SystolicBPMmhg: 155, DiastolicBPMmhg: 96, HypertensionStatus: "stage_1",
			FastingGlucoseMgdl: 148, HbA1cPct: 7.2, DiabetesStatus: "uncontrolled",
			TotalCholesterolMgdl: 220, HDLMgdl: 44, LDLMgdl: 142, TriglyceidesMgdl: 170,
			CVDRisk10YrPct: 48.0, CVDRiskCategory: "moderate",
			OnAntihypertensive: true, OnAntidiabetic: true, OnStatin: false,
			TargetRisk: "moderate", EGFR: 88, UACR: 15,
			ClusterID: clusterBoth, DiagnosisCluster: diagBoth, ClinicalGroup: groupMod,
		},
		// Pasien 7 — Lukman: DM T2, BMI normal, risiko AMAN
		{
			PatientID: p7ID, RecordedAt: daysAgo(40),
			AgeYears: 56, Sex: "male",
			BMI: 23.8, BMICategory: "normal", WaistCircumferenceCm: 85, CentralObesity: false,
			SmokingStatus: "never", AlcoholUse: false, PhysicalActivity: "moderate",
			FamilyHistoryDiabetes: true, FamilyHistoryCVD: false,
			SystolicBPMmhg: 125, DiastolicBPMmhg: 80, HypertensionStatus: "normal",
			FastingGlucoseMgdl: 115, HbA1cPct: 6.2, DiabetesStatus: "controlled",
			TotalCholesterolMgdl: 190, HDLMgdl: 50, LDLMgdl: 110, TriglyceidesMgdl: 130,
			CVDRisk10YrPct: 12.0, CVDRiskCategory: "low",
			OnAntihypertensive: false, OnAntidiabetic: true, OnStatin: false,
			TargetRisk: "low", EGFR: 92, UACR: 8,
			ClusterID: clusterDM, DiagnosisCluster: diagDM, ClinicalGroup: groupLow,
		},
		// Pasien 8 — Siti Rahma: HT, BMI overweight, risiko WASWAS
		{
			PatientID: p8ID, RecordedAt: daysAgo(25),
			AgeYears: 52, Sex: "female",
			BMI: 26.5, BMICategory: "overweight", WaistCircumferenceCm: 88, CentralObesity: true,
			SmokingStatus: "never", AlcoholUse: false, PhysicalActivity: "light",
			FamilyHistoryDiabetes: false, FamilyHistoryCVD: true,
			SystolicBPMmhg: 154, DiastolicBPMmhg: 92, HypertensionStatus: "stage_1",
			FastingGlucoseMgdl: 98, HbA1cPct: 5.5, DiabetesStatus: "none",
			TotalCholesterolMgdl: 210, HDLMgdl: 46, LDLMgdl: 132, TriglyceidesMgdl: 155,
			CVDRisk10YrPct: 48.0, CVDRiskCategory: "moderate",
			OnAntihypertensive: true, OnAntidiabetic: false, OnStatin: false,
			TargetRisk: "moderate", EGFR: 80, UACR: 12,
			ClusterID: clusterHT, DiagnosisCluster: diagHT, ClinicalGroup: groupMod,
		},
		// Pasien 9 — Ahmad: Both, BMI overweight, risiko BAHAYA
		{
			PatientID: p9ID, RecordedAt: daysAgo(50),
			AgeYears: 66, Sex: "male",
			BMI: 28.5, BMICategory: "overweight", WaistCircumferenceCm: 96, CentralObesity: true,
			SmokingStatus: "former_smoker", AlcoholUse: false, PhysicalActivity: "sedentary",
			FamilyHistoryDiabetes: true, FamilyHistoryCVD: true,
			SystolicBPMmhg: 165, DiastolicBPMmhg: 98, HypertensionStatus: "stage_2",
			FastingGlucoseMgdl: 220, HbA1cPct: 9.4, DiabetesStatus: "uncontrolled",
			TotalCholesterolMgdl: 240, HDLMgdl: 36, LDLMgdl: 162, TriglyceidesMgdl: 195,
			CVDRisk10YrPct: 86.0, CVDRiskCategory: "very_high",
			OnAntihypertensive: true, OnAntidiabetic: true, OnStatin: true,
			TargetRisk: "high", EGFR: 62, UACR: 48,
			ClusterID: clusterBoth, DiagnosisCluster: diagBoth, ClinicalGroup: groupHigh,
		},
		// Pasien 10 — Rina: DM T2, BMI normal, risiko AMAN
		{
			PatientID: p10ID, RecordedAt: daysAgo(35),
			AgeYears: 47, Sex: "female",
			BMI: 22.4, BMICategory: "normal", WaistCircumferenceCm: 78, CentralObesity: false,
			SmokingStatus: "never", AlcoholUse: false, PhysicalActivity: "moderate",
			FamilyHistoryDiabetes: true, FamilyHistoryCVD: false,
			SystolicBPMmhg: 120, DiastolicBPMmhg: 75, HypertensionStatus: "normal",
			FastingGlucoseMgdl: 110, HbA1cPct: 6.4, DiabetesStatus: "controlled",
			TotalCholesterolMgdl: 180, HDLMgdl: 58, LDLMgdl: 102, TriglyceidesMgdl: 120,
			CVDRisk10YrPct: 18.0, CVDRiskCategory: "low",
			OnAntihypertensive: false, OnAntidiabetic: true, OnStatin: false,
			TargetRisk: "low", EGFR: 96, UACR: 6,
			ClusterID: clusterDM, DiagnosisCluster: diagDM, ClinicalGroup: groupLow,
		},
	}

	for i := range baselines {
		if err := db.Create(&baselines[i]).Error; err != nil {
			log.Fatal("seed baseline", zap.Int("i", i), zap.Error(err))
		}
	}

	// ── 5. HEALTH LOGS ────────────────────────────────────────────────────────
	fmt.Println("▶ [5/8] Seeding health_logs (14 hari terakhir)...")

	type pConfig struct {
		id            string
		glucoseBase   float64 // mg/dL rata-rata
		glucoseNoise  float64 // variasi ±
		systolicBase  float64
		systolicNoise float64
		diastBase     float64
		sleepBase     float64
		activityMin   float64
		stressBase    float64 // 1-5
	}

	patientConfigs := []pConfig{
		{p1ID, 215, 35, 148, 18, 92, 5.5, 15, 4.2},  // bahaya: glukosa tinggi
		{p2ID, 108, 12, 170, 20, 104, 6.0, 20, 3.8}, // bahaya: sistolik tinggi
		{p3ID, 165, 25, 154, 15, 96, 5.8, 25, 3.5},  // waswas
		{p4ID, 120, 10, 122, 8, 78, 7.2, 40, 2.1},   // aman
		{p5ID, 95, 8, 132, 10, 84, 7.5, 45, 1.8},    // aman
		{p6ID, 148, 20, 156, 18, 98, 6.2, 18, 3.8},  // waswas
		{p7ID, 115, 12, 125, 10, 80, 7.0, 35, 2.2},  // aman
		{p8ID, 98, 8, 154, 15, 92, 6.2, 22, 3.4},   // waswas
		{p9ID, 220, 30, 165, 16, 98, 5.2, 12, 4.5},  // bahaya
		{p10ID, 110, 10, 120, 8, 75, 7.3, 38, 2.0},  // aman
	}

	var healthLogs []entity.HealthLog
	for _, pc := range patientConfigs {
		for day := 14; day >= 0; day-- {
			// Glukosa pagi
			g := pc.glucoseBase + (float64(day%3)-1)*pc.glucoseNoise/2
			healthLogs = append(healthLogs, entity.HealthLog{
				PatientID: pc.id, LoggedBy: entity.LoggedByPatient,
				MetricType: "glucose", ValueNumeric: ptr(math.Round(g*10) / 10),
				MeasuredAt: daysAgoAt(day, 7, 0), Source: entity.LogSourceApp,
			})

			// Tekanan darah sore
			sys := pc.systolicBase + (float64(day%4)-1.5)*pc.systolicNoise/3
			dia := pc.diastBase + (float64(day%3)-1)*4
			bpJSON := fmt.Sprintf(`{"systolic":%d,"diastolic":%d}`,
				int(math.Round(sys)), int(math.Round(dia)))
			healthLogs = append(healthLogs, entity.HealthLog{
				PatientID: pc.id, LoggedBy: entity.LoggedByPatient,
				MetricType: "bp", ValueJSONB: ptr(bpJSON),
				MeasuredAt: daysAgoAt(day, 16, 30), Source: entity.LogSourceApp,
			})

			// Aktivitas malam
			actMin := pc.activityMin + float64(day%3)*5
			healthLogs = append(healthLogs, entity.HealthLog{
				PatientID: pc.id, LoggedBy: entity.LoggedByPatient,
				MetricType: "activity", ValueNumeric: ptr(actMin),
				MeasuredAt: daysAgoAt(day, 19, 0), Source: entity.LogSourceApp,
			})

			// Tidur malam
			healthLogs = append(healthLogs, entity.HealthLog{
				PatientID: pc.id, LoggedBy: entity.LoggedByPatient,
				MetricType: "sleep", ValueNumeric: ptr(pc.sleepBase + float64(day%2)*0.5),
				MeasuredAt: daysAgoAt(day, 22, 0), Source: entity.LogSourceApp,
			})

			// Stres (setiap 2 hari)
			if day%2 == 0 {
				healthLogs = append(healthLogs, entity.HealthLog{
					PatientID: pc.id, LoggedBy: entity.LoggedByPatient,
					MetricType: "stress", ValueNumeric: ptr(pc.stressBase),
					MeasuredAt: daysAgoAt(day, 21, 0), Source: entity.LogSourceApp,
				})
			}

			// Kepatuhan obat (setiap hari, hanya yang pakai obat)
			if pc.id != p4ID { // semua kecuali yuliana (tidak antihipertensif)
				adherence := 1.0
				if day == 5 || day == 11 { // skip 2 hari sebagai simulasi lupa
					adherence = 0.0
				}
				healthLogs = append(healthLogs, entity.HealthLog{
					PatientID: pc.id, LoggedBy: entity.LoggedByPatient,
					MetricType: "med_adherence", ValueNumeric: ptr(adherence),
					MeasuredAt: daysAgoAt(day, 8, 0), Source: entity.LogSourceApp,
				})
			}
		}
	}

	if err := db.CreateInBatches(&healthLogs, 100).Error; err != nil {
		log.Fatal("seed health_logs", zap.Error(err))
	}
	fmt.Printf("   → %d log dibuat\n", len(healthLogs))

	// ── 6. LAB RESULTS ────────────────────────────────────────────────────────
	// entity.LabResult belum ada — insert via raw SQL.
	fmt.Println("▶ [6/8] Seeding lab_results...")

	type labEntry struct {
		patientID string
		labType   string
		value     float64
		unit      string
		daysBack  int
	}

	labEntries := []labEntry{
		// Suharto — HbA1c buruk, LDL tinggi
		{p1ID, "hba1c", 9.2, "%", 30},
		{p1ID, "ldl", 155, "mg/dL", 30},
		{p1ID, "egfr", 68, "mL/min/1.73m2", 30},
		{p1ID, "hba1c", 8.8, "%", 90}, // sebelumnya lebih buruk
		// Hartini — eGFR turun
		{p2ID, "hba1c", 5.8, "%", 20},
		{p2ID, "ldl", 135, "mg/dL", 20},
		{p2ID, "egfr", 75, "mL/min/1.73m2", 20},
		// Agus
		{p3ID, "hba1c", 7.8, "%", 15},
		{p3ID, "ldl", 130, "mg/dL", 15},
		// Yuliana — terkontrol
		{p4ID, "hba1c", 6.8, "%", 60},
		{p4ID, "ldl", 105, "mg/dL", 60},
		{p4ID, "egfr", 95, "mL/min/1.73m2", 60},
		{p4ID, "hba1c", 6.5, "%", 120}, // membaik
		// Bambang
		{p5ID, "hba1c", 5.4, "%", 90},
		{p5ID, "ldl", 100, "mg/dL", 90},
		// Mirnawati — baru
		{p6ID, "hba1c", 7.2, "%", 5},
		{p6ID, "ldl", 142, "mg/dL", 5},
		// Lukman
		{p7ID, "hba1c", 6.2, "%", 30},
		{p7ID, "ldl", 110, "mg/dL", 30},
		// Siti Rahma
		{p8ID, "hba1c", 5.5, "%", 20},
		{p8ID, "ldl", 132, "mg/dL", 20},
		// Ahmad
		{p9ID, "hba1c", 9.4, "%", 30},
		{p9ID, "ldl", 162, "mg/dL", 30},
		{p9ID, "egfr", 62, "mL/min/1.73m2", 30},
		// Rina
		{p10ID, "hba1c", 6.4, "%", 25},
		{p10ID, "ldl", 102, "mg/dL", 25},
	}

	const insertLab = `
		INSERT INTO lab_results (patient_id, recorded_by, lab_type, value_numeric, unit, result_date, source)
		VALUES (?, ?, ?, ?, ?, ?, 'faskes')`

	patientNakes := make(map[string]string)
	for _, p := range patients {
		patientNakes[p.Patient.ID] = p.Patient.AssignedNakesID
	}

	for _, le := range labEntries {
		recordedBy := patientNakes[le.patientID]
		if recordedBy == "" {
			recordedBy = dokter1ID
		}
		if err := db.Exec(insertLab,
			le.patientID, recordedBy, le.labType, le.value, le.unit,
			daysAgo(le.daysBack),
		).Error; err != nil {
			log.Fatal("seed lab_result", zap.String("type", le.labType), zap.Error(err))
		}
	}

	// ── 7. DAILY FEATURES + RISK SCORES ──────────────────────────────────────
	fmt.Println("▶ [7/8] Seeding daily_features + risk_scores...")

	type featureConfig struct {
		patientID      string
		glucoseMean    float64
		glucoseCV      float64 // coefficient of variation
		systolicMean   float64
		sodiumRoll7    float64 // mg
		sleepRoll7     float64
		activityPct    float64 // 0-1 rasio hari aktif
		stressRoll7    float64
		carbsRoll7     float64 // gram
		expectedStatus string
		expectedScore  int
		rule           *string
	}

	featureConfigs := []featureConfig{
		{p1ID, 215, 0.18, 148, 2800, 5.5, 0.2, 4.2, 320, "bahaya", 8, ptr("glucose_critical_high")},
		{p2ID, 108, 0.08, 172, 2500, 6.0, 0.3, 3.8, 280, "bahaya", 22, ptr("systolic_stage2")},
		{p3ID, 165, 0.12, 154, 2600, 5.8, 0.35, 3.5, 295, "waswas", 55, nil},
		{p4ID, 120, 0.06, 122, 1900, 7.2, 0.65, 2.1, 215, "aman", 85, nil},
		{p5ID, 95, 0.04, 132, 1800, 7.5, 0.70, 1.8, 200, "aman", 92, nil},
		{p6ID, 148, 0.14, 156, 2400, 6.2, 0.28, 3.8, 275, "waswas", 58, nil},
		{p7ID, 115, 0.06, 125, 2000, 7.0, 0.60, 2.2, 220, "aman", 88, nil},
		{p8ID, 98, 0.05, 154, 2450, 6.2, 0.32, 3.4, 260, "waswas", 58, nil},
		{p9ID, 220, 0.16, 165, 2900, 5.2, 0.18, 4.5, 335, "bahaya", 20, ptr("glucose_critical_high")},
		{p10ID, 110, 0.05, 120, 1950, 7.3, 0.62, 2.0, 210, "aman", 82, nil},
	}

	type dfRsPair struct {
		df entity.DailyFeature
		rs entity.RiskScore
	}

	// Seed 7 hari terakhir per pasien
	for dayBack := 7; dayBack >= 1; dayBack-- {
		for _, fc := range featureConfigs {
			// Tambahkan sedikit variasi per hari
			variation := 1.0 + (float64(dayBack%3)-1)*0.05

			df := entity.DailyFeature{
				PatientID:        fc.patientID,
				FeatureDate:      daysAgo(dayBack),
				GlucoseMeanRoll7: math.Round(fc.glucoseMean*variation*10) / 10,
				GlucoseCVRoll7:   math.Round(fc.glucoseCV*variation*1000) / 1000,
				SystolicRoll7:    math.Round(fc.systolicMean*variation*10) / 10,
				SodiumRoll7:      math.Round(fc.sodiumRoll7*variation),
				SleepRoll7:       math.Round(fc.sleepRoll7*10) / 10,
				ActivityPctRoll7: math.Round(fc.activityPct*100) / 100,
				StressRoll7:      math.Round(fc.stressRoll7*10) / 10,
				CarbsRoll7:       math.Round(fc.carbsRoll7 * variation),
			}
			if err := db.Create(&df).Error; err != nil {
				log.Fatal("seed daily_feature", zap.Error(err))
			}

			scoreDelta := (dayBack - 4) * 2 // tren meningkat mendekati hari ini
			score := fc.expectedScore + scoreDelta
			if score < 0 {
				score = 0
			}
			if score > 100 {
				score = 100
			}

			mode := "rule_based"
			if fc.rule == nil {
				mode = "cohort"
			}

			factors := topFactors([]string{
				"glucose_mean_roll7",
				"systolic_roll7",
				"activity_pct_roll7",
			})

			rs := entity.RiskScore{
				PatientID:      fc.patientID,
				DailyFeatureID: df.ID,
				Score:          score,
				Status:         fc.expectedStatus,
				ScoringMode:    mode,
				TopFactors:     factors,
				TriggeredRule:  fc.rule,
				ScoredAt:       daysAgoAt(dayBack, 3, 0),
			}
			if err := db.Create(&rs).Error; err != nil {
				log.Fatal("seed risk_score", zap.Error(err))
			}
		}
	}

	// ── 8. ESCALATIONS ────────────────────────────────────────────────────────
	fmt.Println("▶ [8/8] Seeding escalations...")

	// Query risk scores terbaru untuk pasien bahaya & waswas
	type rsRow struct {
		ID        string
		PatientID string
		Status    string
	}
	var latestScores []rsRow
	db.Raw(`
		SELECT DISTINCT ON (patient_id) id, patient_id, status
		FROM risk_scores
		WHERE patient_id IN (?,?,?,?,?,?)
		ORDER BY patient_id, scored_at DESC
	`, p1ID, p2ID, p3ID, p6ID, p8ID, p9ID).Scan(&latestScores)

	sentAt5DaysAgo := daysAgoAt(5, 8, 0)
	viewedAt := daysAgoAt(4, 10, 0)
	actedAt := daysAgoAt(4, 11, 0)
	feedbackAcc := entity.EscalationFeedbackAccurate

	escalations := []entity.Escalation{
		// P1 Suharto — BAHAYA, eskalasi 5 hari lalu sudah acted + feedback accurate
		{
			PatientID:       p1ID,
			RiskScoreID:     latestScores[0].ID,
			FaskesID:        faskes.ID,
			AssignedNakesID: dokter1ID,
			Tier:            entity.EscalationTierAcuteToday,
			Channel:         "whatsapp",
			Status:          entity.EscalationStatusActed,
			SentAt:          sentAt5DaysAgo,
			ViewedAt:        &viewedAt,
			ActedAt:         &actedAt,
			Feedback:        &feedbackAcc,
			FeedbackBy:      &dokter1ID,
			FeedbackAt:      &actedAt,
		},
		// P1 Suharto — BAHAYA, eskalasi baru (hari ini, belum dilihat)
		{
			PatientID:       p1ID,
			RiskScoreID:     latestScores[0].ID,
			FaskesID:        faskes.ID,
			AssignedNakesID: dokter1ID,
			Tier:            entity.EscalationTierAcuteToday,
			Channel:         "whatsapp",
			Status:          entity.EscalationStatusSent,
			SentAt:          daysAgoAt(0, 6, 0),
		},
		// P2 Hartini — BAHAYA, sudah viewed belum acted
		{
			PatientID:       p2ID,
			RiskScoreID:     latestScores[1].ID,
			FaskesID:        faskes.ID,
			AssignedNakesID: dokter1ID,
			Tier:            entity.EscalationTierAcuteToday,
			Channel:         "whatsapp",
			Status:          entity.EscalationStatusViewed,
			SentAt:          daysAgoAt(1, 7, 0),
			ViewedAt:        ptr(daysAgoAt(1, 9, 0)),
		},
		// P3 Agus — WASWAS, tren minggu ini
		{
			PatientID:       p3ID,
			RiskScoreID:     latestScores[2].ID,
			FaskesID:        faskes.ID,
			AssignedNakesID: dokter1ID,
			Tier:            entity.EscalationTierTrendThisWeek,
			Channel:         "whatsapp",
			Status:          entity.EscalationStatusSent,
			SentAt:          daysAgoAt(2, 8, 0),
		},
		// P6 Mirnawati — WASWAS, baru enroll
		{
			PatientID:       p6ID,
			RiskScoreID:     latestScores[3].ID,
			FaskesID:        faskes.ID,
			AssignedNakesID: dokter2ID,
			Tier:            entity.EscalationTierTrendThisWeek,
			Channel:         "whatsapp",
			Status:          entity.EscalationStatusSent,
			SentAt:          daysAgoAt(1, 8, 0),
		},
		// P8 Siti Rahma — WASWAS, tren minggu ini
		{
			PatientID:       p8ID,
			RiskScoreID:     latestScores[4].ID,
			FaskesID:        faskes.ID,
			AssignedNakesID: dokter3ID,
			Tier:            entity.EscalationTierTrendThisWeek,
			Channel:         "whatsapp",
			Status:          entity.EscalationStatusSent,
			SentAt:          daysAgoAt(2, 9, 0),
		},
		// P9 Ahmad — BAHAYA, eskalasi baru (hari ini, belum dilihat)
		{
			PatientID:       p9ID,
			RiskScoreID:     latestScores[5].ID,
			FaskesID:        faskes.ID,
			AssignedNakesID: dokter3ID,
			Tier:            entity.EscalationTierAcuteToday,
			Channel:         "whatsapp",
			Status:          entity.EscalationStatusSent,
			SentAt:          daysAgoAt(0, 7, 0),
		},
	}

	for i := range escalations {
		if err := db.Create(&escalations[i]).Error; err != nil {
			log.Fatal("seed escalation", zap.Int("i", i), zap.Error(err))
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Fatal("failed to commit transaction", zap.Error(err))
	}

	// Use original mainDB for counting summary
	db = mainDB

	// ── SUMMARY ───────────────────────────────────────────────────────────────
	type count struct {
		Tabel string
		Rows  int64
	}
	tables := []string{
		"faskes", "nakes", "patients",
		"patient_clinical_baselines", "health_logs", "lab_results",
		"daily_features", "risk_scores", "escalations",
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║                   Seed Selesai ✓                        ║")
	fmt.Println("╠══════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-35s %-10s       ║\n", "Tabel", "Rows")
	fmt.Println("║  ─────────────────────────────────────────────          ║")
	for _, t := range tables {
		var n int64
		db.Table(t).Count(&n)
		fmt.Printf("║  %-35s %-10d       ║\n", t, n)
	}
	fmt.Println("╠══════════════════════════════════════════════════════════╣")
	fmt.Println("║                                                          ║")
	fmt.Println("║  Login nakes  : dr.rani / dr.budi / dr.siti              ║")
	fmt.Println("║  Password     : asdasdasd                                ║")
	fmt.Println("║  Login pasien : pasien.suharto ... pasien.rina           ║")
	fmt.Println("║  Password     : asdasdasd                                ║")
	fmt.Println("║                                                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	log.Info("seed-demo selesai")
}
