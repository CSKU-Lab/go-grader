package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"

	"github.com/CSKU-Lab/go-grader/domain/constants"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"go.uber.org/zap"
)

// limitedBuffer captures up to max bytes and silently discards the rest. Write
// always reports the full length so it never blocks the process writing to it
// (a blocked stdout pipe would hang a runaway program instead of letting its
// limits kill it). truncated records whether any output was dropped.
type limitedBuffer struct {
	max       int
	buf       []byte
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if remaining := b.max - len(b.buf); remaining > 0 {
		if len(p) <= remaining {
			b.buf = append(b.buf, p...)
		} else {
			b.buf = append(b.buf, p[:remaining]...)
			b.truncated = true
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string { return string(b.buf) }

type IsolateService struct {
	runBoxIds   chan int
	gradeBoxIds chan int
	logger      *zap.SugaredLogger
}

func NewIsolateService(logger *zap.SugaredLogger, runQueueAmount int, gradeQueueAmount int) *IsolateService {
	// Run pool: IDs 0 to runQueueAmount-1
	runBoxIds := make(chan int, runQueueAmount)
	for i := range runQueueAmount {
		runBoxIds <- i
	}

	// Grade pool: IDs runQueueAmount to runQueueAmount + (gradeQueueAmount*2) - 1
	gradePoolSize := gradeQueueAmount * 2
	gradeBoxIds := make(chan int, gradePoolSize)
	for i := range gradePoolSize {
		gradeBoxIds <- runQueueAmount + i
	}

	return &IsolateService{
		runBoxIds:   runBoxIds,
		gradeBoxIds: gradeBoxIds,
		logger:      logger,
	}
}

type IsolateInstance struct {
	boxID        int
	boxPath      string
	metadataPath string
	comparePath  string
	boxIds       chan int
	logger       *zap.SugaredLogger
}

func (s *IsolateService) NewRunInstance() *IsolateInstance {
	boxID := <-s.runBoxIds

	instance := IsolateInstance{
		boxID:        boxID,
		boxPath:      fmt.Sprintf(constants.SANDBOX_PATH+"/box", boxID),
		metadataPath: fmt.Sprintf(constants.SANDBOX_PATH+"/metadata", boxID),
		comparePath:  fmt.Sprintf(constants.SANDBOX_PATH+"/compare", boxID),
		boxIds:       s.runBoxIds,
		logger:       s.logger,
	}

	return &instance
}

// GradePoolSize returns the number of grade sandbox boxes available. Used to
// bound grade fan-out so waiting goroutines never exceed usable sandboxes.
func (s *IsolateService) GradePoolSize() int {
	return cap(s.gradeBoxIds)
}

func (s *IsolateService) NewGradeInstance() *IsolateInstance {
	boxID := <-s.gradeBoxIds

	instance := IsolateInstance{
		boxID:        boxID,
		boxPath:      fmt.Sprintf(constants.SANDBOX_PATH+"/box", boxID),
		metadataPath: fmt.Sprintf(constants.SANDBOX_PATH+"/metadata", boxID),
		comparePath:  fmt.Sprintf(constants.SANDBOX_PATH+"/compare", boxID),
		boxIds:       s.gradeBoxIds,
		logger:       s.logger,
	}

	return &instance
}

func (i *IsolateInstance) log(format string, args ...any) {
	i.logger.Infof("[Instance:%d] :: %s", i.boxID, fmt.Sprintf(format, args...))
}

func (i *IsolateInstance) logFatalf(format string, args ...any) {
	i.logger.Fatalf("[Instance:%d] :: %s", i.boxID, fmt.Sprintf(format, args...))
}

// maxIsolateOutputBytes caps how much program output we buffer in the worker.
// isolate 2.0 on cgroup v2 only enforces --time/--wall-time/--mem/--fsize when
// --cg is set (see isolateGlobalArgs); before those limits kill a runaway
// program it can still emit a burst of stdout. cmd.Output() would buffer all of
// it in memory and OOM the worker, so we cap the capture. 16 MiB is far above
// any legitimate program output and well under the pod memory limit.
const maxIsolateOutputBytes = 16 << 20

// isolateGlobalArgs are prepended to every isolate invocation for a box.
//
// NOTE: no --cg. isolate's wall-time limit already kills a runaway program
// (including one forked from run_script.sh) without control groups — verified
// against isolate 2.0 in this image. --cg additionally requires a delegated
// cgroup subtree (/run/isolate/cgroup) that is not set up in the worker pod, so
// enabling it here breaks isolate entirely. Proper --cg support (needed only for
// per-cgroup CPU-time accounting of multi-process programs) is tracked
// separately as an infra change.
func (i *IsolateInstance) isolateGlobalArgs() []string {
	return []string{fmt.Sprintf("--box-id=%d", i.boxID)}
}

// run executes isolate and returns ONLY its stdout (the program output used for
// comparison). isolate writes its own diagnostics — including the
// "(0.008 sec real, 0.008 sec wall)" summary line — to stderr; that must NOT be
// merged into the returned output or it pollutes the compare and flips passing
// submissions to wrong. stderr is captured separately and surfaced only in the
// error. Program stderr is already folded into stdout by isolate's
// --stderr-to-stdout on program runs.
func (i *IsolateInstance) run(cmd *exec.Cmd) (string, error) {
	stdout := &limitedBuffer{max: maxIsolateOutputBytes}
	stderr := &limitedBuffer{max: 64 << 10}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil && stderr.String() != "" {
		err = fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), err
}

func (i *IsolateInstance) execute(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "isolate", append(i.isolateGlobalArgs(), args...)...)
	return i.run(cmd)
}

func (i *IsolateInstance) executeWithInput(ctx context.Context, input string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "isolate", append(i.isolateGlobalArgs(), args...)...)
	cmd.Stdin = strings.NewReader(input)
	return i.run(cmd)
}

func (i *IsolateInstance) Init(ctx context.Context) {
	i.log("Initializing sandbox...")
	// cleanup any leftover state from a previous crashed run before init
	i.execute(ctx, "--cleanup")
	_, err := i.execute(ctx, "--init")
	if err != nil {
		i.logFatalf("Error on init: %s", err)
	}
}

func (i *IsolateInstance) ID() int {
	return i.boxID
}

func (i *IsolateInstance) BoxPath() string {
	return i.boxPath
}

func (i *IsolateInstance) Cleanup() {
	i.log("Cleaning up sandbox...")
	// Use background context to ensure cleanup always runs even if parent context is canceled
	_, err := i.execute(context.Background(), "--cleanup")
	if err != nil {
		i.logFatalf("Cleanup failed, process must crash: %s", err)
	}

	i.boxIds <- i.boxID
}

func (i *IsolateInstance) CreateFile(name string, content string, filePerm os.FileMode) error {
	i.log("Creating file %s...", name)
	filePath := fmt.Sprintf("%s/%s", i.boxPath, name)
	return os.WriteFile(filePath, []byte(content), filePerm)
}

func (i *IsolateInstance) CreateDir(name string, filePerm os.FileMode) error {
	i.log("Creating directory %s...", name)
	dirPath := fmt.Sprintf("%s/%s", i.boxPath, name)
	return os.Mkdir(dirPath, filePerm)
}

func (i *IsolateInstance) RemoveDir(name string) error {
	i.log("Removing directory %s...", name)
	dirPath := fmt.Sprintf("%s/%s", i.boxPath, name)
	return os.RemoveAll(dirPath)
}

func (i *IsolateInstance) Compile(ctx context.Context) (string, error) {
	i.log("Compiling program...")

	args := []string{
		"--env=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--processes=0",
		"--stderr-to-stdout",
		"--run",
		"--",
		"build_script.sh",
	}

	output, err := i.execute(ctx, args...)
	if err != nil {
		i.log("Compile error : %s", output)
		return output, err
	}

	i.log("Compile done.")

	return output, err
}

func (i *IsolateInstance) CompileUsing(ctx context.Context, scriptDir string) (string, error) {
	i.log("Compiling program...")

	args := []string{
		"--env=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		fmt.Sprintf("--dir=%s", scriptDir),
		"--processes=0",
		"--stderr-to-stdout",
		"--run",
		"--",
		fmt.Sprintf("%s/build_script.sh", scriptDir),
	}

	output, err := i.execute(ctx, args...)
	if err != nil {
		i.log("Compile error : %s", err.Error())
		return output, err
	}

	i.log("Compile done.")

	return output, nil
}

func (i *IsolateInstance) Run(ctx context.Context, scriptDir string, input string, limit *models.Limit) (string, error) {
	i.log("Running program...")
	_limits := getLimitArgs(limit)

	args := []string{
		"--meta=" + i.metadataPath,
		"--processes=100",
		"--stderr-to-stdout",
		"--run",
		"--",
		"run_script.sh",
	}

	args = append(_limits, args...)

	var output string
	var err error

	if input != "" {
		output, err = i.executeWithInput(ctx, input, args...)
	} else {
		output, err = i.execute(ctx, args...)
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// recheck if the command you ran with isolate command is exist
			if exitErr.ExitCode() == 127 {
				return output, errors.New("the command you pass to isolate is not exist")
			}
		}
		return output, err
	}
	return output, nil
}

