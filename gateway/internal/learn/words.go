// Package learn words.go calls learn-service grpc methods
// responsible for words gprc methods
package learn

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"
	"unsafe"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
	tele "gopkg.in/telebot.v3"
)

func (ls *LearnService) NewWords(tctx tele.Context) error {
	const op = "learn.NewWords"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	jsonData := tctx.Message().Text

	res, err := ls.client.NewWords(ctx, &pb.NewWordsReq{
		JsonData: jsonData,
	})
	if err != nil {
		return fmt.Errorf("%s: new word: %w", op, err)
	}

	return tctx.Send(fmt.Sprintf("Insterted: %d", res.Inserted))
}

func (ls *LearnService) GetWords(tctx tele.Context) error {
	const op = "learn.GetWords"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	searchData := tctx.Message().Text

	res, err := ls.client.GetWord(ctx, &pb.GetWordReq{
		SearchData: searchData,
	})
	if err != nil {
		return fmt.Errorf("%s: get words: %w", op, err)
	}

	return tctx.Send(fmt.Sprintf("Search data: %s\nWords: %v", searchData, res.Data))
}

func (ls *LearnService) DeleteWords(tctx tele.Context) error {
	const op = "learn.DeleteWords"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg := tctx.Message().Text
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

	if _, err := ls.client.DelWord(ctx, &pb.DelWordReq{
		Word:   wordStr,
		Serial: int32(serialInt),
	}); err != nil {
		return fmt.Errorf("%s: delete word: %w", op, err)
	}

	return tctx.Send(fmt.Sprintf("Deleted word: %s\nDeleted serial: %d", wordStr, serialInt))
}
