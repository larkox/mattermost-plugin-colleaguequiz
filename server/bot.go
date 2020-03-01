package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

const (
	BotSleepTime = 5 * time.Second
)

var botDone = make(chan struct{})

func finishBotRoutine() bool {
	select {
	case <-botDone:
		return true
	default:
		return false
	}
}

// PostBotDM posts a DM as the cloud bot user.
func (p *Plugin) PostBotDM(userID string, message string) error {
	channel, appError := p.API.GetDirectChannel(userID, p.botUserID)
	if appError != nil {
		return appError
	}
	if channel == nil {
		return fmt.Errorf("could not get direct channel for bot and user_id=%s", userID)
	}

	_, appError = p.API.CreatePost(&model.Post{
		UserId:    p.botUserID,
		ChannelId: channel.Id,
		Message:   message,
	})

	return appError
}

func (p *Plugin) PostBotToChannel(channelID, message string) error {
	_, appError := p.API.CreatePost(&model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   message,
	})

	return appError
}

// MessageHasBeenPosted checks if the message is a DM from an user, and process the message if it is answering a question
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	if p.botUserID == post.UserId {
		return
	}

	ch, appErr := p.API.GetDirectChannel(p.botUserID, post.UserId)
	if appErr != nil {
		p.API.LogError("error getting direct channel: " + appErr.Error())
		return
	}

	if ch.Id != post.ChannelId {
		return
	}

	ok, answerType, quizID, questionIndex := p.HasPendingAnswer(post.UserId)

	if !ok {
		p.PostBotDM(post.UserId, "Thank you for messaging me, but I am a bot. I will get back to you whenever I have questions for you.")
		return
	}

	switch answerType {
	case AskForAnswer:
		err := p.AddAnswer(post.UserId, quizID, questionIndex, post.Message)
		if err != nil {
			p.PostBotDM(post.UserId, "There has been an internal error. Please, answer again.")
			return
		}
		p.PostBotDM(post.UserId, "Thank you for your answer!")
		return
	case AskForNewQuestion:
		if post.Message == "end" {
			err := p.CompleteQuiz(quizID)
			if err != nil {
				p.PostBotDM(post.UserId, "There has been an internal error while completing the quiz. Please, type \"end\" again.")
				return
			}
			p.PostBotDM(post.UserId, "Quiz completed. Now the participants will start to receive the questions to answer.")
			return
		}

		err := p.AddQuestion(quizID, post.Message)
		if err != nil {
			p.PostBotDM(post.UserId, "There has been an internal error while adding the question. Please, write the question again.")
			return
		}
		p.PostBotDM(post.UserId, "Question added!")
		return
	case AskForParticipants:
		err := p.AddParticipants(post.UserId, quizID, post.Message)
		if err != nil {
			p.PostBotDM(post.UserId, "There has been an internal error while adding the participants. Please, write the participant list again.")
		}
		p.PostBotDM(post.UserId, "Participants added!")
		return
	}
}

func (p *Plugin) botRoutine() {
	for {
		if finishBotRoutine() {
			return
		}
		userID, question := p.GetQuestionToAsk()
		for userID != "" {
			p.PostBotDM(userID, question)
			userID, question = p.GetQuestionToAsk()
		}
		time.Sleep(BotSleepTime)
	}
}

func (p *Plugin) quizRoutine(quiz *Quiz, channelID string) {
	p.PostBotToChannel(channelID, fmt.Sprintf("Welcome to Colleague Quiz. Today we have the quiz `%s`.", quiz.ID))
	for _, v := range quiz.Questions {
		if finishBotRoutine() {
			p.PostBotToChannel(channelID, "Quiz game has been cancelled.")
			return
		}
		user, answer := v.GetRandomAnswer()
		if user == "" {
			continue
		}
		p.PostBotToChannel(channelID, fmt.Sprintf("What did %s answer to `%s`? The answer will be given in 10 seconds!", p.GetUserName(user), v.Question))
		time.Sleep(10 * time.Second)
		p.PostBotToChannel(channelID, fmt.Sprintf("The answer is `%s`.", answer))
		time.Sleep(2 * time.Second)
	}
	p.PostBotToChannel(channelID, fmt.Sprintf("That's all for today. Thanks!"))
}
