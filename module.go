package tbpubsub

import (
	"time"

	"github.com/gofiber/fiber/v2"
	appConfig "github.com/semanggilab/webcore-go/app/config"
	"github.com/semanggilab/webcore-go/app/core"
	"github.com/semanggilab/webcore-go/app/loader"
	"github.com/semanggilab/webcore-go/app/logger"
	"github.com/semanggilab/webcore-go/app/out"
	tbconfig "github.com/semanggilab/webcore-go/modules/tb/config"
	tbmodels "github.com/semanggilab/webcore-go/modules/tb/models"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/config"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/handler"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/models"
	"github.com/semanggilab/webcore-go/modules/tbpubsub/repository"
)

const (
	ModuleName    = "tbpubsub"
	ModuleVersion = "1.0.0"
	ModuleRoot    = "/pubsub"
)

// Module implements the module.Module interface
type Module struct {
	config   *config.ModuleConfig
	context  *core.AppContext
	consumer loader.IPubSub
	producer loader.IPubSub

	// Add any module-specific fields here
	repositoryTB     *repository.CKGTBRepository
	repositoryPubSub *repository.PubSubRepository
	routes           []*core.ModuleRoute
}

// NewModule creates a new Module instance
func NewModule() *Module {
	return &Module{}
}

// Name returns the unique name of the module
func (m *Module) Name() string {
	return ModuleName
}

// Version returns the version of the module
func (m *Module) Version() string {
	return ModuleVersion
}

// Dependencies returns the dependencies of the module to other modules
func (m *Module) Dependencies() []string {
	// WAJIB
	// memastikan module "tb" dimuat sebelum module ini
	// Config di module "tb" di-required oleh module ini
	return []string{"tb"}
}

// ModuleHealth returns the health status of the module
func (m *Module) Health(c *fiber.Ctx) error {
	health := map[string]any{
		"status":    "healthy",
		"module":    ModuleName,
		"version":   ModuleVersion,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	return c.JSON(health)
}

// ModuleInfo returns information about the module
func (m *Module) Info(c *fiber.Ctx) error {
	endpoints := []string{}
	for _, endpoint := range m.routes {
		endpoint := endpoint.Method + " " + endpoint.Path
		endpoints = append(endpoints, endpoint)
	}

	// path := "/" + ModuleName
	path := ModuleRoot

	info := map[string]any{
		"name":        ModuleName,
		"version":     ModuleVersion,
		"description": "TB Konsolidator API",
		"path":        path,
		"endpoints":   endpoints,
		"config":      m.config,
		"pubsub": map[string]any{
			"consumer": m.consumer != nil,
			"producer": m.producer != nil,
		},
	}
	return c.JSON(info)
}

// Init initializes the module with the given app and dependencies
func (m *Module) Init(ctx *core.AppContext) error {
	modmanager := core.Instance().ModuleManager
	modTB, err := modmanager.GetModule("tb")
	if err != nil {
		return err
	}

	// Load configuration
	m.config = &config.ModuleConfig{}
	if err := appConfig.LoadDefaultConfigModule(m.Name(), m.config); err != nil {
		return err
	}

	// inject tb.ModuleConfig, module "tb" wajib di-load sebelum module ini
	// module "tb" harus diletakkan sebagai Dependencies
	m.config.TB = modTB.Config().(*tbconfig.ModuleConfig)

	// libmanager := core.Instance().LibraryManager
	// lName := "database:" + ctx.Config.Database.Driver

	// Initialize module components
	// if lib, ok := libmanager.GetSingletonInstance(lName); ok {
	if lib, ok := ctx.GetDefaultSingletonInstance("database"); ok {
		db := lib.(loader.IDatabase)
		m.repositoryTB = repository.NewCKGTBRepository(ctx.Context, m.config, db)
		m.repositoryPubSub = repository.NewPubSubRepository(ctx.Context, m.config, db)
	}

	// PubSub utama sebagai Consumer
	lib, ok := ctx.GetDefaultSingletonInstance("pubsub")
	if ok {
		consumer := lib.(loader.IPubSub)
		m.consumer = consumer

		// start consume and process incoming message using receiver
		m.consumer.RegisterReceiver(handler.NewCkgReceiver(ctx.Context, m.config, m.repositoryTB, m.repositoryPubSub))
		m.consumer.StartReceiving(ctx.Context)

		logger.Info("PubSub Consumer", "subscription", ctx.Config.PubSub.Subscription)

		// Siapkan PubSub kedua sebagai Producer dengan Subscription ID yang berbeda
		if m.config.ProducerSubscription != "" {
			load, err := ctx.GetLibraryLoader("pubsub")
			if err != nil {
				return nil
			}

			// Gunakan konfigurasi utama lalu ganti subscription
			newConfig := ctx.Config.PubSub
			newConfig.Subscription = m.config.ProducerSubscription
			lib, err := ctx.LoadInstance(load, "producer", ctx.Context, newConfig)
			if err != nil {
				return err
			}

			producer := lib.(loader.IPubSub)
			m.producer = producer
			logger.Info("PubSub Producer", "subscription", newConfig.Subscription)
		}

		logger.Info("PubSub successfully initialize", "topic", ctx.Config.PubSub.Topic)
	}

	// Register routes
	m.registerRoutes(ctx.Root)
	m.context = ctx

	// Register services and repositories
	// These can be accessed through the central registry
	logger.Info("Module PubSub CKG TB initialized successfully")

	return nil
}

func (m *Module) Destroy() error {
	return nil
}

func (m *Module) Config() appConfig.Configurable {
	return m.config
}

func (m *Module) Routes() []*core.ModuleRoute {
	return m.routes
}

// Services returns the services provided by this module
func (m *Module) Services() map[string]any {
	// Return services that can be used by other modules
	return map[string]any{}
}

// Repositories returns the repositories provided by this module
func (m *Module) Repositories() map[string]any {
	// Return repositories that can be used by other modules
	return map[string]any{
		"repositoryTB":     m.repositoryTB,
		"repositoryPubSub": m.repositoryPubSub,
	}
}

func (m *Module) PublishPubSub(c *fiber.Ctx) error {
	if m.producer == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(out.Error(
			fiber.StatusInternalServerError,
			5,
			"PUBSUB_NOT_READY",
			"Terjadi kesalahan saat menginisiasi koneksi ke PubSub"))
	}

	var payload models.HttpProxyOutgoingMessage[any]

	// Bind body JSON dari request ke dalam variabel payload.
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(out.ErrorDetail(
			fiber.StatusBadRequest,
			4,
			"PAYLOAD_INVALID",
			"Format body salah", err))
	}

	if payload.Data == nil {
		return c.Status(fiber.StatusBadRequest).JSON(out.Error(
			fiber.StatusBadRequest,
			4,
			"PAYLOAD_INVALID",
			"field 'data' harus diisi"))
	}

	if payload.Topic == nil || *payload.Topic == "" {
		payload.Topic = &m.context.Config.PubSub.Topic
	}

	id, err := m.producer.Publish(m.context.Context, payload.Data, payload.Attributes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(out.ErrorDetail(
			fiber.StatusInternalServerError,
			3,
			"INTERNAL",
			"Gagal Publish ke PubSub",
			err))
	}
	return c.JSON(out.SuccessDataMessage(struct {
		PublishID string
	}{
		PublishID: id,
	}, "Pesan telah diteruskan ke PubSub"))
}