func (i *IsolateInstance) RunWithEnvVars(ctx context.Context, limit *models.Limit, envVars ...string) (string, error) {
	i.log("Running program with env vars...")
	_limits := getLimitArgs(limit)

	args := []string{
		"--meta=" + i.metadataPath,
		"--processes=100",
		"--stderr-to-stdout",
	}

	for _, env := range envVars {
		args = append(args, fmt.Sprintf("--env=%s", env))
	}

	args = append(args, "--run", "--", "run_script.sh")
	args = append(_limits, args...)

	output, err := i.execute(ctx, args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 127 {
				return output, errors.New("the command you pass to isolate is not exist")
			}
			// exit 1 is normal for compare scripts indicating mismatch
			if exitErr.ExitCode() == 1 {
				return output, nil
			}
		}
		return output, err
	}
	return output, nil
}

func (i *IsolateInstance) RunCompare(ctx context.Context) (output string, exitCode int, err error) {
	i.log("Running compare script...")

	args := []string{
		"--meta=" + i.metadataPath,
		"--processes=100",
		"--stderr-to-stdout",
		"--run", "--", "run_script.sh",
	}

	output, err = i.execute(ctx, args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			if exitCode == 127 {
				return output, exitCode, errors.New("the command you pass to isolate is not exist")
			}
			return output, exitCode, nil
		}
		return output, -1, err
	}
	return output, 0, nil
}

