package setup

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	"github.com/CSKU-Lab/go-grader/internal/file"
)

func Init(runners []models.RunnerConfig, compares []models.CompareConfig) {
	var wg sync.WaitGroup

	setupConfigDir()
	wg.Add(2)
	go setupRunners(&wg, runners)
	go setupCompares(&wg, compares)
	wg.Wait()

	log.Println("Setup completed. :D")
}

func Cleanup() {
	err := os.RemoveAll(constants.CONFIG_DIR)
	if err != nil {
		log.Fatalln("Cannot remove config directory : ", err)
	}
}

func setupConfigDir() {
	err := os.MkdirAll(constants.CONFIG_DIR, 0755)
	if err != nil {
		log.Fatalln("Cannot create config directory : ", err)
	}

	err = os.Mkdir(constants.RUNNER_DIR, 0755)
	if err != nil {
		log.Fatalln("Cannot create runners directory")
	}

	err = os.Mkdir(constants.COMPARE_DIR, 0755)
	if err != nil {
		log.Fatalln("Cannot create compares directory")
	}

	log.Println("Config directory is created.")
}

func setupRunners(wg *sync.WaitGroup, runners []models.RunnerConfig) {
	defer wg.Done()

	for _, runner := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runnerDir := path.Join(constants.RUNNER_DIR, runner.ID)
			err := os.Mkdir(runnerDir, 0755)
			if err != nil {
				log.Fatalf("[setupRunners] : Cannot create %s config directory : %s", runner.ID, err)
			}

			if runner.BuildScript != "" {
				buildPath := path.Join(runnerDir, "build_script.sh")
				err := os.WriteFile(buildPath, []byte(runner.BuildScript), 0755)
				if err != nil {
					log.Fatalf("Cannot write %s build_script.sh : %s", runner.ID, err)
				}
			}

			if runner.RunScript != "" {
				runPath := path.Join(runnerDir, "run_script.sh")
				err := os.WriteFile(runPath, []byte(runner.RunScript), 0655)
				if err != nil {
					log.Fatalf("Cannot write %s run_script.sh : %s", runner.ID, err)
				}
			}
			log.Printf("%s setup completed ✅", runner.ID)
		}()
	}
}

func setupCompares(wg *sync.WaitGroup, compares []models.CompareConfig) {
	defer wg.Done()

	isolateService := services.NewIsolateService(context.Background())
	for _, compare := range compares {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner := isolateService.NewInstance()

			comparePath := path.Join(constants.COMPARE_DIR, compare.ID)
			err := os.Mkdir(comparePath, 0755)
			if err != nil {
				log.Fatalln("Cannot create compare directory : ", err)
			}

			scriptPath := path.Join(comparePath, compare.RunName)

			if compare.BuildScript != "" {
				exePath, err := buildCompareScript(runner, &compare)
				if err != nil {
					log.Fatalf("Cannot build compare script for %s : %s", compare.ID, err)
				}

				err = file.MoveFile(exePath, scriptPath)
				if err != nil {
					log.Fatalf("Cannot move compare script %s : %s", compare.ID, err)
				}
			} else {
				for _, file := range compare.Files {
					replacedContent := replaceEnv(file.Content, scriptPath)
					err := os.WriteFile(path.Join(comparePath, file.Name), []byte(replacedContent), 0655)
					if err != nil {
						log.Fatalf("Cannot write %s file %s : %s", compare.ID, file.Name, err)
					}
				}
			}

			runScriptPath := path.Join(comparePath, "run_script.sh")
			err = createRunScript(runScriptPath, scriptPath, compare.RunScript)
			if err != nil {
				log.Fatalf("Cannot create run_script.sh of %s : %s", compare.ID, err)
			}
			runner.Cleanup()
			log.Printf("Write %s to compares.json", compare.ID)

			log.Printf("%s setup completed ✅", compare.ID)
		}()
	}

	writeToComparesJson(compares)
}

func writeToComparesJson(compares []models.CompareConfig) error {
	var localCompares []models.LocalCompare
	for _, compare := range compares {
		localCompares = append(localCompares, models.LocalCompare{
			ID:      compare.ID,
			RunName: compare.RunName,
			Path:    path.Join(constants.COMPARE_DIR, compare.ID),
		})
	}

	data, err := json.Marshal(&models.LocalCompareList{
		Compares: localCompares,
	})
	if err != nil {
		return err
	}

	err = os.WriteFile(constants.COMPARE_LIST_PATH, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func buildCompareScript(runner *services.IsolateInstance, compare *models.CompareConfig) (string, error) {
	for _, file := range compare.Files {
		err := runner.CreateFile(file.Name, file.Content, 0644)
		if err != nil {
			return "", err
		}
	}

	err := runner.CreateFile("build_script.sh", compare.BuildScript, 0555)
	if err != nil {
		return "", err
	}

	err = runner.Compile()
	if err != nil {
		return "", err
	}

	exePath := path.Join(runner.BoxPath(), compare.RunName)
	return exePath, nil
}

func createRunScript(runScriptPath, scriptPath, content string) error {
	replacedContent := replaceEnv(content, scriptPath)

	return os.WriteFile(runScriptPath, []byte(replacedContent), 0655)
}

func replaceEnv(content, runName string) string {
	return strings.ReplaceAll(content, "$RUN_NAME", runName)
}