func (m *Module) PublishPubSubSkriningCKG(c *fiber.Ctx) error {
	if m.producer == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(out.Error(
			fiber.StatusInternalServerError,
			5,
			"PUBSUB_NOT_READY",
			"Terjadi kesalahan saat menginisiasi koneksi ke PubSub"))
	}

	var payload models.HttpProxyOutgoingMessage[tbmodels.DataSkriningTBResult]

	// Bind body JSON dari request ke dalam variabel payload.
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(out.ErrorDetail(
			fiber.StatusBadRequest,
			4,
			"PAYLOAD_INVALID",
			"Format body salah", err))
	}

	if payload.Data == nil {
		return c.Status(fiber.StatusBadRequest).JSON(out.Error(
			fiber.StatusBadRequest,
			4,
			"PAYLOAD_INVALID",
			"field 'data' harus diisi"))
	}

	if payload.Topic == nil || *payload.Topic == "" {
		payload.Topic = &m.context.Config.PubSub.Topic
	}

	data := tbmodels.DataSkriningTBResult{}
	data.FromMap(payload.Data.(map[string]any))
	pubsubObjectWrapper := models.NewPubSubProducerWrapper(m.config, []*tbmodels.DataSkriningTBResult{&data})
	dataStr, err := pubsubObjectWrapper.ToJSON()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(out.ErrorDetail(
			fiber.StatusInternalServerError,
			3,
			"INTERNAL",
			"Gagal Publish Data Skrining CKG ke PubSub",
			err))
	}
	id, err := m.producer.Publish(m.context.Context, dataStr, payload.Attributes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(out.ErrorDetail(
			fiber.StatusInternalServerError,
			3,
			"INTERNAL",
			"Gagal Publish ke PubSub",
			err))
	}
	return c.JSON(out.SuccessDataMessage(struct {
		PublishID string
	}{
		PublishID: id,
	}, "Pesan telah diteruskan ke PubSub"))
}

// registerRoutes registers the module's routes
func (m *Module) registerRoutes(root fiber.Router) {
	// Module routes
	// moduleRoot := root.Group("/" + m.Name())
	moduleRoot := root.Group(ModuleRoot) // tidak menggunakan nama module

	m.routes = core.AppendRouteToArray(m.routes, &core.ModuleRoute{
		Method:  "POST",
		Path:    "/publish", // Helper to publish pubsub message via HTTP Requiest
		Handler: m.PublishPubSub,
		Root:    moduleRoot,
	})

	m.routes = core.AppendRouteToArray(m.routes, &core.ModuleRoute{
		Method:  "POST",
		Path:    "/publish/skrining", // Helper to publish pubsub message via HTTP Requiest
		Handler: m.PublishPubSubSkriningCKG,
		Root:    moduleRoot,
	})

	m.routes = core.AppendRouteToArray(m.routes, &core.ModuleRoute{
		Method:  "GET",
		Path:    "/health",
		Handler: m.Health,
		Root:    moduleRoot,
	})

	m.routes = core.AppendRouteToArray(m.routes, &core.ModuleRoute{
		Method:  "GET",
		Path:    "/info",
		Handler: m.Info,
		Root:    moduleRoot,
	})
}
