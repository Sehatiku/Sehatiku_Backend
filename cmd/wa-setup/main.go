package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sehatiku-backend/internal/config"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
	cfg := config.NewViper()
	log := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, log)

	ctx := context.Background()

	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get sql.DB: %v\n", err)
		os.Exit(1)
	}

	// ponytail: waLog.Noop membungkam log reconnect internal whatsmeow (mis. penanganan
	// stream-error 515 yang wajib terjadi setelah pair-device-sign) — tanpa ini, kegagalan
	// reconnect pasca-pairing tidak terlihat sama sekali. Pakai waLog.Stdout bawaan whatsmeow.
	waLogger := waLog.Stdout("wa-setup", "DEBUG", true)
	container := sqlstore.NewWithDB(sqlDB, "postgres", waLogger)
	if err := container.Upgrade(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to upgrade whatsmeow schema: %v\n", err)
		os.Exit(1)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get device store: %v\n", err)
		os.Exit(1)
	}

	client := whatsmeow.NewClient(deviceStore, waLogger)

	if client.Store.ID != nil {
		fmt.Println("Akun WhatsApp sudah terpasang.")
		fmt.Println("Untuk pair ulang, hapus baris di tabel whatsmeow_device lalu jalankan ulang perintah ini.")
		return
	}

	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get QR channel: %v\n", err)
		os.Exit(1)
	}

	if err := client.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Scan QR code di bawah ini dengan WhatsApp kamu:")
	fmt.Println()

	for evt := range qrChan {
		switch evt.Event {
		case whatsmeow.QRChannelEventCode:
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			fmt.Println()
			fmt.Println("Scan QR di atas. Menunggu konfirmasi pairing...")
		case "success":
			// ponytail: WhatsApp mewajibkan reconnect kedua (dipicu stream-error 515,
			// ditangani otomatis oleh whatsmeow) sebelum HP menandai companion device
			// online. Tanpa menunggu di sini, proses ini keluar duluan sebelum reconnect
			// sempat jalan — kredensial tersimpan tapi HP loading sampai timeout.
			fmt.Println("Pairing diterima, menunggu sesi tersambung ke WhatsApp...")
			deadline := time.Now().Add(30 * time.Second)
			for time.Now().Before(deadline) && !client.IsLoggedIn() {
				time.Sleep(500 * time.Millisecond)
			}
			if client.IsLoggedIn() {
				fmt.Println("Pairing berhasil dan tersambung! Jalankan server dengan: go run ./cmd/web")
			} else {
				fmt.Println("Kredensial tersimpan tapi sesi belum tersambung dalam 30 detik.")
				fmt.Println("Cek log whatsmeow di atas untuk detail error, lalu coba 'go run ./cmd/web'.")
			}
		case "timeout":
			fmt.Println("QR code expired. Jalankan ulang perintah ini.")
			os.Exit(1)
		default:
			if evt.Error != nil {
				fmt.Fprintf(os.Stderr, "pairing error: %v\n", evt.Error)
				os.Exit(1)
			}
		}
	}
}
