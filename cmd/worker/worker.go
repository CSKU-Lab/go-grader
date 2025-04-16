package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path"
	"sync"

	pb "github.com/CSKU-Lab/go-grader/genproto/config/v1"
	"github.com/CSKU-Lab/go-grader/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/services"
	"github.com/CSKU-Lab/go-grader/utils"
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

	var wg sync.WaitGroup

	setupConfigDir()
	wg.Add(2)
	go setupLanguages(&wg, langRes.Languages)
	go setupCompares(&wg, compareRes.Compares)
	wg.Wait()

	q, err := queue.NewRabbitMQ()
	if err != nil {
		log.Fatalln("Cannot initialize RabbitMQ")
	}

	isolateService := services.NewIsolateService(ctx)
	compileService := services.NewCompileService(ctx)
	languageService := services.NewLanguageConfigService()
	runnerService := services.NewRunnerService(isolateService, compileService, languageService)

	log.Println("Worker is ready to start working 🤖...")

	q.Consume(ctx, "running", func(message []byte) {
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

func setupConfigDir() {
	configPath := "/var/lib/worker"
	err := os.MkdirAll(configPath, 0755)
	if err != nil {
		log.Fatalln("Cannot create config directory : ", err)
	}

	langPath := "/var/lib/worker/languages"
	err = os.Mkdir(langPath, 0755)
	if err != nil {
		log.Fatalln("Cannot create languages directory")
	}

	comparePath := "/var/lib/worker/compares"
	err = os.Mkdir(comparePath, 0755)
	if err != nil {
		log.Fatalln("Cannot create compares directory")
	}

	log.Println("Config directory is created.")
}

func setupLanguages(wg *sync.WaitGroup, languages []*pb.Language) {
	defer wg.Done()
	langPath := "/var/lib/worker/languages"

	for _, lang := range languages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			langDir := path.Join(langPath, lang.GetId())
			err := os.Mkdir(langDir, 0755)
			if err != nil {
				log.Fatalf("Cannot create %s config directory : %s", lang.Id, err)
			}

			if lang.GetBuildScript() != "" {
				buildPath := path.Join(langDir, "build_script.sh")
				err := os.WriteFile(buildPath, []byte(lang.GetBuildScript()), 0755)
				if err != nil {
					log.Fatalf("Cannot write %s build_script.sh : %s", lang.GetId(), err)
				}
			}

			if lang.GetRunScript() != "" {
				runPath := path.Join(langDir, "run_script.sh")
				err := os.WriteFile(runPath, []byte(lang.GetRunScript()), 0757)
				if err != nil {
					log.Fatalf("Cannot write %s run_script.sh : %s", lang.GetId(), err)
				}
			}
			log.Printf("✅ %s setup completed", lang.GetId())
		}()
	}

	log.Println("Finish setup languages config. :D")
}

func setupCompares(wg *sync.WaitGroup, compares []*pb.CompareResponse) {
	defer wg.Done()
	comparePath := "/var/lib/worker/compares"

	isolateService := services.NewIsolateService(context.Background())
	for _, compare := range compares {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner := isolateService.NewInstance()

			comparePath := path.Join(comparePath, compare.GetId())
			err := os.Mkdir(comparePath, 0755)
			if err != nil {
				log.Fatalln("Cannot create compare directory")
			}

			exePath, err := buildCompareScript(runner, compare)
			if err != nil {
				log.Fatalf("Cannot build compare script for %s : %s", compare.GetId(), err)
			}

			scriptPath := path.Join(comparePath, compare.GetRunName())
			err = utils.MoveFile(exePath, scriptPath)
			if err != nil {
				log.Fatalf("Cannot move compare script %s : %s", compare.GetId(), err)
			}

			runScriptPath := path.Join(comparePath, "run_script.sh")
			err = createRunScript(runScriptPath, compare.GetRunScript())
			if err != nil {
				log.Fatalf("Cannot create run_script.sh of %s : %s", compare.GetId(), err)
			}
			runner.Cleanup()
			log.Printf("✅ %s setup completed", compare.GetId())
		}()
	}

	log.Println("Finish setup compares config. :D")
}

func buildCompareScript(runner *services.IsolateInstance, compare *pb.CompareResponse) (string, error) {
	err := runner.CreateFile(compare.GetScriptName(), compare.GetScript())
	if err != nil {
		return "", nil
	}

	err = runner.CreateFile("build_script.sh", compare.GetBuildScript())
	if err != nil {
		return "", nil
	}

	err = runner.Run([]string{"/bin/sh", "-c", "./build_script.sh"}, nil, false)
	if err != nil {
		return "", nil
	}

	exePath := path.Join(runner.BoxPath(), compare.GetRunName())
	return exePath, nil
}

func createRunScript(path, content string) error {
	return os.WriteFile(path, []byte(content), 0755)
}
