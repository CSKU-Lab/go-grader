package configs

import (
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type env struct {
	configServerURL string
	taskServerURL   string
	rbmqServerURL   string
	port            string
	logger          *zap.SugaredLogger
}

func NewEnv(logger *zap.SugaredLogger) *env {
	if os.Getenv("ENV") == "" {
		logger.Fatal("You forget to set the ENV environment variable!")
	}

	if os.Getenv("ENV") != "docker" {
		logger.Info("Loading .env file...")

		err := godotenv.Load()
		if err != nil {
			logger.Fatalw("Error loading .env file", "error", err)
		}
	}

	return &env{
		configServerURL: os.Getenv("CONFIG_SERVER_URL"),
		taskServerURL:   os.Getenv("TASK_SERVER_URL"),
		rbmqServerURL:   os.Getenv("RBMQ_SERVER_URL"),
		port:            os.Getenv("PORT"),
		logger:          logger,
	}
}

func (m *env) GetConfigServerURL() string {
	if m.configServerURL == "" {
		m.logger.Fatal("You forget to set the CONFIG_SERVER_URL environment variable!")
	}
	return m.configServerURL
}

func (m *env) GetTaskServerURL() string {
	if m.taskServerURL == "" {
		m.logger.Fatal("You forget to set the TASK_SERVER_URL environment variable!")
	}
	return m.taskServerURL
}

func (m *env) GetQueueServerURL() string {
	if m.rbmqServerURL == "" {
		m.logger.Fatal("You forget to set the QUEUE_SERVER_URL environment variable!")
	}
	return m.rbmqServerURL
}

func (m *env) GetPort() string {
	if m.port == "" {
		m.logger.Fatal("You forget to set the PORT environment variable!")
	}
	return m.port
}
