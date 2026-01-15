package config

import tbconfig "github.com/semanggilab/webcore-go/modules/tb/config"

type ModuleConfig struct {
	TB            *tbconfig.ModuleConfig // refer to config in tb.ModuleConfig
	TableIncoming string                 `mapstructure:"tableincoming"`
	TableOutgoing string                 `mapstructure:"tableoutgoing"`
	Marker        PubSubMarker           `mapstructure:"marker"`
	ProducerTopic string                 `mapstructure:"producer_topic"`
}

type PubSubMarker struct {
	Field   string `mapstructure:"field"`
	Consume string `mapstructure:"consume"`
	Produce string `mapstructure:"produce"`
}

func (c *ModuleConfig) SetEnvBindings() map[string]string {
	return map[string]string{
		"module.tbpubsub.tableincoming":  "MODULE_TBPUBSUB_CKG_TABLE_INCOMING",
		"module.tbpubsub.tableoutgoing":  "MODULE_TBPUBSUB_CKG_TABLE_OUTGOING",
		"module.tbpubsub.producer_topic": "MODULE_TBPUBSUB_PRODUCER_TOPIC",
		"module.tbpubsub.marker.field":   "MODULE_TBPUBSUB_MARKER_FIELD",
		"module.tbpubsub.marker.consume": "MODULE_TBPUBSUB_MARKER_CONSUME",
		"module.tbpubsub.marker.produce": "MODULE_TBPUBSUB_MARKER_PRODUCE",
	}
}

func (c *ModuleConfig) SetDefaults() map[string]any {
	return map[string]any{
		"module.tbpubsub.tableincoming":  "ckg_pubsub_incoming",
		"module.tbpubsub.tableoutgoing":  "ckg_pubsub_outgoing",
		"module.tbpubsub.producer_topic": "",
		"module.tbpubsub.marker.field":   "transactionSource",
		"module.tbpubsub.marker.consume": "STATUS-PASIEN-TB",
		"module.tbpubsub.marker.produce": "SKRINING-CKG-TB",
	}
}
