package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/messaging"
	"github.com/CSKU-Lab/go-grader/domain/models"
	"github.com/CSKU-Lab/go-grader/domain/services"
	pb "github.com/CSKU-Lab/go-grader/genproto/grader/v1"
	"github.com/CSKU-Lab/go-grader/internal/adapters/sqlx"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

func storeGradeResult(ctx context.Context, logger *zap.SugaredLogger, service services.ResultService, q messaging.Queue) {
	err := q.Consume(ctx, "grade_results", 1, func(msg []byte) error {
		result := &models.GradeResult{}
		err := json.Unmarshal(msg, result)
		if err != nil {
			logger.Errorln("Cannot unmarshal run result message", "error", err)
			return err
		}
		err = service.CreateGradeResult(ctx, result.ID, result)
		if err != nil {
			logger.Errorln("Cannot store run result to the database", "error", err)
			return err
		}
		return nil
	})
	if err != nil {
		logger.Fatalw("Cannot consume run result messages", "error", err)
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

	db := configs.NewDB(logger, os.Getenv("DATABASE_URL"))
	resultRepo := sqlx.NewSQLXInstance(db)
	resultService := services.NewResultService(resultRepo)

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

	var wg sync.WaitGroup

	wg.Go(func() {
		sig := <-sigs
		logger.Info("Receive %s signal from OS, going to shutdown...\n", sig)
		timer := time.AfterFunc(10*time.Second, func() {
			logger.Infoln("Server couldn't stop grafully in time. Doing force stop.")
			s.Stop()
		})
		defer timer.Stop()
		s.GracefulStop()
		logger.Infoln("Successfully gracefully shutdown the server :D")
	})

	if err := s.Serve(lis); err != nil {
		logger.Fatalln("Cannot start grpc server :", err)
	}

	wg.Wait()
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

func (s *graderGRPCServer) Run(req *pb.RunRequest, stream grpc.ServerStreamingServer[pb.RunResultResponse]) error {
	ctx := stream.Context()

	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Fatalw("Cannot generate UUIDv7", "error", err)
		return err
	}

	var files = make([]models.File, len(req.Files))
	for i, file := range req.GetFiles() {
		files[i] = models.File{
			Name:    file.GetName(),
			Content: file.GetContent(),
		}
	}

	payload := models.RunExecution{
		ID:       id.String(),
		Files:    files,
		Input:    req.GetInput(),
		RunnerID: req.GetRunnerId(),
	}

	message, err := json.Marshal(&payload)
	if err != nil {
		s.logger.Fatalw("Cannot parse execution struct to json", "error", err)
	}

	err = s.q.Publish(ctx, "run", message)
	if err != nil {
		s.logger.Fatalw("Cannot publish message to the execution queue", "error", err)
	}
	s.logger.Info("Publish message to the queue successfully!")

	err = stream.Send(&pb.RunResultResponse{
		ExecutionId: id.String(),
		Status:      executionStatusToProtoStatus(execution.QUEUED),
	})
	if err != nil {
		s.logger.Errorln("Cannot send queued status to the client", "error", err)
		return status.Error(codes.Internal, err.Error())
	}

	err = s.q.ConsumeFromTopic(ctx, "topic.run_results", "result."+id.String(), 1, func(msg []byte, exit chan struct{}) error {
		result := &models.RunResult{}
		err := json.Unmarshal(msg, result)
		if err != nil {
			s.logger.Errorln("Cannot unmarshal run result message", "error", err)
			return err
		}
		s.logger.Infof("Received run result for execution ID %s with status %s", result.ID, result.Status)

		if result.Status != execution.QUEUED && result.Status != execution.RUNNING {
			exit <- struct{}{}
		}

		output := result.StdOut
		if output == "" {
			output = result.StdErr
		}

		return stream.Send(&pb.RunResultResponse{
			ExecutionId: result.ID,
			Status:      executionStatusToProtoStatus(result.Status),
			Output:      output,
			WallTime:    result.WallTime,
			Memory:      result.Memory,
		})
	})
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	return nil
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
	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Fatalw("Cannot generate UUIDv7", "error", err)
		return nil, err
	}

	var files = make([]models.File, len(req.Files))
	for i, file := range req.GetFiles() {
		files[i] = models.File{
			Name:    file.GetName(),
			Content: file.GetContent(),
		}
	}

	execution := models.GradeExecution{
		ID:     id.String(),
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
		ExecutionId: id.String(),
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

func (s *graderGRPCServer) GenerateTestCases(ctx context.Context, req *pb.GenerateTestCasesRequest) (*pb.GenerateTestCasesResponse, error) {
	var files = make([]models.File, len(req.Files))
	for i, file := range req.GetFiles() {
		files[i] = models.File{
			Name:    file.GetName(),
			Content: file.GetContent(),
		}
	}

	runResults := make([]*pb.RunResultResponse, 0, len(req.GetInputs()))

	var eg errgroup.Group
	for _, input := range req.GetInputs() {
		eg.Go(func() error {
			id, err := uuid.NewV7()
			if err != nil {
				return errors.New("Cannot generate UUIDv7")
			}

			payload := models.RunExecution{
				ID:       id.String(),
				Files:    files,
				Input:    input,
				RunnerID: req.GetRunnerId(),
			}

			message, err := json.Marshal(&payload)
			if err != nil {
				return errors.New("Cannot parse execution struct to json")
			}

			err = s.q.Publish(ctx, "run", message)
			if err != nil {
				return errors.New("Cannot publish message to the execution queue")
			}
			s.logger.Info("Publish message to the queue successfully!")

			return s.q.ConsumeFromTopic(ctx, "topic.run_results", "result."+id.String(), 1, func(msg []byte, exit chan struct{}) error {
				result := &models.RunResult{}
				err := json.Unmarshal(msg, result)
				if err != nil {
					return errors.New("Cannot unmarshal run result message")
				}
				s.logger.Infof("Received run result for execution ID %s with status %s", result.ID, result.Status)

				if result.Status != execution.QUEUED && result.Status != execution.RUNNING {
					exit <- struct{}{}
				}

				output := result.StdOut
				if output == "" {
					output = result.StdErr
				}

				runResults = append(runResults, &pb.RunResultResponse{
					ExecutionId: result.ID,
					Status:      executionStatusToProtoStatus(result.Status),
					Output:      output,
					WallTime:    result.WallTime,
					Memory:      result.Memory,
				})

				return nil
			})
		})
	}

	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	return &pb.GenerateTestCasesResponse{
		RunResults: runResults,
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
	case execution.QUEUED:
		return pb.ExecutionStatus_STATUS_QUEUED
	case execution.RUNNING:
		return pb.ExecutionStatus_STATUS_RUNNING
	default:
		return pb.ExecutionStatus_STATUS_UNSPECIFIED
	}
}
