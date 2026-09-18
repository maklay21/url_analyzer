package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	pb "backend/internal/grpcapi"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	userID := flag.String("user", "test-user", "user id (metadata)")
	mode := flag.String("mode", "analyze", "analyze|get|list")
	url := flag.String("url", "", "url to analyze")
	taskID := flag.String("task", "", "task id")
	flag.Parse()

	conn, err := grpc.Dial(*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewAnalyzerServiceClient(conn)

	// Прокидываем user-id как metadata (эмуляция шлюза)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "user-id", *userID)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	switch *mode {
	case "analyze":
		if *url == "" {
			log.Fatal("--url required")
		}
		resp, err := client.Analyze(ctx, &pb.AnalyzeRequest{Url: *url})
		if err != nil {
			log.Fatalf("analyze: %v", err)
		}
		fmt.Printf("task_id=%s status=%s\n", resp.TaskId, resp.Status)

	case "get":
		if *taskID == "" {
			log.Fatal("--task required")
		}
		resp, err := client.GetTask(ctx, &pb.GetTaskRequest{TaskId: *taskID})
		if err != nil {
			log.Fatalf("get: %v", err)
		}
		fmt.Printf("status=%s\nsummary=%s\nfile=%s\nerror=%s\n",
			resp.Status, resp.Summary, resp.FilePath, resp.ErrorMessage)

	case "list":
		resp, err := client.ListTasks(ctx, &pb.ListTasksRequest{Page: 1, PageSize: 20})
		if err != nil {
			log.Fatalf("list: %v", err)
		}
		fmt.Printf("total=%d\n", resp.Total)
		for _, t := range resp.Tasks {
			fmt.Printf("- %s [%s] %s\n", t.Url, t.Status, t.Summary)
		}

	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}
