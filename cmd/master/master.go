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

	cskuotel "github.com/CSKU-Lab/otel"
	"github.com/CSKU-Lab/go-grader/configs"
	"github.com/CSKU-Lab/go-grader/domain/constants/broadcast"
	"github.com/CSKU-Lab/go-grader/domain/constants/execution"
	"github.com/CSKU-Lab/go-grader/domain/models"
	pb "github.com/CSKU-Lab/go-grader/genproto/grader/v1"
	"github.com/CSKU-Lab/go-grader/internal/logging"
	"github.com/CSKU-Lab/queue"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
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

	otelShutdown, err := cskuotel.Init(context.Background())
	if err != nil {
		logger.Warnw("tracing unavailable", "error", err)
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otelShutdown(shutdownCtx); err != nil {
				logger.Warnw("tracer shutdown error", "error", err)
			}
		}()
	}

	env := configs.NewEnv(logger)

	rb, err := queue.NewRabbitMQ(env.GetQueueServerURL())
	if err != nil {
		logger.Fatalw("Cannot initialize RabbitMQ", "error", err)
	}
	defer rb.Close()

	if err := rb.DeclareExchange(context.Background(), "broadcast", "fanout", true); err != nil {
		logger.Fatalw("Cannot declare broadcast fanout exchange", "error", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", env.GetPort()))
	if err != nil {
		logger.Fatalln("failed to listen: ", err)
	}

	s := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
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
	q      queue.Queue
	pb.UnimplementedGraderServiceServer
}

func newGraderGRPCServer(logger *zap.SugaredLogger, q queue.Queue) *graderGRPCServer {
	return &graderGRPCServer{
		logger: logger,
		q:      q,
	}
}

func protoActionToSystemAction(pbAction pb.BroadcastAction) broadcast.Action {
	switch pbAction {
	case pb.BroadcastAction_REFETCH_CONFIG:
		return broadcast.REFETCH_CONFIG
	default:
		return "UNKNOWN"
	}
}

func (s *graderGRPCServer) Broadcast(ctx context.Context, req *pb.BroadcastRequest) (*emptypb.Empty, error) {
	action := protoActionToSystemAction(req.GetAction())
	body, err := json.Marshal(action)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal broadcast message")
	}

	err = s.q.Publish(ctx, "broadcast", "", &queue.Derivery{
		Body: body,
	})
	if err != nil {
		s.logger.Errorw("Cannot publish message to the broadcast queue", "error", err)
		return nil, status.Error(codes.Internal, "failed to broadcast message")
	}
	s.logger.Info("Broadcast message to the queue successfully!")

	return &emptypb.Empty{}, nil
}

func (s *graderGRPCServer) Run(req *pb.RunRequest, stream grpc.ServerStreamingServer[pb.RunResultResponse]) error {
	ctx := stream.Context()

	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Errorw("Cannot generate UUIDv7", "error", err)
		return status.Error(codes.Internal, "failed to generate execution ID")
	}

	files := make([]models.File, len(req.Files))
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
			Stack:        l.GetStack(),
			MaxOpenFiles: l.GetMaxOpenFiles(),
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

	qName, err := s.q.CreateQueue(ctx, "result."+id.String(), &queue.QueueOptions{
		AutoDelete: true,
	})
	if err != nil {
		s.logger.Errorw("Cannot create result queue", "error", err)
		return status.Error(codes.Internal, "failed to create result queue")
	}

	err = s.q.Publish(ctx, "", "run", &queue.Derivery{
		Body:    message,
		ReplyTo: qName,
		Headers: cskuotel.InjectTraceHeaders(ctx),
	})
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

	err = s.q.Consume(ctx, qName, 1, false, func(derivery *queue.Derivery, exit chan struct{}) error {
		result := &models.RunResult{}
		err := json.Unmarshal(derivery.Body, result)
		if err != nil {
			s.logger.Errorln("Cannot unmarshal run result message", "error", err)
			return err
		}
		s.logger.Infof("Received run result for execution ID %s with status %s", result.ID, result.Status)

		if result.Status != execution.QUEUED && result.Status != execution.RUNNING {
			exit <- struct{}{}
		}

		return stream.Send(&pb.RunResultResponse{
			ExecutionId: result.ID,
			Status:      executionStatusToProtoStatus(result.Status),
			Output:      result.Output,
			WallTime:    result.WallTime,
			Memory:      result.Memory,
		})
	})
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	return nil
}

func (s *graderGRPCServer) GenerateTestCases(ctx context.Context, req *pb.GenerateTestCasesRequest) (*pb.GenerateTestCasesResponse, error) {
	files := make([]models.File, len(req.Files))
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
			Stack:        l.GetStack(),
			MaxOpenFiles: l.GetMaxOpenFiles(),
			MaxFileSize:  l.GetMaxFileSize(),
			NetworkAllow: l.GetNetworkAllow(),
		}
	}

	runResults := make([]*pb.TestCaseResponse, 0, len(req.GetTestcases()))
	var mu sync.Mutex

	var eg errgroup.Group
	eg.SetLimit(10)
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

			qName, err := s.q.CreateQueue(ctx, "result."+id.String(), &queue.QueueOptions{
				AutoDelete: true,
			})
			if err != nil {
				s.logger.Errorw("Cannot create result queue", "error", err)
				return status.Error(codes.Internal, "failed to create result queue")
			}

			err = s.q.Publish(ctx, "", "run", &queue.Derivery{
				CorrelationID: id.String(),
				ReplyTo:       qName,
				Body:          message,
				Headers:       cskuotel.InjectTraceHeaders(ctx),
			})
			if err != nil {
				s.logger.Errorw("Cannot publish message to the execution queue", "error", err)
				return status.Error(codes.Internal, "failed to queue execution")
			}
			s.logger.Info("Publish message to the queue successfully!")

			return s.q.Consume(ctx, qName, 1, false, func(derivery *queue.Derivery, exit chan struct{}) error {
				result := &models.RunResult{}
				err := json.Unmarshal(derivery.Body, result)
				if err != nil {
					s.logger.Errorln("Cannot unmarshal run result message", "error", err)
					return err
				}
				s.logger.Infof("Received run result for execution ID %s with status %s", result.ID, result.Status)

				if result.Status != execution.QUEUED && result.Status != execution.RUNNING {
					exit <- struct{}{}

					mu.Lock()
					defer mu.Unlock()

					runResults = append(runResults, &pb.TestCaseResponse{
						Id:     testcase.GetId(),
						Order:  testcase.GetOrder(),
						Input:  testcase.GetInput(),
						Output: result.Output,
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

func testCaseGroupsResultToProto(tcgModel []models.TestCaseGroupResult) []*pb.TestCaseGroup {
	tcgPB := make([]*pb.TestCaseGroup, 0, len(tcgModel))
	for _, tcg := range tcgModel {
		testCases := make([]*pb.TestCaseResult, 0, len(tcg.Results))
		for _, tc := range tcg.Results {
			testCases = append(testCases, &pb.TestCaseResult{
				Id:       tc.ID,
				Status:   executionStatusToProtoStatus(tc.Status),
				Message:  tc.Message,
				WallTime: tc.WallTime,
				Memory:   tc.Memory,
				Output:   tc.Output,
			})
		}

		tcgPB = append(tcgPB, &pb.TestCaseGroup{
			Id:        tcg.ID,
			Score:     tcg.Score,
			TestCases: testCases,
		})
	}

	return tcgPB
}
