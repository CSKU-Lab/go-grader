package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	pb "github.com/CSKU-Lab/go-grader/genproto/config/v1"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/internal/setup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	grpc, closeGRPC := initgRPCClient()
	defer closeGRPC()

	runnerRes, err := grpc.GetRunners(ctx, &pb.GetRunnersRequest{})
	if err != nil {
		log.Fatalln("Cannot get languages from gRPC server : ", err)
	}

	compareRes, err := grpc.GetCompares(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatalln("Cannot get compares from gRPC server : ", err)
	}

	runners := runnerPbToModel(runnerRes.Runners)
	compares := comparePbToModel(compareRes.Compares)

	setup.Init(runners, compares)

	q, err := queue.NewRabbitMQ()
	if err != nil {
		log.Fatalln("Cannot initialize RabbitMQ")
	}

	runnerService := services.NewRunnerService()
	compareService := services.NewCompareService()

	isolateService := services.NewIsolateService(ctx)
	executorService := services.NewExecutorService(isolateService, runnerService, compareService)

	log.Println("Worker is ready to start working 🤖...")

	go func() {
		err := q.Consume(ctx, "running", func(message []byte) {
			execution := &models.Execution{}

			err := json.Unmarshal(message, execution)
			if err != nil {
				log.Fatalln("Cannot unmarshal message")
			}

			executor := executorService.NewExecutor()
			defer executor.Cleanup()

			executor.SetRunner(execution.RunnerID)
			executor.SetFiles(execution.Files)

			result, err := executor.Run()
			if err != nil {
				log.Fatalln("Error from runner ", err)
			}

			log.Println(result)
		})
		if err != nil {
			log.Fatalln("Cannot consume message from the queue: ", err)
		}
	}()

	go q.Consume(ctx, "grading", func(message []byte) {
		execution := &models.Execution{}

		err := json.Unmarshal(message, execution)
		if err != nil {
			log.Fatalln("Cannot unmarshal message")
		}

		executor := executorService.NewExecutor()
		executor.SetRunner(execution.RunnerID)
		executor.SetFiles(execution.Files)

		result, err := executor.Grade()
		if err != nil {
			log.Fatalln("Error from runner ", err)
		}

		log.Println(result)
	})

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs
	log.Printf("Receive %s signal from OS, going to shutdown...\n", sig)
	timer := time.AfterFunc(10*time.Second, func() {
		log.Println("Server couldn't stop grafully in time. Doing force stop.")
	})
	defer timer.Stop()
	cancel()

	q.Close()
	log.Println("RabbitMQ connection is closed.")
	closeGRPC()
	log.Println("gRPC connection is closed.")
	log.Println("Successfully gracefully shutdown the server :D")
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

func runnerPbToModel(languages []*pb.Runner) []models.RunnerConfig {
	_runners := make([]models.RunnerConfig, 0, 10)
	for _, lang := range languages {
		_runners = append(_runners, models.RunnerConfig{
			ID:          lang.GetId(),
			BuildScript: lang.GetBuildScript(),
			RunScript:   lang.GetRunScript(),
		})
	}
	return _runners
}

func comparePbToModel(compares []*pb.CompareResponse) []models.CompareConfig {
	_compares := make([]models.CompareConfig, 0, 10)
	for _, compare := range compares {
		files := make([]models.File, 0, 10)
		for _, file := range compare.GetFiles() {
			files = append(files, models.File{
				Name:    file.GetName(),
				Content: file.GetContent(),
			})
		}

		_compares = append(_compares, models.CompareConfig{
			ID:          compare.GetId(),
			Files:       files,
			BuildScript: compare.GetBuildScript(),
			RunScript:   compare.GetRunScript(),
			RunName:     compare.GetRunName(),
		})
	}
	return _compares
}