func (i *IsolateInstance) RunWithDir(ctx context.Context, mountDir string, limit *models.Limit) (string, error) {
	i.log("Running program with mounted dir %s...", mountDir)
	_limits := getLimitArgs(limit)

	args := []string{
		"--meta=" + i.metadataPath,
		fmt.Sprintf("--dir=%s", mountDir),
		"--processes=100",
		"--stderr-to-stdout",
	}

	args = append(args, "--run", "--", "run_script.sh")
	args = append(_limits, args...)

	output, err := i.execute(ctx, args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 127 {
				return output, errors.New("the command you pass to isolate is not exist")
			}
			// exit 1 is normal for compare scripts indicating mismatch
			if exitErr.ExitCode() == 1 {
				return output, nil
			}
		}
		return output, err
	}
	return output, nil
}

func (i *IsolateInstance) RunFromDir(ctx context.Context, scriptDir string, input string, limit *models.Limit, envVars ...string) (string, error) {
	i.log("Running program...")
	_limits := getLimitArgs(limit)

	args := []string{
		"--meta=" + i.metadataPath,
		fmt.Sprintf("--dir=%s", scriptDir),
		"--processes=100",
		"--stderr-to-stdout",
	}

	for _, env := range envVars {
		args = append(args, fmt.Sprintf("--env=%s", env))
	}

	args = append(args, "--run", "--", fmt.Sprintf("%s/run_script.sh", scriptDir))
	args = append(_limits, args...)

	var output string
	var err error

	if input != "" {
		output, err = i.executeWithInput(ctx, input, args...)
	} else {
		output, err = i.execute(ctx, args...)
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// recheck if the command you ran with isolate command is exist
			if exitErr.ExitCode() == 127 {
				return output, errors.New("the command you pass to isolate is not exist")
			}
		}
		return output, err
	}
	return output, nil
}

func (i *IsolateInstance) GetMetadata() (*models.Metadata, error) {
	i.log("Getting Metadata...")
	data, err := os.ReadFile(i.metadataPath)
	if err != nil {
		return nil, fmt.Errorf("Cannot read metadata file : %s", err)
	}

	return models.ParseMetadata(string(data))
}

func (i *IsolateInstance) GetCompareResult() (string, error) {
	data, err := os.ReadFile(i.boxPath + "/compare_result.txt")
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func getLimitArgs(limit *models.Limit) []string {
	if limit == nil {
		return nil
	}

	v := reflect.ValueOf(limit).Elem()
	t := v.Type()

	args := make([]string, 0)

	for i := range v.NumField() {
		field := v.Field(i)
		arg := t.Field(i).Tag.Get("arg")

		switch field.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Int() == 0 {
				continue
			}
			args = append(args, fmt.Sprintf("%s=%d", arg, field.Int()))
		case reflect.Float32, reflect.Float64:
			f := field.Float()
			if f == 0 {
				continue
			}
			// isolate parses size options (e.g. --fsize) as integers and time
			// options as integer-or-fractional. FormatFloat with -1 precision
			// keeps whole numbers integral ("5120", not "5120.000000") and never
			// emits exponent form ("1e+06"), both of which isolate rejects.
			args = append(args, fmt.Sprintf("%s=%s", arg, strconv.FormatFloat(f, 'f', -1, 64)))
		case reflect.Bool:
			if !field.Bool() {
				continue
			}
			args = append(args, arg)
		}
	}

	return args
}
