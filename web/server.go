package web

import (
	"os"

	"github.com/andrelcunha/ottermq/internal/core/broker"
	_ "github.com/andrelcunha/ottermq/web/docs"
	"github.com/andrelcunha/ottermq/web/handlers/api"
	"github.com/andrelcunha/ottermq/web/handlers/api_admin"
	"github.com/andrelcunha/ottermq/web/middleware"

	"github.com/gofiber/swagger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
)

type WebServer struct {
	config  *Config
	Broker  *broker.Broker
	Client  *amqp091.Connection
	Channel *amqp091.Channel
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
}

func (ws *WebServer) Close() {
	ws.Channel.Close()
	ws.Client.Close()
}

func NewWebServer(config *Config, broker *broker.Broker, conn *amqp091.Connection) (*WebServer, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &WebServer{
		config:  config,
		Broker:  broker,
		Client:  conn,
		Channel: ch,
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
		log.Info().Str("path", "/docs/index.html").Msg("Swagger docs enabled")
		app.Get("/docs/*", swagger.HandlerDefault)
	}

	ws.AddApi(app)

	ws.AddAdminApi(app)

	return app
}

func (ws *WebServer) AddApi(app *fiber.App) {
	// Public API routes
	app.Post("/api/login", api_admin.Login)

	// Protected API routes
	apiGrp := app.Group("/api")
	apiGrp.Get("/queues", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListQueues(c, ws.Broker)
	})
	apiGrp.Post("/queues", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.CreateQueue(c, ws.Broker)
	})
	apiGrp.Delete("/queues/:queue", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.DeleteQueue(c, ws.Broker)
	})
	apiGrp.Post("/queues/:queue/consume", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.GetMessage(c, ws.Channel)
	})
	apiGrp.Post("/messages/:id/ack", middleware.JwtMiddleware(ws.config.JwtKey), api.AckMessage)
	apiGrp.Post("/messages", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.PublishMessage(c, ws.Channel)
	})

	apiGrp.Get("/exchanges", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListExchanges(c, ws.Broker)
	})
	apiGrp.Post("/exchanges", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.CreateExchange(c, ws.Broker)
	})

	apiGrp.Delete("/exchanges/:exchange", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.DeleteExchange(c, ws.Broker)
	})
	apiGrp.Get("/bindings/:exchange", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListBindings(c, ws.Broker)
	})
	apiGrp.Post("/bindings", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.BindQueue(c, ws.Channel)
	})
	apiGrp.Delete("/bindings", middleware.JwtMiddleware(ws.config.JwtKey), api.DeleteBinding)
	apiGrp.Get("/connections", middleware.JwtMiddleware(ws.config.JwtKey), func(c *fiber.Ctx) error {
		return api.ListConnections(c, ws.Broker)
	})
}

func (ws *WebServer) AddAdminApi(app *fiber.App) {
	// Admin API routes
	apiAdminGrp := app.Group("/api/admin")
	apiAdminGrp.Use(middleware.JwtMiddleware(ws.config.JwtKey))
	apiAdminGrp.Get("/users", api_admin.GetUsers)
	apiAdminGrp.Post("/users", api_admin.AddUser)
}

func (ws *WebServer) configServer(logFile *os.File) *fiber.App {

	config := fiber.Config{

		Prefork:               false,
		AppName:               "ottermq-webadmin",
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
