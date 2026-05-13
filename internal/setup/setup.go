package setup

import (
	"context"
	"encoding/json"
	"errors"
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

func isDirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func setupConfigDir(logger *zap.SugaredLogger) {
	isConfigDirExists, err := isDirExists(constants.CONFIG_DIR)
	if err != nil {
		logger.Fatalw("Cannot check config directory", "error", err)
	}

	if !isConfigDirExists {
		err := os.MkdirAll(constants.CONFIG_DIR, 0755)
		if err != nil {
			logger.Fatalw("Cannot create config directory", "error", err)
		}
	}

	isRunnerDirExists, err := isDirExists(constants.RUNNER_DIR)
	if err != nil {
		logger.Fatalw("Cannot check config directory", "error", err)
	}

	if !isRunnerDirExists {
		err = os.Mkdir(constants.RUNNER_DIR, 0755)
		if err != nil {
			logger.Fatalw("Cannot create runners directory", "error", err)
		}
	}

	isCompareDirExists, err := isDirExists(constants.COMPARE_DIR)
	if err != nil {
		logger.Fatalw("Cannot check compare directory", "error", err)
	}

	if !isCompareDirExists {
		err = os.Mkdir(constants.COMPARE_DIR, 0755)
		if err != nil {
			logger.Fatal("Cannot create compares directory")
		}
	}

	logger.Info("Config directory is created.")
}

func setupRunners(logger *zap.SugaredLogger, wg *sync.WaitGroup, runners []models.RunnerConfig) {
	defer wg.Done()

	for _, runner := range runners {
		wg.Go(func() {
			runnerDir := path.Join(constants.RUNNER_DIR, runner.ID)
			isRunnerDirExists, err := isDirExists(runnerDir)
			if err != nil {
				logger.Fatalf("[setupRunners] : Cannot check %s config directory : %s", runner.ID, err)
			}

			if isRunnerDirExists {
				logger.Infof("%s already exists, skipping...", runner.ID)
				return
			}

			err = os.Mkdir(runnerDir, 0755)
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
		})
	}
}

func setupCompares(logger *zap.SugaredLogger, wg *sync.WaitGroup, compares []models.CompareConfig) {
	defer wg.Done()

	isolateService := services.NewIsolateService(logger, len(compares), 0)
	for _, compare := range compares {
		wg.Go(func() {
			if compare.RunName == "" || len(compare.Files) == 0 {
				logger.Warnf("[setupCompares] %s skipped: not fully configured (run_name=%q, files=%d)", compare.ID, compare.RunName, len(compare.Files))
				return
			}

			ctx := context.Background()
			runner := isolateService.NewRunInstance()
			runner.Init(ctx)

			comparePath := path.Join(constants.COMPARE_DIR, compare.ID)

			isComapreDirExists, err := isDirExists(comparePath)
			if err != nil {
				logger.Fatalf("[setupRunners] : Cannot check %s config directory : %s", runner.ID, err)
			}

			if isComapreDirExists {
				logger.Infof("%s already exists, skipping...", runner.ID)
				return
			}

			err = os.Mkdir(comparePath, 0755)
			if err != nil {
				logger.Fatalw("Cannot create compare directory", "error", err)
			}

			scriptPath := path.Join(comparePath, compare.RunName)

			if compare.BuildScript != "" {
				exePath, err := buildCompareScript(ctx, runner, &compare)
				if err != nil {
					logger.Fatalf("Cannot build compare script for %s : %s", compare.ID, err)
				}

				if _, statErr := os.Stat(exePath); os.IsNotExist(statErr) {
					logger.Warnf("[setupCompares] %s skipped: build script ran but produced no %q output — configure the compare script first", compare.ID, compare.RunName)
					runner.Cleanup()
					os.RemoveAll(comparePath)
					return
				}

				err = file.MoveFile(exePath, scriptPath)
				if err != nil {
					logger.Fatalf("Cannot move compare script %s : %s", compare.ID, err)
				}
			} else {
				hasRunFile := false
				for _, f := range compare.Files {
					if f.Name == compare.RunName {
						hasRunFile = true
					}
					replacedContent := replaceEnv(f.Content, scriptPath)
					err := os.WriteFile(path.Join(comparePath, f.Name), []byte(replacedContent), 0655)
					if err != nil {
						logger.Fatalf("Cannot write %s file %s : %s", compare.ID, f.Name, err)
					}
				}
				if !hasRunFile {
					logger.Warnf("[setupCompares] %s skipped: no file named %q found in sandbox — configure the compare script first", compare.ID, compare.RunName)
					runner.Cleanup()
					os.RemoveAll(comparePath)
					return
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
		})
	}

	writeToComparesJson(compares)
}

func writeToComparesJson(compares []models.CompareConfig) error {
	var localCompares []models.LocalCompare
	for _, compare := range compares {
		if compare.RunName == "" || len(compare.Files) == 0 {
			continue
		}
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

func buildCompareScript(ctx context.Context, runner *services.IsolateInstance, compare *models.CompareConfig) (string, error) {
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

	_, err = runner.Compile(ctx)
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
