// Package learn words.go calls learn-service grpc methods
// responsible for words gprc methods
package learn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unsafe"

	"enbstr/internal/services"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
	"go.uber.org/zap"
)

type Word struct {
	Word        string `json:"word"`
	Level       string `json:"level"`
	Explain     string `json:"explain"`
	FirstLetter string `json:"first_letter"`
}

func (ls *LearnService) NewWords(msg, reqTrace string) (int32, error) {
	const op = "learn.NewWords"

	ls.log.Debug("New words request",
		zap.String("op", op),
		zap.Int("msg len", len(msg)),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := services.CallRPC(ls.cb, func() (*pb.NewWordsRes, error) {
		return ls.client.NewWords(ctx, &pb.NewWordsReq{
			JsonData:     msg,
			RequestTrace: reqTrace,
		})
	})
	if err != nil {
		return 0, fmt.Errorf("%s: rpc call: %w", op, err)
	}

	ls.log.Debug("New words response",
		zap.String("op", op),
		zap.Int32("inserted", res.Inserted),
		zap.String("reqTrace", reqTrace))

	return res.Inserted, nil
}

func (ls *LearnService) GetWords(searchData, reqTrace string, buf *[]Word) error {
	const op = "learn.GetWords"

	ls.log.Debug("Get words request",
		zap.String("op", op),
		zap.Int("searchData len", len(searchData)),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := services.CallRPC(ls.cb, func() (*pb.GetWordsRes, error) {
		return ls.client.GetWords(ctx, &pb.GetWordsReq{
			SearchData:   searchData,
			RequestTrace: reqTrace,
		})
	})
	if err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	dataBytes := unsafe.Slice(unsafe.StringData(res.Data), len(res.Data))
	if err := json.Unmarshal(dataBytes, buf); err != nil {
		return fmt.Errorf("%s: unmarshal words: %w", op, err)
	}

	ls.log.Debug("Get words response",
		zap.String("op", op),
		zap.Int("words len", len(*buf)),
		zap.String("reqTrace", reqTrace))

	return nil
}

func (ls *LearnService) DeleteWord(msg, reqTrace string) error {
	const op = "learn.DeleteWord"

	ls.log.Debug("Delete word request",
		zap.String("op", op),
		zap.Int("msg len", len(msg)),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msgBytes := unsafe.Slice(unsafe.StringData(msg), len(msg))
	word, serial, has := bytes.Cut(msgBytes, []byte(" "))
	if !has {
		return fmt.Errorf("%s: cut message: invalid message structure", op)
	}
	wordStr := unsafe.String(unsafe.SliceData(word), len(word))
	serialStr := unsafe.String(unsafe.SliceData(serial), len(serial))
	serialInt, err := strconv.Atoi(serialStr)
	if err != nil {
		return fmt.Errorf("%s: atoi serial: invalid serial", op)
	}

	if _, err := services.CallRPC(ls.cb, func() (*pb.DelWordRes, error) {
		return ls.client.DelWord(ctx, &pb.DelWordReq{
			Word:         wordStr,
			Serial:       int32(serialInt),
			RequestTrace: reqTrace,
		})
	}); err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	ls.log.Debug("Delete word successfully",
		zap.String("op", op),
		zap.String("reqTrace", reqTrace))

	return nil
}
