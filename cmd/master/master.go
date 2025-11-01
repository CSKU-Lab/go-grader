package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/messaging"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	pb "github.com/CSKU-Lab/go-grader/genproto/grader/v1"
	"github.com/CSKU-Lab/go-grader/internal/adapters/sqlx"
	"github.com/CSKU-Lab/go-grader/internal/generators"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func storeRunResult(ctx context.Context, logger *zap.SugaredLogger, service services.ResultService, q messaging.Queue) {
	err := q.Consume(ctx, "run_results", func(msg []byte) {
		result := &models.RunResult{}
		err := json.Unmarshal(msg, result)
		if err != nil {
			logger.Errorln("Cannot unmarshal run result message", "error", err)
		}
		err = service.CreateRunResult(ctx, result.ID, result)
		if err != nil {
			logger.Errorln("Cannot store run result to the database", "error", err)
		}
	})
	if err != nil {
		logger.Fatalw("Cannot consume run result messages", "error", err)
	}
}

func storeGradeResult(ctx context.Context, logger *zap.SugaredLogger, service services.ResultService, q messaging.Queue) {
	err := q.Consume(ctx, "grade_results", func(msg []byte) {
		result := &models.GradeResult{}
		err := json.Unmarshal(msg, result)
		if err != nil {
			logger.Errorln("Cannot unmarshal run result message", "error", err)
		}
		err = service.CreateGradeResult(ctx, result.ID, result)
		if err != nil {
			logger.Errorln("Cannot store run result to the database", "error", err)
		}
	})
	if err != nil {
		logger.Fatalw("Cannot consume run result messages", "error", err)
	}
}

func seedQueue(logger *zap.SugaredLogger, q messaging.Queue) {
	err := q.Declare("grade")
	if err != nil {
		logger.Fatalw("Cannot declare grading queue", "error", err)
	}

	err = q.Declare("run")
	if err != nil {
		logger.Fatalw("Cannot declare grading queue", "error", err)
	}

	err = q.Declare("run_results")
	if err != nil {
		logger.Fatalw("Cannot declare run_results queue", "error", err)
	}

	err = q.Declare("grade_results")
	if err != nil {
		logger.Fatalw("Cannot declare grade_results queue", "error", err)
	}
}

func main() {
	logger, loggerCleanup, err := logging.New(os.Getenv("ENV"))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := loggerCleanup(); err != nil {
			logger.Warnw("failed to flush logger", "error", err)
		}
	}()

	env := configs.NewEnv(logger)

	rb, err := queue.NewRabbitMQ(logger, env.GetQueueServerURL())
	if err != nil {
		logger.Fatalw("Cannot initialize RabbitMQ", "error", err)
	}
	defer rb.Close()

	seedQueue(logger, rb)

	db := configs.NewDB(logger, os.Getenv("DATABASE_URL"))
	resultRepo := sqlx.NewSQLXInstance(db)
	resultService := services.NewResultService(resultRepo)

	go storeRunResult(context.Background(), logger, resultService, rb)
	go storeGradeResult(context.Background(), logger, resultService, rb)

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", env.GetPort()))
	if err != nil {
		logger.Fatalln("failed to listen: ", err)
	}

	s := grpc.NewServer()
	pb.RegisterGraderServiceServer(s, newGraderGRPCServer(logger, rb, resultService))

	reflection.Register(s)
	logger.Infoln("gRPC ConfigService registered")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		logger.Info("Receive %s signal from OS, going to shutdown...\n", sig)
		timer := time.AfterFunc(10*time.Second, func() {
			logger.Infoln("Server couldn't stop grafully in time. Doing force stop.")
			s.Stop()
		})
		defer timer.Stop()
		s.GracefulStop()
		logger.Infoln("Successfully gracefully shutdown the server :D")
	}()

	if err := s.Serve(lis); err != nil {
		logger.Fatalln("Cannot start grpc server :", err)
	}
}

type graderGRPCServer struct {
	logger  *zap.SugaredLogger
	q       messaging.Queue
	service services.ResultService
	pb.UnimplementedGraderServiceServer
}

func newGraderGRPCServer(logger *zap.SugaredLogger, q messaging.Queue, service services.ResultService) *graderGRPCServer {
	return &graderGRPCServer{
		logger:  logger,
		q:       q,
		service: service,
	}
}

