package main

import (
	"math/rand"
	"time"
)

type Quiz struct {
	ID                   string
	CreatorID            string
	Participants         []string
	Questions            []*Question
	Complete             bool
	AskingParticipants   bool
	AskingForNewQuestion bool
}

type Question struct {
	Question string
	Answers  map[string]*Answer
}

type Answer struct {
	Asking bool
	Answer string
}

func (q *Question) GetRandomAnswer() (string, string) {
	validKeys := []string{}
	for k, v := range q.Answers {
		if v.Answer == "" {
			continue
		}

		validKeys = append(validKeys, k)
	}

	if len(validKeys) == 0 {
		return "", ""
	}
	rand.Seed(time.Now().Unix())
	randomUser := validKeys[rand.Intn(len(validKeys))]
	randomAnswer := q.Answers[randomUser].Answer
	return randomUser, randomAnswer
}
