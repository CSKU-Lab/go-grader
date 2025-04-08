package main

import (
	"context"
	"encoding/json"
	"log"

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

	languages, err := grpc.GetLanguages(ctx, &pb.GetLanguagesRequest{})
	if err != nil {
		log.Fatalln("Cannot get languages from gRPC server : ", err)
	}

	log.Println(languages)

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

	log.Println("Languages from gRPC server: ", languages)
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
