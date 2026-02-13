package handler

import (
	"context"
	"fmt"
	"slices"

	tbmodels "github.com/semanggilab/webcore-go/modules/tb/models"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/config"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/models"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/repository"
	ppubsub "github.com/webcore-go/lib-pubsub"
	"github.com/webcore-go/webcore/infra/logger"
	"github.com/webcore-go/webcore/port"
)

type CkgReceiver struct {
	Configurations *config.ModuleConfig
	CkgRepo        *repository.CKGTBRepository
	PubSubRepo     *repository.PubSubRepository
}

func NewCkgReceiver(ctx context.Context, config *config.ModuleConfig, ckgRepo *repository.CKGTBRepository, pubsubRepo *repository.PubSubRepository) *CkgReceiver {
	return &CkgReceiver{
		Configurations: config,
		PubSubRepo:     pubsubRepo,
		CkgRepo:        ckgRepo,
	}
}

func (r *CkgReceiver) Prepare(ctx context.Context, messages []port.IPubSubMessage) map[string][]any {
	validMessages := make(map[string][]any)

	// Extract message IDs
	messageIDs := make([]string, 0, len(messages))
	for _, msg := range messages {
		messageIDs = append(messageIDs, msg.GetID())
	}

	// Periksa semua message ID lalu hanya ambil yang belum pernah diproses saja
	existingIDs, err := r.PubSubRepo.GetIncomingIDs(messageIDs)
	if err != nil {
		logger.Debug("Gagal mengambil daftar message ID existing", "error", err)
		existingIDs = []string{}
	}

	// Process semua message satu-satu
	for _, msg := range messages {
		// Skip jika message ID sudah diproses sebelumnya
		if slices.Contains(existingIDs, msg.GetID()) {
			logger.Debug("Skip message", "id", msg.GetID())
			continue
		}

		// Parse message data
		dataStr := pubsubDataToString(msg.GetData())

		pubsubObjectWrapper := models.NewPubSubConsumerWrapper[*tbmodels.StatusPasienTBInput](r.Configurations)
		err := pubsubObjectWrapper.FromJSON(dataStr)
		if err != nil {
			logger.Debug("Gagal parsing", "id", msg.GetID(), "error", err)
			continue
		}
		logger.DebugJson("PubSub Receive Object:", pubsubObjectWrapper)

		// Hanya pedulikan Object CKG yang valid
		if !pubsubObjectWrapper.IsCKGObject() {
			logger.Debug("Abaikan message non-CKG", "id", msg.GetID())
			continue
		}

		// Simpan incomming message agar tidak diproses berulang kali
		incoming := models.IncomingMessageStatusTB{
			ID:   msg.GetID(),
			Data: &dataStr,
			// ReceivedAt:  msg.PublishTime.String(),
			ProcessedAt: nil,
		}
		if err := r.PubSubRepo.SaveNewIncoming(incoming); err != nil {
			logger.Info("Gagal menyimpan incoming message", "id", msg.GetID(), "error", err)
		}

		// register ke validMessages
		validMessages[msg.GetID()] = []any{incoming, msg, pubsubObjectWrapper.Data}
	}

	return validMessages
}

func (r *CkgReceiver) Consume(ctx context.Context, messages []port.IPubSubMessage) (map[string]bool, error) {
	results := make(map[string]bool)

	// Filter message hanya yang belum diproses saja
	validMessages := r.Prepare(ctx, messages)

	// Process each valid message
	for msgID, data := range validMessages {
		logger.DebugJson("DATA0", data)

		// incoming := data[0].(*models.IncomingMessageStatusTB)
		msg := data[1].(*ppubsub.PubSubMessage)
		rawStatusPasien := data[2].([]*tbmodels.StatusPasienTBInput)
		statusPasien := make([]tbmodels.StatusPasienTBInput, 0)
		for _, status := range rawStatusPasien {
			statusPasien = append(statusPasien, *status)
		}

		// Process the message
		err := r.Process(ctx, statusPasien, msg)
		if err != nil {
			logger.Info("Saat memproses message", "id", msgID, "error", err)
			results[msgID] = false
			continue
		}

		results[msgID] = true
	}

	return results, nil
}

func (r *CkgReceiver) Process(ctx context.Context, statusPasien []tbmodels.StatusPasienTBInput, msg *ppubsub.PubSubMessage) error {
	logger.Debug(fmt.Sprintf("Received valid CKG SkriningCKG object [%s].\n Data: %s\n Attributes: %v", msg.ID, string(msg.Data), msg.Attributes))
	logger.DebugJson("DATA", statusPasien)
	// Save to database
	_, err := r.CkgRepo.UpdateTbPatientStatus(statusPasien)
	r.PubSubRepo.UpdateIncoming(msg.ID, nil)

	return err
}

func pubsubDataToString(data []byte) string {
	rawStr := string(data)
	// dec, err := base64.StdEncoding.DecodeString(rawStr)
	// if err == nil {
	// 	return string(dec)
	// }
	return rawStr
}
