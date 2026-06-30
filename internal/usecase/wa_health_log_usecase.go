package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/helper"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// waPatientRepo adalah interface minimal yang dibutuhkan WAHealthLogUseCase untuk
// mencari pasien berdasarkan nomor WA pengirim. Di-interface-kan agar usecase bisa
// diuji dengan mock tanpa menyentuh DB (be_implementation §7).
type waPatientRepo interface {
	FindByPhone(db *gorm.DB, phone string) (*entity.Patient, error)
	FindByCompanionPhone(db *gorm.DB, phone string) (*entity.Patient, error)
}

// waHealthLogRepo adalah interface minimal yang dibutuhkan — hanya Create.
// Sama dengan healthLogRepository yang dipakai HealthLogUseCase, tapi tidak
// meng-inherit interface itu untuk menghindari dependensi silang antar file.
type waHealthLogRepo interface {
	Create(db *gorm.DB, log *entity.HealthLog) error
}

// waReplySender adalah interface outbound untuk balasan WA ke pasien/pendamping.
// Di-interface-kan agar usecase bisa diuji dengan mock (pemanggilan WA mahal/eksternal).
type waReplySender interface {
	SendHealthLogConfirmation(ctx context.Context, toPhone, patientName, metricLabel, valueStr string) error
	SendHealthLogParseError(ctx context.Context, toPhone string) error
	SendHealthLogNotRegistered(ctx context.Context, toPhone string) error
}

// WAHealthLogUseCase menangani pesan WA inbound dari pasien/pendamping yang bermaksud
// mencatat data harian (gula darah, tekanan darah, obat, makanan, dll). Alurnya:
//
//  1. Lookup pasien berdasarkan nomor pengirim (phone atau companion_phone).
//  2. Parse teks pesan → metric_type + nilai.
//  3. Validasi range nilai (konsisten dengan HealthLogUseCase).
//  4. Insert ke health_logs (insert-only, source=whatsapp).
//  5. Kirim balasan konfirmasi ke pengirim.
//
// Setiap langkah yang gagal mengirim balasan WA yang tepat dan return nil —
// kegagalan parsir atau lookup bukan error teknis, tidak perlu di-propagate ke caller.
type WAHealthLogUseCase struct {
	DB          *gorm.DB
	PatientRepo waPatientRepo
	LogRepo     waHealthLogRepo
	Extractor   foodExtractor // opsional — NER makanan; pakai interface dari health_log_usecase.go
	WhatsApp    waReplySender
	Log         *zap.Logger
}

// HandleInbound dipanggil untuk setiap pesan teks WA masuk dari pengirim dengan nomor
// `senderPhone` (format internasional tanpa '+', sudah dinormalisasi oleh caller).
// Mengembalikan nil selalu — error internal di-log, error bisnis (tidak terdaftar, format
// salah) dikirim sebagai balasan WA sehingga caller tidak perlu menanganinya.
func (u *WAHealthLogUseCase) HandleInbound(ctx context.Context, senderPhone, messageText string) error {
	patient, loggedBy, err := u.lookupPatient(ctx, senderPhone)
	if err != nil {
		// Error teknis DB — log dan return error agar caller bisa log juga
		return fmt.Errorf("wa health log lookup for %s: %w", helper.MaskPhone(senderPhone), err)
	}
	if patient == nil {
		// Nomor tidak terdaftar — balas WA, bukan error
		u.replyNotRegistered(ctx, senderPhone)
		return nil
	}

	parsed := helper.ParseWAMessage(messageText)
	if parsed.Err != nil {
		// Pesan tidak dikenali — kirim panduan format
		u.Log.Debug("wa message not parsed",
			zap.String("patient_id", patient.ID),
			zap.String("sender", helper.MaskPhone(senderPhone)),
			zap.Error(parsed.Err),
		)
		u.replyParseError(ctx, senderPhone)
		return nil
	}

	log, err := u.buildLog(patient.ID, loggedBy, parsed, time.Now())
	if err != nil {
		// Validasi range gagal — kirim panduan format dengan detail error
		u.Log.Debug("wa health log value invalid",
			zap.String("patient_id", patient.ID),
			zap.String("metric_type", parsed.MetricType),
			zap.Error(err),
		)
		u.replyParseError(ctx, senderPhone)
		return nil
	}

	// Enrichment makanan via NER — fire-and-forget degradasi anggun (ML opsional)
	if parsed.MetricType == "food" && u.Extractor != nil && log.ValueText != nil {
		enrichFoodJSONB(ctx, u.Extractor, log, *log.ValueText, u.Log)
	}

	if err := u.LogRepo.Create(u.DB, log); err != nil {
		return fmt.Errorf("inserting wa health log for patient %s: %w", patient.ID, err)
	}

	u.Log.Info("wa health log created",
		zap.String("patient_id", patient.ID),
		zap.String("metric_type", log.MetricType),
		zap.String("logged_by", loggedBy),
	)

	label, valueStr := metricDisplay(parsed)
	u.replyConfirmation(ctx, senderPhone, patient.FullName, label, valueStr)
	return nil
}

