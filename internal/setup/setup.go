package setup

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	"github.com/CSKU-Lab/go-grader/internal/file"
	"go.uber.org/zap"
)

func Init(logger *zap.SugaredLogger, runners []models.RunnerConfig, compares []models.CompareConfig) {
	var wg sync.WaitGroup

	setupConfigDir(logger)
	wg.Add(2)
	go setupRunners(logger, &wg, runners)
	go setupCompares(logger, &wg, compares)
	wg.Wait()

	logger.Info("Setup completed. :D")
}

func Cleanup(logger *zap.SugaredLogger) {
	err := os.RemoveAll(constants.CONFIG_DIR)
	if err != nil {
		logger.Fatalw("Cannot remove config directory", "error", err)
	}
}

func setupConfigDir(logger *zap.SugaredLogger) {
	err := os.MkdirAll(constants.CONFIG_DIR, 0755)
	if err != nil {
		logger.Fatalw("Cannot create config directory", "error", err)
	}

	err = os.Mkdir(constants.RUNNER_DIR, 0755)
	if err != nil {
		logger.Fatal("Cannot create runners directory")
	}

	err = os.Mkdir(constants.COMPARE_DIR, 0755)
	if err != nil {
		logger.Fatal("Cannot create compares directory")
	}

	logger.Info("Config directory is created.")
}

func setupRunners(logger *zap.SugaredLogger, wg *sync.WaitGroup, runners []models.RunnerConfig) {
	defer wg.Done()

	for _, runner := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runnerDir := path.Join(constants.RUNNER_DIR, runner.ID)
			err := os.Mkdir(runnerDir, 0755)
			if err != nil {
				logger.Fatalf("[setupRunners] : Cannot create %s config directory : %s", runner.ID, err)
			}

			if runner.BuildScript != "" {
				buildPath := path.Join(runnerDir, "build_script.sh")
				err := os.WriteFile(buildPath, []byte(runner.BuildScript), 0755)
				if err != nil {
					logger.Fatalf("Cannot write %s build_script.sh : %s", runner.ID, err)
				}
			}

			if runner.RunScript != "" {
				runPath := path.Join(runnerDir, "run_script.sh")
				err := os.WriteFile(runPath, []byte(runner.RunScript), 0655)
				if err != nil {
					logger.Fatalf("Cannot write %s run_script.sh : %s", runner.ID, err)
				}
			}
			logger.Infof("%s setup completed ✅", runner.ID)
		}()
	}
}

func setupCompares(logger *zap.SugaredLogger, wg *sync.WaitGroup, compares []models.CompareConfig) {
	defer wg.Done()

	isolateService := services.NewIsolateService(context.Background(), logger)
	for _, compare := range compares {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner := isolateService.NewInstance()

			comparePath := path.Join(constants.COMPARE_DIR, compare.ID)
			err := os.Mkdir(comparePath, 0755)
			if err != nil {
				logger.Fatalw("Cannot create compare directory", "error", err)
			}

			scriptPath := path.Join(comparePath, compare.RunName)

			if compare.BuildScript != "" {
				exePath, err := buildCompareScript(runner, &compare)
				if err != nil {
					logger.Fatalf("Cannot build compare script for %s : %s", compare.ID, err)
				}

				err = file.MoveFile(exePath, scriptPath)
				if err != nil {
					logger.Fatalf("Cannot move compare script %s : %s", compare.ID, err)
				}
			} else {
				for _, file := range compare.Files {
					replacedContent := replaceEnv(file.Content, scriptPath)
					err := os.WriteFile(path.Join(comparePath, file.Name), []byte(replacedContent), 0655)
					if err != nil {
						logger.Fatalf("Cannot write %s file %s : %s", compare.ID, file.Name, err)
					}
				}
			}

			runScriptPath := path.Join(comparePath, "run_script.sh")
			err = createRunScript(runScriptPath, scriptPath, compare.RunScript)
			if err != nil {
				logger.Fatalf("Cannot create run_script.sh of %s : %s", compare.ID, err)
			}
			runner.Cleanup()
			logger.Infof("Write %s to compares.json", compare.ID)

			logger.Infof("%s setup completed ✅", compare.ID)
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
