package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path"

	pb "github.com/CSKU-Lab/go-grader/genproto/config/v1"
	"github.com/CSKU-Lab/go-grader/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()

	grpc, close := initgRPCClient()
	defer close()

	res, err := grpc.GetLanguages(ctx, &pb.GetLanguagesRequest{})
	if err != nil {
		log.Fatalln("Cannot get languages from gRPC server : ", err)
	}

	setupLanguages(res.Languages)

	q, err := queue.NewRabbitMQ()
	if err != nil {
		log.Fatalln("Cannot initialize RabbitMQ")
	}

	isolateService := services.NewIsolateService(ctx)
	compileService := services.NewCompileService(ctx)
	languageService := services.NewLanguageConfigService()
	runnerService := services.NewRunnerService(isolateService, compileService, languageService)

	q.Consume(ctx, "execution", func(message []byte) {
		execution := &models.Execution{}

		err := json.Unmarshal(message, execution)
		if err != nil {
			log.Fatalln("Cannot unmarshal message")
		}
		stdOut, stdErr, metadata, err := runnerService.Run(execution)
		if err != nil {
			log.Fatalln("Error from runner ", err)
		}

		log.Println(stdOut, stdErr, metadata)
	})
}

func initgRPCClient() (client pb.ConfigServiceClient, close func()) {
	conn, err := grpc.NewClient("host.docker.internal:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}

	c := pb.NewConfigServiceClient(conn)

	return c, func() {
		conn.Close()
	}
}

func setupLanguages(languages []*pb.Language) {
	configPath := "/var/lib/worker/languages"

	err := os.MkdirAll(configPath, 0755)
	if err != nil {
		log.Fatalln("Cannot create config directory : ", err)
	}
	log.Println("Config directory is created.")

	for _, lang := range languages {
		langDir := path.Join(configPath, lang.GetId())
		err := os.Mkdir(langDir, 0755)
		if err != nil {
			log.Fatalf("Cannot create %s config directory", lang.Id)
		}

		if lang.GetBuildScript() != "" {
			buildPath := path.Join(langDir, "build_script.sh")
			err := os.WriteFile(buildPath, []byte(lang.GetBuildScript()), 0755)
			if err != nil {
				log.Fatalf("Cannot write %s build_script.sh", lang.GetId())
			}
		}

		if lang.GetRunScript() != "" {
			runPath := path.Join(langDir, "run_script.sh")
			err := os.WriteFile(runPath, []byte(lang.GetRunScript()), 0757)
			if err != nil {
				log.Fatalf("Cannot write %s run_script.sh", lang.GetId())
			}
		}
	}
	log.Println("Finish setup languages config. :D")
}
