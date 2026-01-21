package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/messaging"
	"github.com/CSKU-Lab/go-grader/domain/models"
	pb "github.com/CSKU-Lab/go-grader/genproto/grader/v1"
	"github.com/CSKU-Lab/go-grader/internal/infrastructure/queue"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

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

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", env.GetPort()))
	if err != nil {
		logger.Fatalln("failed to listen: ", err)
	}

	s := grpc.NewServer()
	pb.RegisterGraderServiceServer(s, newGraderGRPCServer(logger, rb))

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
	logger *zap.SugaredLogger
	q      messaging.Queue
	pb.UnimplementedGraderServiceServer
}

func newGraderGRPCServer(logger *zap.SugaredLogger, q messaging.Queue) *graderGRPCServer {
	return &graderGRPCServer{
		logger: logger,
		q:      q,
	}
}

func (s *graderGRPCServer) Run(req *pb.RunRequest, stream grpc.ServerStreamingServer[pb.RunResultResponse]) error {
	ctx := stream.Context()

	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Errorw("Cannot generate UUIDv7", "error", err)
		return status.Error(codes.Internal, "failed to generate execution ID")
	}

	var files = make([]models.File, len(req.Files))
	for i, file := range req.GetFiles() {
		files[i] = models.File{
			Name:    file.GetName(),
			Content: file.GetContent(),
		}
	}

	var limit *models.Limit
	if req.GetLimit() != nil {
		l := req.GetLimit()
		limit = &models.Limit{
			CPUTime:      l.GetCpuTime(),
			CPUExtraTime: l.GetCpuExtraTime(),
			WallTime:     l.GetWallTime(),
			Memory:       l.GetMemory(),
			Stack:        int(l.GetStack()),
			MaxOpenFiles: int(l.GetMaxOpenFiles()),
			MaxFileSize:  l.GetMaxFileSize(),
			NetworkAllow: l.GetNetworkAllow(),
		}
	}

	payload := models.RunExecution{
		ID:       id.String(),
		Files:    files,
		Input:    req.GetInput(),
		RunnerID: req.GetRunnerId(),
		Limit:    limit,
	}

	message, err := json.Marshal(&payload)
	if err != nil {
		s.logger.Errorw("Cannot parse execution struct to json", "error", err)
		return status.Error(codes.Internal, "failed to marshal execution payload")
	}

	err = s.q.PublishWithContext(ctx, "run", "run", id.String(), message)
	if err != nil {
		s.logger.Errorw("Cannot publish message to the execution queue", "error", err)
		return status.Error(codes.Internal, "failed to queue execution")
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

	err = s.q.ConsumeFromTopic(ctx, "run_results", "result."+id.String(), 1, func(msg []byte, exit chan struct{}) error {
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

func (s *graderGRPCServer) Grade(ctx context.Context, req *pb.GradeRequest) (*pb.GradeResultResponse, error) {
	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Errorw("Cannot generate UUIDv7", "error", err)
		return nil, status.Error(codes.Internal, "failed to generate execution ID")
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
		s.logger.Errorw("Cannot parse execution struct to json", "error", err)
		return nil, status.Error(codes.Internal, "failed to marshal execution payload")
	}

	err = s.q.PublishWithContext(ctx, "grade", "grade", id.String(), message)
	if err != nil {
		s.logger.Errorw("Cannot publish message to the execution queue", "error", err)
		return nil, status.Error(codes.Internal, "failed to queue execution")
	}
	s.logger.Info("Publish message to the queue successfully!")

	return &pb.GradeResultResponse{
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

	var limit *models.Limit
	if req.GetLimit() != nil {
		l := req.GetLimit()
		limit = &models.Limit{
			CPUTime:      l.GetCpuTime(),
			CPUExtraTime: l.GetCpuExtraTime(),
			WallTime:     l.GetWallTime(),
			Memory:       l.GetMemory(),
			Stack:        int(l.GetStack()),
			MaxOpenFiles: int(l.GetMaxOpenFiles()),
			MaxFileSize:  l.GetMaxFileSize(),
			NetworkAllow: l.GetNetworkAllow(),
		}
	}

	runResults := make([]*pb.TestCaseResponse, 0, len(req.GetTestcases()))
	var mu sync.Mutex

	var eg errgroup.Group
	for _, testcase := range req.GetTestcases() {
		eg.Go(func() error {
			id, err := uuid.NewV7()
			if err != nil {
				s.logger.Errorw("Cannot generate UUIDv7", "error", err)
				return status.Error(codes.Internal, "failed to generate execution ID")
			}

			payload := models.RunExecution{
				ID:       id.String(),
				Files:    files,
				Input:    testcase.GetInput(),
				RunnerID: req.GetRunnerId(),
				Limit:    limit,
			}

			message, err := json.Marshal(&payload)
			if err != nil {
				s.logger.Errorw("Cannot parse execution struct to json", "error", err)
				return status.Error(codes.Internal, "failed to marshal execution payload")
			}

			err = s.q.PublishWithContext(ctx, "run", "run", id.String(), message)
			if err != nil {
				s.logger.Errorw("Cannot publish message to the execution queue", "error", err)
				return status.Error(codes.Internal, "failed to queue execution")
			}
			s.logger.Info("Publish message to the queue successfully!")

			return s.q.ConsumeFromTopic(ctx, "run_results", "result."+id.String(), 1, func(msg []byte, exit chan struct{}) error {
				result := &models.RunResult{}
				err := json.Unmarshal(msg, result)
				if err != nil {
					s.logger.Errorln("Cannot unmarshal run result message", "error", err)
					return err
				}
				s.logger.Infof("Received run result for execution ID %s with status %s", result.ID, result.Status)

				if result.Status != execution.QUEUED && result.Status != execution.RUNNING {
					exit <- struct{}{}

					output := result.StdOut
					if output == "" {
						output = result.StdErr
					}

					mu.Lock()
					defer mu.Unlock()

					runResults = append(runResults, &pb.TestCaseResponse{
						Id:     testcase.GetId(),
						Order:  testcase.GetOrder(),
						Input:  testcase.GetInput(),
						Output: output,
					})
				}

				return nil
			})
		})
	}

	err := eg.Wait()
	if err != nil {
		s.logger.Errorw("Error during test case generation", "error", err)
		return nil, err
	}

	slices.SortFunc(runResults, func(a, b *pb.TestCaseResponse) int {
		return int(a.Order - b.Order)
	})

	return &pb.GenerateTestCasesResponse{
		Results: runResults,
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
