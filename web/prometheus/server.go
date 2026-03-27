package prometheus

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

type Server struct {
	config   *Config
	app      *fiber.App
	exporter *Exporter
}

func NewServer(config *Config, exporter *Exporter) *Server {
	return &Server{
		config:   config,
		exporter: exporter,
	}
}

func (s *Server) Setup() {
	s.app = fiber.New(fiber.Config{
		AppName:               "ottermq-prometheus",
		DisableStartupMessage: true,
		EnablePrintRoutes:     false,
	})
	s.app.Get(s.config.Path, MetricsHandler())

	s.app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})
}

func (s *Server) Start() error {
	s.Setup()

	log.Info().
		Str("port", s.config.Port).
		Str("path", s.config.Path).
		Msg("Starting Prometheus metrics server")

	s.exporter.Start()

	return s.app.Listen(":" + s.config.Port)
}

func (s *Server) Shutdown() error {
	log.Info().Msg("Shutting down Prometheus server")
	s.exporter.Stop()
	if s.app != nil {
		return s.app.Shutdown()
	}
	return nil
}
