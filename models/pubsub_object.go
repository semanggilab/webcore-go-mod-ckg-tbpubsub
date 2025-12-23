package models

import (
	"fmt"
	"reflect"

	"github.com/semanggilab/webcore-go/app/helper"
	"github.com/semanggilab/webcore-go/app/logger"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/config"
)

const (
	PUBSUB_CONSUME = iota
	PUBSUB_PRODUCE
)

// PubSubObject is the base class for CKG data objects
type PubSubObjectWrapper[T PubSubObject] struct {
	CKGObject bool
	Type      int
	Config    *config.ModuleConfig
	Data      []T `json:"data"`
}

type PubSubObject interface {
	FromMap(data map[string]any)
	ToMap() map[string]any
}

func NewPubSubConsumerWrapper[T PubSubObject](config *config.ModuleConfig) PubSubObjectWrapper[T] {
	return PubSubObjectWrapper[T]{
		Type:   PUBSUB_CONSUME,
		Config: config,
	}
}

func NewPubSubProducerWrapper[T PubSubObject](config *config.ModuleConfig, data []T) PubSubObjectWrapper[T] {
	return PubSubObjectWrapper[T]{
		Type:   PUBSUB_PRODUCE,
		Config: config,
		Data:   data,
	}
}

// FromMap creates a PubSubObject from a map
func (t *PubSubObjectWrapper[T]) DataFromMap(obj map[string]any) error {
	logger.DebugJson("PubSub Receive MapObject:", obj)

	cfg := t.Config

	markerValueObj, ok := obj[cfg.Marker.Field].(string)
	if !ok || markerValueObj == "" {
		return fmt.Errorf("Field Marker tidak ditemukan")
	}

	markerValueStruct := ""
	switch t.Type {
	case PUBSUB_PRODUCE:
		markerValueStruct = cfg.Marker.Produce
	case PUBSUB_CONSUME:
		markerValueStruct = cfg.Marker.Consume
	}

	logger.Debug("PubSub Receive CheckObject:", "markerKey", cfg.Marker.Field, "presentValue", markerValueObj, "requiredValue", markerValueStruct)

	// Check if this is a CKG object
	if markerValueObj == markerValueStruct {
		t.CKGObject = true
		t.Data = make([]T, 0)

		// Ambil type data
		var zero T
		r := reflect.TypeOf(zero)

		if r.Kind() == reflect.Ptr {
			r = r.Elem()
		}

		if data, ok := obj["data"].([]any); ok {
			for _, item := range data {
				if dataMap, ok := item.(map[string]any); ok {
					// buat instance
					x := reflect.New(r).Interface().(T)

					x.FromMap(dataMap)
					t.Data = append(t.Data, x)
				}
			}
		}
	}

	return nil
}

func (t *PubSubObjectWrapper[T]) FromJSON(jsonStr string) error {
	data := make(map[string]any)

	if err := helper.JSONUnmarshal([]byte(jsonStr), &data); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	t.DataFromMap(data)
	return nil
}

func (t *PubSubObjectWrapper[T]) DataToMap() map[string]any {
	data := make(map[string]any)

	items := make([]any, 0)
	for _, item := range t.Data {
		items = append(items, item.ToMap())
	}

	data["data"] = items
	return data
}

// ToJSON converts PubSubObject to JSON string
func (t *PubSubObjectWrapper[T]) ToJSON() (string, error) {
	data := t.DataToMap()
	cfg := t.Config

	// Add marker field
	if t.Type == PUBSUB_PRODUCE {
		data[cfg.Marker.Field] = cfg.Marker.Produce
	} else {
		data[cfg.Marker.Field] = cfg.Marker.Consume
	}

	jsonBytes, err := helper.JSONMarshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %v", err)
	}

	return string(jsonBytes), nil
}

func (t *PubSubObjectWrapper[T]) IsCKGObject() bool {
	return t.CKGObject
}
