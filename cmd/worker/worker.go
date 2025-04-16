package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"

	pb "github.com/CSKU-Lab/go-grader/genproto/config/v1"
	"github.com/CSKU-Lab/go-grader/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/services"
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

	setupConfigDir()
	setupLanguages(langRes.Languages)
	setupCompares(compareRes.Compares)

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

func setupLanguages(languages []*pb.Language) {
	langPath := "/var/lib/worker/languages"

	for _, lang := range languages {
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
	}
	log.Println("Finish setup languages config. :D")
}

func setupCompares(compares []*pb.CompareResponse) {
	comparePath := "/var/lib/worker/compares"

	for _, compare := range compares {
		comparePath := path.Join(comparePath, compare.GetId())
		err := os.Mkdir(comparePath, 0755)
		if err != nil {
			log.Fatalln("Cannot create compare directory")
		}

		tmpPath := fmt.Sprintf("/tmp/%s", compare.GetId())
		exePath, err := buildCompareScript(tmpPath, compare)
		if err != nil {
			log.Fatalf("Cannot build compare script for %s : %s", compare.GetId(), err)
		}

		scriptPath := path.Join(comparePath, compare.GetId())
		err = moveFile(exePath, scriptPath)
		if err != nil {
			log.Fatalf("Cannot move compare script %s : %s", compare.GetId(), err)
		}

		err = os.RemoveAll(tmpPath)
		if err != nil {
			log.Fatalf("Cannot remove tmp directory of %s : %s", compare.GetId(), err)
		}

		runScriptPath := path.Join(comparePath, "run_script.sh")
		err = createRunScript(runScriptPath, compare.GetRunScript())
		if err != nil {
			log.Fatalf("Cannot create run_script.sh of %s : %s", compare.GetId(), err)
		}

		log.Printf("✅ %s setup completed", compare.GetId())
	}
	log.Println("Finish setup compares config. :D")
}

func buildCompareScript(tmpPath string, compare *pb.CompareResponse) (string, error) {
	err := os.Mkdir(tmpPath, 0705)
	if err != nil {
		return "", err
	}

	scriptName := fmt.Sprintf("%s.cpp", compare.GetName())
	scriptPath := path.Join(tmpPath, scriptName)
	err = os.WriteFile(scriptPath, []byte(compare.GetScript()), 0755)
	if err != nil {
		return "", err
	}

	buildScriptPath := path.Join(tmpPath, "build_script.sh")
	err = os.WriteFile(buildScriptPath, []byte(compare.GetBuildScript()), 0755)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("/bin/sh", "-c", buildScriptPath)
	var stdErr bytes.Buffer
	cmd.Stderr = &stdErr
	cmd.Dir = tmpPath

	err = cmd.Run()
	if err != nil {
		return "", errors.New(stdErr.String())
	}

	exePath := path.Join(tmpPath, compare.GetName())

	return exePath, nil
}

func createRunScript(path, content string) error {
	return os.WriteFile(path, []byte(content), 0755)
}

func moveFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.Chmod(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	return os.Remove(src)
}
