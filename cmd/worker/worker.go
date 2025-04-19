package main

import (
	"context"
	"encoding/json"
	"log"

	pb "github.com/CSKU-Lab/go-grader/genproto/config/v1"
	"github.com/CSKU-Lab/go-grader/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/services"
	"github.com/CSKU-Lab/go-grader/setup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	ctx := context.Background()

	grpc, close := initgRPCClient()
	defer close()

	langRes, err := grpc.GetLanguages(ctx, &pb.GetLanguagesRequest{})
	if err != nil {
		log.Fatalln("Cannot get languages from gRPC server : ", err)
	}

	compareRes, err := grpc.GetCompares(ctx, &emptypb.Empty{})

	languages := langPbToModel(langRes.Languages)
	compares := comparePbToModel(compareRes.Compares)

	setup.Init(languages, compares)

	q, err := queue.NewRabbitMQ()
	if err != nil {
		log.Fatalln("Cannot initialize RabbitMQ")
	}

	isolateService := services.NewIsolateService(ctx)
	runnerService := services.NewRunnerService(isolateService)

	log.Println("Worker is ready to start working 🤖...")

	// q.Consume(ctx, "running", func(message []byte) {
	// 	execution := &models.Execution{}
	//
	// 	err := json.Unmarshal(message, execution)
	// 	if err != nil {
	// 		log.Fatalln("Cannot unmarshal message")
	// 	}
	// 	stdOut, stdErr, metadata, err := runnerService.Run(execution)
	// 	if err != nil {
	// 		log.Fatalln("Error from runner ", err)
	// 	}
	//
	// 	log.Println(stdOut, stdErr, metadata)
	// })

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

func langPbToModel(languages []*pb.Language) []models.LanguageConfig {
	_languages := make([]models.LanguageConfig, 10)
	for _, lang := range languages {
		_languages = append(_languages, models.LanguageConfig{
			ID:          lang.GetId(),
			BuildScript: lang.GetBuildScript(),
			RunScript:   lang.GetRunScript(),
			Files:       lang.GetFileNames(),
		})
	}
	return _languages
}

func comparePbToModel(compares []*pb.CompareResponse) []models.CompareConfig {
	_compares := make([]models.CompareConfig, 10)
	for _, compare := range compares {

		files := make([]models.File, 10)
		for _, file := range compare.GetFiles() {
			files = append(files, models.File{
				Name:    file.GetName(),
				Content: file.GetContent(),
			})
		}

		_compares = append(_compares, models.CompareConfig{
			ID:    compare.GetId(),
			Files: files,
		})
	}
	return _compares
}
