package setup

import (
	"context"
	"log"
	"os"
	"path"
	"sync"

	"github.com/CSKU-Lab/go-grader/models"
	"github.com/CSKU-Lab/go-grader/services"
	"github.com/CSKU-Lab/go-grader/utils"
)

func Init(languages []models.LanguageConfig, compares []models.CompareConfig) {
	var wg sync.WaitGroup

	setupConfigDir()
	wg.Add(2)
	go setupLanguages(&wg, languages)
	go setupCompares(&wg, compares)
	wg.Wait()
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

func setupLanguages(wg *sync.WaitGroup, languages []models.LanguageConfig) {
	defer wg.Done()
	langPath := "/var/lib/worker/languages"

	for _, lang := range languages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			langDir := path.Join(langPath, lang.ID)
			err := os.Mkdir(langDir, 0755)
			if err != nil {
				log.Fatalf("Cannot create %s config directory : %s", lang.ID, err)
			}

			if lang.BuildScript != "" {
				buildPath := path.Join(langDir, "build_script.sh")
				err := os.WriteFile(buildPath, []byte(lang.BuildScript), 0755)
				if err != nil {
					log.Fatalf("Cannot write %s build_script.sh : %s", lang.ID, err)
				}
			}

			if lang.RunScript != "" {
				runPath := path.Join(langDir, "run_script.sh")
				err := os.WriteFile(runPath, []byte(lang.RunScript), 0757)
				if err != nil {
					log.Fatalf("Cannot write %s run_script.sh : %s", lang.ID, err)
				}
			}
			log.Printf("✅ %s setup completed", lang.ID)
		}()
	}

	log.Println("Finish setup languages config. :D")
}

func setupCompares(wg *sync.WaitGroup, compares []models.CompareConfig) {
	defer wg.Done()
	comparePath := "/var/lib/worker/compares"

	isolateService := services.NewIsolateService(context.Background())
	for _, compare := range compares {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner := isolateService.NewInstance()

			comparePath := path.Join(comparePath, compare.ID)
			err := os.Mkdir(comparePath, 0755)
			if err != nil {
				log.Fatalln("Cannot create compare directory")
			}

			exePath, err := buildCompareScript(runner, &compare)
			if err != nil {
				log.Fatalf("Cannot build compare script for %s : %s", compare.ID, err)
			}

			scriptPath := path.Join(comparePath, compare.RunName)
			err = utils.MoveFile(exePath, scriptPath)
			if err != nil {
				log.Fatalf("Cannot move compare script %s : %s", compare.ID, err)
			}

			runScriptPath := path.Join(comparePath, "run_script.sh")
			err = createRunScript(runScriptPath, compare.RunScript)
			if err != nil {
				log.Fatalf("Cannot create run_script.sh of %s : %s", compare.ID, err)
			}
			runner.Cleanup()
			log.Printf("✅ %s setup completed", compare.ID)
		}()
	}

	log.Println("Finish setup compares config. :D")
}

func buildCompareScript(runner *services.IsolateInstance, compare *models.CompareConfig) (string, error) {
	for _, file := range compare.Files {
		err := runner.CreateFile(file.Name, file.Content)
		if err != nil {
			return "", err
		}
	}

	err := runner.CreateFile("build_script.sh", compare.BuildScript)
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

func createRunScript(path, content string) error {
	return os.WriteFile(path, []byte(content), 0755)
}
