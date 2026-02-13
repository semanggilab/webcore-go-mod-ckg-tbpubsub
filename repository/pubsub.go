package repository

import (
	"context"

	"github.com/semanggilab/webcore-go/modules/tbpubsub/config"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/models"
	"github.com/webcore-go/webcore/port"
)

type PubSub interface {
	GetIncomingIDs(messageIDs []string) ([]string, error)
	SaveNewIncoming(incoming models.IncomingMessageStatusTB) error
	UpdateIncoming(messageID string, processedAt *string) error
	DeleteIncomingMessage(dateExpired string)

	GetOutgoingIDs(messageIDs []string) ([]string, error)
	GetLastOutgoingTimestamp() (string, error)
	SaveOutgoing(outgoing models.OutgoingMessageSkriningTB) error
}

type PubSubRepository struct {
	Configurations *config.ModuleConfig
	Context        context.Context
	Connnection    port.IDatabase
}

func NewPubSubRepository(ctx context.Context, config *config.ModuleConfig, conn port.IDatabase) *PubSubRepository {
	return &PubSubRepository{
		Configurations: config,
		Context:        ctx,
		Connnection:    conn,
	}
}

func (r *PubSubRepository) GetIncomingIDs(messageIDs []string) ([]string, error) {
	// filter := port.DbMap{
	// 	"id": port.DbMap{
	// 		"$in": messageIDs,
	// 	},
	// }
	filter := []port.DbExpression{
		{Expr: "id", Op: "IN", Args: []any{messageIDs}},
	}
	var ids []port.DbMap
	err := r.Connnection.Find(r.Context, &ids, r.Configurations.TableIncoming, []string{"id"}, filter, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, entry := range ids {
		id := entry["id"]
		result = append(result, id.(string))
	}

	return result, nil
}

func (r *PubSubRepository) SaveNewIncoming(incoming models.IncomingMessageStatusTB) error {
	_, err := r.Connnection.InsertOne(r.Context, r.Configurations.TableIncoming, incoming)
	if err != nil {
		return err
	}

	return nil
}

func (r *PubSubRepository) UpdateIncoming(messageID string, processedAt *string) error {
	// filter := port.DbMap{
	// 	"id": messageID,
	// }
	filter := []port.DbExpression{
		{Expr: "id", Op: "=", Args: []any{messageID}},
	}
	update := port.DbMap{
		"processed_at": processedAt,
	}
	_, err := r.Connnection.UpdateOne(r.Context, r.Configurations.TableIncoming, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (r *PubSubRepository) DeleteIncomingMessage(dateExpired string) {
	// filter := port.DbMap{
	// 	"received_at": port.DbMap{
	// 		"$lt": dateExpired,
	// 	},
	// }
	filter := []port.DbExpression{
		{Expr: "received_at", Op: "<", Args: []any{dateExpired}},
	}
	r.Connnection.DeleteOne(r.Context, r.Configurations.TableIncoming, filter)
}

func (r *PubSubRepository) GetOutgoingIDs(messageIDs []string) ([]string, error) {
	// filter := port.DbMap{
	// 	"id": port.DbMap{
	// 		"$in": messageIDs,
	// 	},
	// }
	filter := []port.DbExpression{
		{Expr: "id", Op: "IN", Args: []any{messageIDs}},
	}
	var ids []port.DbMap
	err := r.Connnection.Find(r.Context, &ids, r.Configurations.TableOutgoing, []string{"id"}, filter, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, entry := range ids {
		id := entry["id"]
		result = append(result, id.(string))
	}
	return result, nil
}

func (r *PubSubRepository) GetLastOutgoingTimestamp() (string, error) {
	sort := map[string]int{
		"created_at": -1,
	}
	var outgoing models.OutgoingMessageSkriningTB
	err := r.Connnection.FindOne(r.Context, &outgoing, r.Configurations.TableOutgoing, nil, nil, sort)
	if err != nil {
		return "", err
	}
	return outgoing.CreatedAt, nil
}

func (r *PubSubRepository) SaveOutgoing(outgoing models.OutgoingMessageSkriningTB) error {
	var out models.OutgoingMessageSkriningTB
	// filter := port.DbMap{
	// 	"id": outgoing.ID,
	// }
	filter := []port.DbExpression{
		{Expr: "id", Op: "=", Args: []any{outgoing.ID}},
	}
	err := r.Connnection.FindOne(r.Context, &out, r.Configurations.TableOutgoing, nil, filter, nil)
	if err == nil && out.ID != "" {
		// filter := port.DbMap{
		// 	"id": out.ID,
		// }
		filter := []port.DbExpression{
			{Expr: "id", Op: "=", Args: []any{out.ID}},
		}
		update := port.DbMap{
			"updated_at": out.UpdatedAt,
		}
		r.Connnection.UpdateOne(r.Context, r.Configurations.TableOutgoing, filter, update)
	} else {
		r.Connnection.InsertOne(r.Context, r.Configurations.TableOutgoing, outgoing)
	}

	return nil
}
