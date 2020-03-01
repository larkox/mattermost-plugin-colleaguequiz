package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type AskType int

const (
	AskForParticipants AskType = iota
	AskForNewQuestion
	AskForAnswer
)

func quizzesKey() string {
	return "quizzes"
}

func (p *Plugin) AddQuiz(name string, creatorID string) error {
	quizzes, originalJSON := p.GetAllQuizzes()
	if _, ok := quizzes[name]; ok {
		return fmt.Errorf("quiz named %s already exist", name)
	}

	quizzes[name] = &Quiz{
		ID:                   name,
		CreatorID:            creatorID,
		Participants:         []string{},
		Questions:            []*Question{},
		Complete:             false,
		AskingParticipants:   false,
		AskingForNewQuestion: false,
	}
	return p.SaveAllQuizzes(quizzes, originalJSON)
}

func (p *Plugin) GetAllQuizzes() (map[string]*Quiz, []byte) {
	originalJSON, appErr := p.API.KVGet(quizzesKey())
	if appErr != nil {
		return make(map[string]*Quiz), nil
	}

	var quizzes map[string]*Quiz
	jsonErr := json.Unmarshal(originalJSON, &quizzes)
	if jsonErr != nil {
		return make(map[string]*Quiz), nil
	}

	return quizzes, originalJSON
}

func (p *Plugin) SaveAllQuizzes(quizzes map[string]*Quiz, originalJSON []byte) error {
	quizzesJSON, jsonError := json.Marshal(quizzes)
	if jsonError != nil {
		return jsonError
	}
	ok, err := p.API.KVCompareAndSet(quizzesKey(), originalJSON, quizzesJSON)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("old value is not the same")
	}

	return nil
}

func (p *Plugin) IsUserAvailable(userID string) bool {
	quizzes, _ := p.GetAllQuizzes()
	return p.isUserAvailable(userID, quizzes)
}

func (p *Plugin) isUserAvailable(userID string, quizzes map[string]*Quiz) bool {
	for _, v := range quizzes {
		if userID == v.CreatorID {
			if v.AskingParticipants {
				return false
			}
			if v.AskingForNewQuestion {
				return false
			}
		}
		for _, q := range v.Questions {
			if q.Answers[userID] != nil {
				if q.Answers[userID].Asking {
					return false
				}
			}
		}
	}
	return true
}

func (p *Plugin) HasPendingAnswer(userID string) (bool, AskType, string, int) {
	quizzes, _ := p.GetAllQuizzes()
	for _, v := range quizzes {
		if userID == v.CreatorID {
			if v.AskingParticipants {
				return true, AskForParticipants, v.ID, 0
			}
			if v.AskingForNewQuestion {
				return true, AskForNewQuestion, v.ID, 0
			}
		}
		for i, q := range v.Questions {
			if q.Answers[userID] != nil {
				if q.Answers[userID].Asking {
					return true, AskForAnswer, v.ID, i
				}
			}
		}
	}
	return false, 0, "", 0
}

func (p *Plugin) GetQuestionToAsk() (string, string) {
	quizzes, original := p.GetAllQuizzes()
	for _, v := range quizzes {
		if !v.Complete && p.isUserAvailable(v.CreatorID, quizzes) {
			if len(v.Participants) > 0 {
				err := p.SetAskForNewQuestion(quizzes, v.ID, original)
				if err != nil {
					return "", ""
				}
				return v.CreatorID, fmt.Sprintf("Type a new question for the quiz `%s` or type `end` to finish adding questions.", v.ID)
			}

			err := p.SetAskParticipants(quizzes, v.ID, original)
			if err != nil {
				return "", ""
			}
			return v.CreatorID, fmt.Sprintf("Type all the participants for quiz %s.", v.ID)
		}

		if !v.Complete {
			continue
		}

		for _, userID := range v.Participants {
			if !p.isUserAvailable(userID, quizzes) {
				continue
			}
			for i, q := range v.Questions {
				if _, ok := q.Answers[userID]; !ok {
					err := p.SetAskForAnswer(quizzes, v.ID, i, userID, original)
					if err != nil {
						return "", ""
					}
					return userID, fmt.Sprintf("Answer this question: %s", q.Question)
				}
			}
		}
	}
	return "", ""
}

func (p *Plugin) SetAskForNewQuestion(quizzes map[string]*Quiz, quizID string, originalJSON []byte) error {
	quizzes[quizID].AskingForNewQuestion = true
	return p.SaveAllQuizzes(quizzes, originalJSON)
}

func (p *Plugin) SetAskParticipants(quizzes map[string]*Quiz, quizID string, originalJSON []byte) error {
	quizzes[quizID].AskingParticipants = true
	return p.SaveAllQuizzes(quizzes, originalJSON)
}

func (p *Plugin) SetAskForAnswer(quizzes map[string]*Quiz, quizID string, i int, userID string, originalJSON []byte) error {
	quizzes[quizID].Questions[i].Answers[userID] = &Answer{Asking: true}
	return p.SaveAllQuizzes(quizzes, originalJSON)
}

func (p *Plugin) AddAnswer(userID string, quizID string, questionIndex int, answer string) error {
	quizzes, originalJSON := p.GetAllQuizzes()
	quizzes[quizID].Questions[questionIndex].Answers[userID].Answer = answer
	quizzes[quizID].Questions[questionIndex].Answers[userID].Asking = false
	return p.SaveAllQuizzes(quizzes, originalJSON)
}

func (p *Plugin) CompleteQuiz(quizID string) error {
	quizzes, originalJSON := p.GetAllQuizzes()
	quizzes[quizID].Complete = true
	quizzes[quizID].AskingForNewQuestion = false
	return p.SaveAllQuizzes(quizzes, originalJSON)
}

func (p *Plugin) AddQuestion(quizID string, question string) error {
	quizzes, originalJSON := p.GetAllQuizzes()
	quizzes[quizID].Questions = append(quizzes[quizID].Questions, &Question{
		Question: question,
		Answers:  make(map[string]*Answer),
	})
	quizzes[quizID].AskingForNewQuestion = false
	return p.SaveAllQuizzes(quizzes, originalJSON)
}

func (p *Plugin) GetQuiz(quizID string) (*Quiz, error) {
	quizzes, _ := p.GetAllQuizzes()
	q, ok := quizzes[quizID]
	if !ok {
		return nil, fmt.Errorf("Cannot find quiz %s.", quizID)
	}
	return q, nil
}

func (p *Plugin) AddParticipants(userID, quizID, in string) error {
	participantsUserNames := strings.Split(strings.TrimSpace(in), " ")
	participantsIDs := []string{}
	for _, v := range participantsUserNames {
		userName := v
		if v[0] == '@' {
			userName = v[1:]
		}
		user, appError := p.API.GetUserByUsername(userName)
		if appError != nil {
			p.PostBotDM(userID, fmt.Sprintf("Could not find user `%s`. It will not be added to the quiz. Err=%s.", userName, appError.Error()))
			continue
		}
		participantsIDs = append(participantsIDs, user.Id)
	}

	if len(participantsIDs) == 0 {
		p.PostBotDM(userID, "You should add at least one valid participant. Please, enter the participant list again.")
		return nil
	}

	quizzes, originalJSON := p.GetAllQuizzes()
	quizzes[quizID].Participants = participantsIDs
	quizzes[quizID].AskingParticipants = false
	return p.SaveAllQuizzes(quizzes, originalJSON)
}
