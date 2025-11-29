package web

import (
	"os"

	"github.com/andrelcunha/ottermq/internal/core/broker"
	"github.com/andrelcunha/ottermq/web/docs"
	_ "github.com/andrelcunha/ottermq/web/docs"
	"github.com/andrelcunha/ottermq/web/handlers/api"
	"github.com/andrelcunha/ottermq/web/handlers/api_admin"
	"github.com/andrelcunha/ottermq/web/middleware"

	"github.com/gofiber/swagger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/rs/zerolog/log"
)

type WebServer struct {
	config *Config
	Broker *broker.Broker
}

type Config struct {
	BrokerHost    string
	BrokerPort    string
	Username      string
	Password      string
	JwtKey        string
	WebServerPort string
	EnableUI      bool
	EnableSwagger bool
	SwaggerPrefix string
	ApiPrefix     string
}

func NewWebServer(config *Config, broker *broker.Broker) (*WebServer, error) {
	return &WebServer{
		config: config,
		Broker: broker,
	}, nil
}

func (ws *WebServer) SetupApp(logFile *os.File) *fiber.App {

	// ws.Client = conn
	app := ws.configServer(logFile)

	// Serve static files (ui -- Vue frontend)
	if ws.config.EnableUI {
		log.Info().Msg("Web UI enabled")
		app.Static("/", "./ui")
	}
	if ws.config.EnableSwagger {
		docs.SwaggerInfo.Host = "localhost:" + ws.config.WebServerPort
		log.Info().Str("path", ws.config.SwaggerPrefix+"/index.html").Msg("Swagger docs enabled")
		app.Get(ws.config.SwaggerPrefix+"/*", swagger.HandlerDefault)
	}

	ws.AddApi(app)

	ws.AddAdminApi(app)

	return app
}

func (ws *WebServer) AddApi(app *fiber.App) {
	// Public API routes
	app.Post(ws.config.ApiPrefix+"/login", api_admin.Login)

	app.Get(ws.config.ApiPrefix+"/overview/broker", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetBasicBrokerInfo(c, ws.Broker)
	})
	// Protected API routes
	apiGrp := app.Group(ws.config.ApiPrefix)
	apiGrp.Get("/overview", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetOverview(c, ws.Broker)
	})

	// Queue routes

	apiGrp.Get("/queues", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListQueues(c, ws.Broker)
	})
	apiGrp.Get("/queues/:vhost/:queue", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetQueue(c, ws.Broker)
	})
	apiGrp.Get("/queues/:vhost/:queue/bindings", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListQueueBindings(c, ws.Broker)
	})
	apiGrp.Post("/queues/:vhost/", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.CreateQueueEmptyName(c, ws.Broker)
	})
	apiGrp.Post("/queues/:vhost/:queue", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.CreateQueue(c, ws.Broker)
	})
	apiGrp.Delete("/queues/:vhost/:queue", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.DeleteQueue(c, ws.Broker)
	})
	apiGrp.Delete("/queues/:vhost/:queue/content", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.PurgeQueue(c, ws.Broker)
	})
	apiGrp.Post("/queues/:vhost/:queue/get", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetMessage(c, ws.Broker)
	})

	apiGrp.Post("/messages", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.PublishMessage(c, ws.Broker)
	})

	// Exchange routes

	apiGrp.Get("/exchanges", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListExchanges(c, ws.Broker)
	})
	apiGrp.Get("/exchanges/:vhost/:exchange", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetExchange(c, ws.Broker)
	})
	apiGrp.Post("/exchanges", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.CreateExchange(c, ws.Broker)
	})
	apiGrp.Delete("/exchanges/:vhost/:exchange", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.DeleteExchange(c, ws.Broker)
	})
	apiGrp.Get("/exchanges/:vhost/:exchange/bindings/source", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetBindingsExchangeSource(c, ws.Broker)
	})
	apiGrp.Post("/exchanges/:vhost/:exchange/publish", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.PublishMessage(c, ws.Broker)
	})

	// Binding routes
	apiGrp.Get("/bindings", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListBindings(c, ws.Broker)
	})
	apiGrp.Get("/bindings/:vhost", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListBindings(c, ws.Broker)
	})
	apiGrp.Post("/bindings", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.BindQueue(c, ws.Broker)
	})
	apiGrp.Delete("/bindings", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.DeleteBinding(c, ws.Broker)
	})

	// Consumer routes

	apiGrp.Get("/consumers", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListConsumers(c, ws.Broker)
	})
	apiGrp.Get("/consumers/:vhost", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListVhostConsumers(c, ws.Broker)
	})
	apiGrp.Get("/queues/:vhost/:queueName/consumers", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListQueueConsumers(c, ws.Broker)
	})

	// Channel routes

	apiGrp.Get("/channels", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListChannels(c, ws.Broker)
	})
	apiGrp.Get("/channels/:vhost", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetChannelbyVhost(c, ws.Broker)
	})
	apiGrp.Get("/connections/:name/channels/:channel", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetChannel(c, ws.Broker)
	})

	// Connection routes

	apiGrp.Get("/connections", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListConnections(c, ws.Broker)
	})
	apiGrp.Get("/connections/:name/channels", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListConnectionChannels(c, ws.Broker)
	})
	apiGrp.Delete("/connections/:name", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.CloseConnection(c, ws.Broker)
	})
}

func (ws *WebServer) AddAdminApi(app *fiber.App) {
	// Admin API routes
	apiAdminGrp := app.Group(ws.config.ApiPrefix + "/admin")
	apiAdminGrp.Use(middleware.JwtMiddleware(ws.config.JwtKey))
	apiAdminGrp.Get("/users", api_admin.GetUsers)
	apiAdminGrp.Post("/users", api_admin.AddUser)
}

func (ws *WebServer) configServer(logFile *os.File) *fiber.App {

	config := fiber.Config{

		Prefork:               false,
		AppName:               "ottermq-management-ui",
		ViewsLayout:           "layout",
		DisableStartupMessage: true,
	}
	app := fiber.New(config)

	// Enable CORS
	app.Use(middleware.CORSMiddleware())

	app.Use(logger.New(logger.Config{
		Output: logFile,
	}))
	return app
}
