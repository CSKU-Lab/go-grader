package configs

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type env struct {
	configServerURL string
	taskServerURL   string
}

func NewEnv() *env {
	if os.Getenv("ENV") == "" {
		log.Fatalln("You forget to set the ENV environment variable!")
	}

	if os.Getenv("ENV") != "docker" {
		log.Println("Loading .env file...")

		err := godotenv.Load()
		if err != nil {
			log.Fatalln("Error loading .env file")
		}
	}

	return &env{
		configServerURL: os.Getenv("CONFIG_SERVER_URL"),
		taskServerURL:   os.Getenv("TASK_SERVER_URL"),
	}
}

func (m *env) GetConfigServerURL() string {
	if m.configServerURL == "" {
		log.Fatalln("You forget to set the CONFIG_SERVER_URL environment variable!")
	}
	return m.configServerURL
}

func (m *env) GetTaskServerURL() string {
	if m.configServerURL == "" {
		log.Fatalln("You forget to set the TASK_SERVER_URL environment variable!")
	}
	return m.configServerURL
}