// lookupPatient mencari pasien berdasarkan nomor WA pengirim. Mencoba phone_number dulu
// (pasien sendiri), lalu companion_phone (pendamping). Mengembalikan (nil, "", nil) bila
// tidak ditemukan — bukan error teknis.
func (u *WAHealthLogUseCase) lookupPatient(ctx context.Context, phone string) (*entity.Patient, string, error) {
	patient, err := u.PatientRepo.FindByPhone(u.DB, phone)
	if err == nil {
		return patient, entity.LoggedByPatient, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", fmt.Errorf("finding patient by phone: %w", err)
	}

	// Tidak ditemukan sebagai pasien — coba sebagai pendamping
	patient, err = u.PatientRepo.FindByCompanionPhone(u.DB, phone)
	if err == nil {
		return patient, entity.LoggedByCompanion, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", fmt.Errorf("finding patient by companion phone: %w", err)
	}

	// Tidak ditemukan sama sekali
	return nil, "", nil
}

// buildLog memetakan ParsedMetric ke entity.HealthLog, sekaligus memvalidasi range nilai
// (konsisten dengan applyMetricValue di health_log_usecase.go).
func (u *WAHealthLogUseCase) buildLog(patientID, loggedBy string, parsed helper.ParsedMetric, measuredAt time.Time) (*entity.HealthLog, error) {
	log := &entity.HealthLog{
		PatientID:  patientID,
		LoggedBy:   loggedBy,
		MetricType: parsed.MetricType,
		Source:     entity.LogSourceWhatsApp,
		MeasuredAt: measuredAt,
	}

	switch parsed.MetricType {
	case "glucose":
		v := *parsed.ValueNumeric
		if v < 20 || v > 600 {
			return nil, fmt.Errorf("gula darah %g di luar range valid (20–600 mg/dL)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "bp":
		sys, dia := *parsed.BPSystolic, *parsed.BPDiastolic
		// Validasi sudah dilakukan di parser, tapi kita validasi ulang di sini
		// untuk pertahanan berlapis (defense in depth).
		if sys < 40 || sys > 300 || dia < 20 || dia > 200 || sys <= dia {
			return nil, fmt.Errorf("tekanan darah tidak valid (sistolik %d, diastolik %d)", sys, dia)
		}
		jsonb := fmt.Sprintf(`{"systolic":%d,"diastolic":%d}`, sys, dia)
		log.ValueJSONB = &jsonb

	case "med_adherence":
		v := *parsed.ValueNumeric
		if v < 0 || v > 100 {
			return nil, fmt.Errorf("kepatuhan obat %g di luar range valid (0–100%%)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "activity":
		v := *parsed.ValueNumeric
		if v < 0 || v > 1440 {
			return nil, fmt.Errorf("olahraga %g menit di luar range valid (0–1440)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "sleep":
		v := *parsed.ValueNumeric
		if v < 0 || v > 24 {
			return nil, fmt.Errorf("tidur %g jam di luar range valid (0–24)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "stress":
		v := *parsed.ValueNumeric
		if v < 1 || v > 10 {
			return nil, fmt.Errorf("level stres %g di luar range valid (1–10)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "weight":
		v := *parsed.ValueNumeric
		if v < 1 || v > 500 {
			return nil, fmt.Errorf("berat badan %g kg di luar range valid (1–500)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "smoking":
		v := *parsed.ValueNumeric
		if v < 0 || v > 200 {
			return nil, fmt.Errorf("jumlah rokok %g di luar range valid (0–200)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "alcohol":
		v := *parsed.ValueNumeric
		if v < 0 || v > 100 {
			return nil, fmt.Errorf("alkohol %g di luar range valid (0–100)", v)
		}
		log.ValueNumeric = parsed.ValueNumeric

	case "food":
		text := *parsed.ValueText
		if len(text) > foodTextMaxLen {
			text = text[:foodTextMaxLen]
		}
		log.ValueText = &text

	default:
		return nil, fmt.Errorf("metric_type tidak dikenal: %s", parsed.MetricType)
	}

	return log, nil
}

// metricDisplay mengembalikan label bahasa Indonesia dan string nilai untuk pesan
// konfirmasi WA. Harus mudah dipahami lansia.
func metricDisplay(p helper.ParsedMetric) (label, value string) {
	switch p.MetricType {
	case "glucose":
		return "Gula darah", fmt.Sprintf("%.0f mg/dL", *p.ValueNumeric)
	case "bp":
		return "Tekanan darah", fmt.Sprintf("%d/%d mmHg", *p.BPSystolic, *p.BPDiastolic)
	case "med_adherence":
		if *p.ValueNumeric >= 100 {
			return "Kepatuhan obat", "sudah minum obat ✅"
		}
		return "Kepatuhan obat", "belum minum obat ⚠️"
	case "activity":
		return "Olahraga", fmt.Sprintf("%.0f menit", *p.ValueNumeric)
	case "sleep":
		return "Tidur", fmt.Sprintf("%.1f jam", *p.ValueNumeric)
	case "stress":
		return "Level stres", fmt.Sprintf("%.0f/10", *p.ValueNumeric)
	case "weight":
		return "Berat badan", fmt.Sprintf("%.1f kg", *p.ValueNumeric)
	case "smoking":
		if *p.ValueNumeric == 0 {
			return "Rokok", "tidak merokok ✅"
		}
		return "Rokok", fmt.Sprintf("%.0f batang", *p.ValueNumeric)
	case "alcohol":
		if *p.ValueNumeric == 0 {
			return "Alkohol", "tidak minum alkohol ✅"
		}
		return "Alkohol", fmt.Sprintf("%.0f unit", *p.ValueNumeric)
	case "food":
		text := *p.ValueText
		if len(text) > 50 {
			text = text[:50] + "..."
		}
		return "Makanan", text
	default:
		return p.MetricType, ""
	}
}

// ─── reply helpers ────────────────────────────────────────────────────────────
// Semua reply adalah fire-and-forget — error hanya di-log, tidak dipropagasi ke caller.

func (u *WAHealthLogUseCase) replyConfirmation(ctx context.Context, toPhone, patientName, label, value string) {
	if u.WhatsApp == nil {
		return
	}
	if err := u.WhatsApp.SendHealthLogConfirmation(ctx, toPhone, patientName, label, value); err != nil {
		u.Log.Warn("gagal kirim konfirmasi health log WA",
			zap.String("phone", helper.MaskPhone(toPhone)), zap.Error(err))
	}
}

func (u *WAHealthLogUseCase) replyParseError(ctx context.Context, toPhone string) {
	if u.WhatsApp == nil {
		return
	}
	if err := u.WhatsApp.SendHealthLogParseError(ctx, toPhone); err != nil {
		u.Log.Warn("gagal kirim panduan format WA",
			zap.String("phone", helper.MaskPhone(toPhone)), zap.Error(err))
	}
}

func (u *WAHealthLogUseCase) replyNotRegistered(ctx context.Context, toPhone string) {
	if u.WhatsApp == nil {
		return
	}
	if err := u.WhatsApp.SendHealthLogNotRegistered(ctx, toPhone); err != nil {
		u.Log.Warn("gagal kirim notif tidak terdaftar WA",
			zap.String("phone", helper.MaskPhone(toPhone)), zap.Error(err))
	}
}
