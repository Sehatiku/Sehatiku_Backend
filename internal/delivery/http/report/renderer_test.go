package report

import (
	"strings"
	"testing"
	"time"

	"sehatiku-backend/internal/model"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func TestRenderBrief_Populated(t *testing.T) {
	scoredAt := time.Date(2026, 7, 6, 1, 0, 0, 0, time.UTC)
	age := 58
	data := &model.BriefReportData{
		Patient: model.BriefPatientHeader{
			FullName: "Budi Santoso", AgeYears: &age, Sex: "male", DiseaseType: "diabetes_t2",
		},
		Brief: &model.PreVisitBriefResponse{
			Period:      model.SummaryPeriod{Start: "2026-06-07", End: "2026-07-06"},
			Coverage:    model.SummaryCoverage{LoggedDays: 21, WindowDays: 30, StreakDays: 4},
			HistoryDays: 45,
			Trajectory: model.BriefTrajectory{
				Daily: []model.RecordHistoryItem{
					{Date: "2026-06-07", BloodSugar: ptrF(215), Systolic: ptrI(168), Diastolic: ptrI(99), Weight: ptrF(92)},
					{Date: "2026-06-08", BloodSugar: ptrF(200), Systolic: ptrI(160), Diastolic: ptrI(95), Weight: ptrF(91)},
					{Date: "2026-06-09", BloodSugar: ptrF(190), Systolic: ptrI(150), Diastolic: ptrI(90), Weight: ptrF(91)},
				},
				GlucoseSlopePerWeek:  ptrF(4.2),
				SystolicSlopePerWeek: ptrF(-1.1),
			},
			Risk: &model.BriefRisk{
				Score: 40, Status: "waswas", ScoringMode: "cohort", ScoredAt: scoredAt,
				TopFactors: []string{"Gula darah rata-rata Anda cukup tinggi (212.5 mg/dL)."},
			},
			MedAdherence: model.BriefMedAdherence{
				AdherenceRatePct: 73.3, TakenDays: 11, MissedDays: 4,
				MissedDates:    []string{"2026-06-14", "2026-06-21"},
				MissedWeekdays: map[string]int{"Sabtu": 2, "Minggu": 2},
			},
			Narrative:          "Pasien menunjukkan tren gula darah menurun namun tekanan darah masih tinggi.",
			AnamnesisQuestions: []string{"Apakah Bapak sering lupa minum obat di akhir pekan?"},
			GeneratedAt:        scoredAt,
		},
	}

	html, err := RenderBrief(data)
	if err != nil {
		t.Fatalf("RenderBrief error: %v", err)
	}

	for _, want := range []string{
		"Budi Santoso",              // patient name
		"Waswas",                    // risk status label
		"<svg",                      // charts rendered
		"tren gula darah menurun",   // narrative text
		"lupa minum obat di akhir",  // anamnesis question
		"Diabetes Tipe 2",           // disease label
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

// Jalur data jarang: brief selalu tersedia walau sparse — render tidak boleh panic.
func TestRenderBrief_Sparse(t *testing.T) {
	data := &model.BriefReportData{
		Patient: model.BriefPatientHeader{FullName: "Kosong", Sex: "female"},
		Brief: &model.PreVisitBriefResponse{
			Period:               model.SummaryPeriod{Start: "2026-06-07", End: "2026-07-06"},
			Trajectory:           model.BriefTrajectory{Daily: []model.RecordHistoryItem{}},
			Aggregates:           nil,
			Risk:                 nil,
			MedAdherence:         model.BriefMedAdherence{MissedDates: []string{}, MissedWeekdays: map[string]int{}},
			EscalationsThisMonth: []model.BriefEscalation{},
			Narrative:            "Ringkasan tidak tersedia.",
			GeneratedAt:          time.Now(),
		},
	}

	html, err := RenderBrief(data)
	if err != nil {
		t.Fatalf("RenderBrief sparse error: %v", err)
	}
	for _, want := range []string{"Belum pernah diskor", "data belum cukup", "Tidak ada eskalasi"} {
		if !strings.Contains(html, want) {
			t.Errorf("sparse HTML missing %q", want)
		}
	}
}
