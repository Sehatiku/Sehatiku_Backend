package main

import (
	"context"
	"fmt"
	"os"

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

	container := sqlstore.NewWithDB(sqlDB, "postgres", waLog.Noop)
	if err := container.Upgrade(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to upgrade whatsmeow schema: %v\n", err)
		os.Exit(1)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get device store: %v\n", err)
		os.Exit(1)
	}

	client := whatsmeow.NewClient(deviceStore, waLog.Noop)

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
			fmt.Println("Pairing berhasil! Jalankan server dengan: go run ./cmd/web")
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
