// Package main provides learn service grpc methods.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"unsafe"

	"learn/internal/db"
	"learn/internal/parser"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.uber.org/zap"
)

// learnserver provides grpc learn service methods.
type learnserver struct {
	db  *db.DB
	log *zap.Logger
	pb.UnimplementedLearnServiceServer
}

var tasksPool = sync.Pool{
	New: func() any {
		t := make([]parser.Task, 0, 100)
		return &t
	},
}

var wordsPool = sync.Pool{
	New: func() any {
		w := make([]parser.Word, 0, 100)
		return &w
	},
}

func main() {
	log, _ := zap.NewDevelopment()
	defer log.Sync()

	creds, err := credentials.NewServerTLSFromFile("ssl/server.crt", "ssl/server.key")
	if err != nil {
		log.Fatal("failed to create credentials", zap.Error(err))
	}

	lis, err := net.Listen("tcp", ":"+os.Getenv("LEARN_PORT"))
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	db, err := db.NewDB(log)
	if err != nil {
		log.Fatal("failed to create db", zap.Error(err))
	}

	s := learnserver{log: log, db: db}
	srv := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterLearnServiceServer(srv, &s)

	log.Debug("Learn service successfully started")

	if err := srv.Serve(lis); err != nil {
		log.Fatal("failed to serve", zap.Error(err))
	}
}

// NewTask inserts new tasks into database.
func (s *learnserver) NewTasks(ctx context.Context, req *pb.NewTasksReq) (*pb.NewTasksRes, error) {
	const op = "learnserver.NewTasks"

	data := req.GetJsonData()
	if data == "" {
		return nil, fmt.Errorf("%s: empty data", op)
	}
	reqTrace := req.GetRequestTrace()

	s.log.Debug("New Tasks request",
		zap.Int("data len", len(data)),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	tasksPtr := tasksPool.Get().(*[]parser.Task)
	*tasksPtr = (*tasksPtr)[:0]
	defer tasksPool.Put(tasksPtr)

	if err := parser.ParseTask(data, tasksPtr); err != nil {
		return nil, fmt.Errorf("%s: parse json data: %w", op, err)
	}

	rowsAffected, err := s.db.NewTaskBulk(ctx, *tasksPtr, reqTrace)
	if err != nil {
		return nil, fmt.Errorf("%s: insert tasks: %w", op, err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("%s: no rows affected", op)
	}

	s.log.Debug("Succesfully added task",
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	return &pb.NewTasksRes{Inserted: rowsAffected}, nil
}

func (s *learnserver) GetTasks(ctx context.Context, req *pb.GetTasksReq) (*pb.GetTasksRes, error) {
	const op = "learnserver.GetTasks"

	level := req.GetLevel()
	if level == "" {
		return nil, fmt.Errorf("%s: empty level", op)
	}
	pos := req.GetPosition()
	reqTrace := req.GetRequestTrace()

	s.log.Debug("Get Tasks request",
		zap.String("level", level),
		zap.Int32("position", pos),
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	tasksPtr := tasksPool.Get().(*[]parser.Task)
	*tasksPtr = (*tasksPtr)[:0]
	defer tasksPool.Put(tasksPtr)

	if err := s.db.GetTasks(ctx, level, pos, tasksPtr, reqTrace); err != nil {
		return nil, fmt.Errorf("%s: get tasks: %w", op, err)
	}

	tasksBytes, err := json.Marshal(*tasksPtr)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal tasks: %w", op, err)
	}
	tasksStr := unsafe.String(unsafe.SliceData(tasksBytes), len(tasksBytes))

	s.log.Debug("Succesfully get task",
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	return &pb.GetTasksRes{Data: tasksStr}, nil
}

func (s *learnserver) DelTask(ctx context.Context, req *pb.DelTaskReq) (*pb.DelTaskRes, error) {
	const op = "learnserver.DelTask"

	lvl := req.GetLevel()
	pos := req.GetPosition()
	reqTrace := req.GetRequestTrace()

	s.log.Debug("Delete task",
		zap.String("level", lvl),
		zap.Int32("position", pos),
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	if err := s.db.DelTask(ctx, lvl, pos, reqTrace); err != nil {
		return nil, fmt.Errorf("%s: del task: %w", op, err)
	}

	s.log.Debug("Succesfully delete task",
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	return nil, nil
}

func (s *learnserver) NewWords(ctx context.Context, req *pb.NewWordsReq) (*pb.NewWordsRes, error) {
	const op = "learnserver.NewWords"

	data := req.GetJsonData()
	if data == "" {
		return nil, fmt.Errorf("%s: empty data", op)
	}
	reqTrace := req.GetRequestTrace()

	s.log.Debug("New words request",
		zap.Int("data len", len(data)),
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	wordsPtr := wordsPool.Get().(*[]parser.Word)
	*wordsPtr = (*wordsPtr)[:0]
	defer wordsPool.Put(wordsPtr)

	if err := parser.ParseWords(data, wordsPtr); err != nil {
		return nil, fmt.Errorf("%s: parse json data: %w", op, err)
	}

	rowsAffected, err := s.db.NewWordsBulk(ctx, *wordsPtr, reqTrace)
	if err != nil {
		return nil, fmt.Errorf("%s: insert words: %w", op, err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("%s: no rows affected", op)
	}

	s.log.Debug("Succesfully added words",
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	return &pb.NewWordsRes{Inserted: rowsAffected}, nil
}

func (s *learnserver) GetWords(ctx context.Context, req *pb.GetWordsReq) (*pb.GetWordsRes, error) {
	const op = "learnserver.GetWords"

	searchData := req.GetSearchData()
	if searchData == "" {
		return nil, fmt.Errorf("%s: empty search data", op)
	}
	reqTrace := req.GetRequestTrace()

	s.log.Debug("Get words request",
		zap.Int("search_data len", len(searchData)),
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	wordsPtr := wordsPool.Get().(*[]parser.Word)
	*wordsPtr = (*wordsPtr)[:0]
	defer wordsPool.Put(wordsPtr)

	if err := s.db.GetWords(ctx, searchData, wordsPtr, reqTrace); err != nil {
		return nil, fmt.Errorf("%s: get words: %w", op, err)
	}

	wordsBytes, err := json.Marshal(*wordsPtr)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal words: %w", op, err)
	}
	wordsStr := unsafe.String(unsafe.SliceData(wordsBytes), len(wordsBytes))

	s.log.Debug("Succesfully get words",
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	return &pb.GetWordsRes{Data: wordsStr}, nil
}

func (s *learnserver) DelWord(ctx context.Context, req *pb.DelWordReq) (*pb.DelWordRes, error) {
	const op = "learnserver.DelWord"

	word := req.GetWord()
	serial := req.GetSerial()
	reqTrace := req.GetRequestTrace()

	s.log.Debug("Delete word",
		zap.String("word", word),
		zap.Int32("serial", serial),
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	if err := s.db.DelWords(ctx, word, serial, reqTrace); err != nil {
		return nil, fmt.Errorf("%s: del words: %w", op, err)
	}

	s.log.Debug("Succesfully deleted word",
		zap.String("op", op),
		zap.String("request_trace", reqTrace))

	return nil, nil
}