func (s *graderGRPCServer) Run(ctx context.Context, req *pb.RunRequest) (*pb.RunResponse, error) {
	id := generators.UUID()

	var files = make([]models.File, len(req.Files))
	for i, file := range req.GetFiles() {
		files[i] = models.File{
			Name:    file.GetName(),
			Content: file.GetContent(),
		}
	}

	execution := models.RunExecution{
		ID:       id,
		Files:    files,
		Input:    req.GetInput(),
		RunnerID: req.GetRunnerId(),
	}

	message, err := json.Marshal(&execution)
	if err != nil {
		s.logger.Fatalw("Cannot parse execution struct to json", "error", err)
	}

	err = s.q.Publish(ctx, "run", message)
	if err != nil {
		s.logger.Fatalw("Cannot publish message to the execution queue", "error", err)
	}
	s.logger.Info("Publish message to the queue successfully!")

	return &pb.RunResponse{
		ExecutionId: id,
	}, nil
}

func (s *graderGRPCServer) GetRunResult(ctx context.Context, req *pb.GetRunResultRequest) (*pb.RunResultResponse, error) {
	result, err := s.service.GetRunResultByID(ctx, req.GetExecutionId())
	if err != nil {
		return nil, err
	}

	return &pb.RunResultResponse{
		ExecutionId: result.ID,
		Status:      executionStatusToProtoStatus(result.Status),
		Output:      result.Output,
		WallTime:    result.WallTime,
		Memory:      result.Memory,
	}, nil
}

func (s *graderGRPCServer) Grade(ctx context.Context, req *pb.GradeRequest) (*pb.GradedResponse, error) {
	id := generators.UUID()

	var files = make([]models.File, len(req.Files))
	for i, file := range req.GetFiles() {
		files[i] = models.File{
			Name:    file.GetName(),
			Content: file.GetContent(),
		}
	}

	execution := models.GradeExecution{
		ID:     id,
		Files:  files,
		TaskID: req.GetTaskId(),
	}

	message, err := json.Marshal(&execution)
	if err != nil {
		s.logger.Fatalw("Cannot parse execution struct to json", "error", err)
	}

	err = s.q.Publish(ctx, "grade", message)
	if err != nil {
		s.logger.Fatalw("Cannot publish message to the execution queue", "error", err)
	}
	s.logger.Info("Publish message to the queue successfully!")

	return &pb.GradedResponse{
		ExecutionId: id,
	}, nil
}

func (s *graderGRPCServer) GetGradeResult(ctx context.Context, req *pb.GetGradeResultRequest) (*pb.GradeResultResponse, error) {
	result, err := s.service.GetGradeResultByID(ctx, req.GetExecutionId())
	if err != nil {
		return nil, err
	}

	var testCaseResults = make([]*pb.TestCaseResult, len(result.TestCaseResults))
	for i, testCaseResult := range result.TestCaseResults {
		testCaseResults[i] = &pb.TestCaseResult{
			TestCaseId: testCaseResult.ID,
			Status:     executionStatusToProtoStatus(testCaseResult.Status),
			Output:     testCaseResult.Output,
			Message:    testCaseResult.Message,
			WallTime:   testCaseResult.WallTime,
			Memory:     testCaseResult.Memory,
		}
	}

	return &pb.GradeResultResponse{
		ExecutionId:     result.ID,
		Status:          executionStatusToProtoStatus(result.Status),
		AvgWallTime:     result.AvgWallTime,
		AvgMemory:       result.AvgMemory,
		TestCaseResults: testCaseResults,
	}, nil
}

func executionStatusToProtoStatus(status execution.Status) pb.ExecutionStatus {
	switch status {
	case execution.COMPILE_FAILED:
		return pb.ExecutionStatus_STATUS_COMPILE_FAILED
	case execution.RUN_PASSED:
		return pb.ExecutionStatus_STATUS_RUN_PASSED
	case execution.RUN_FAILED:
		return pb.ExecutionStatus_STATUS_RUN_FAILED
	case execution.TIME_LIMIT_EXCEEDED:
		return pb.ExecutionStatus_STATUS_TIME_LIMIT_EXCEEDED
	case execution.MEMORY_LIMIT_EXCEEDED:
		return pb.ExecutionStatus_STATUS_MEMORY_LIMIT_EXCEEDED
	case execution.RUNTIME_ERROR:
		return pb.ExecutionStatus_STATUS_RUNTIME_ERROR
	case execution.SIGNAL_ERROR:
		return pb.ExecutionStatus_STATUS_SIGNAL_ERROR
	case execution.GRADER_ERROR:
		return pb.ExecutionStatus_STATUS_GRADER_ERROR
	default:
		return pb.ExecutionStatus_STATUS_UNSPECIFIED
	}
}
