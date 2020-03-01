package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

func getHelp() string {
	return `Available Commands:

create [quizName]
	Creates a new Quiz. quizName must be unique.

	example: /cquiz MyAwesomeQuiz

list
	Lists all quiz.

start [quizName] [channelName]
	Starts a quiz on certain channel with the answers fetched so far

	example: /cquiz MyAwesomeQuiz Town-Square

help
	Display usage.
`
}

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "cquiz",
		DisplayName:      "Colleague Quiz",
		Description:      "Create quiz games with your colleagues.",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: add, list, start",
		AutoCompleteHint: "[command]",
	}
}

func getCommandResponse(responseType, text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: responseType,
		Text:         text,
		Username:     "Colleague Quiz",
		//IconURL:      fmt.Sprintf("/plugins/%s/profile.png", manifest.ID),
	}
}

// ExecuteCommand executes any command to mattermud
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	stringArgs := strings.Split(strings.TrimSpace(args.Command), " ")
	lengthOfArgs := len(stringArgs)
	restOfArgs := []string{}

	var handler func([]string, *model.CommandArgs) (*model.CommandResponse, bool, error)
	if lengthOfArgs == 1 {
		handler = p.runListCommand
	} else {
		command := stringArgs[1]
		if lengthOfArgs > 2 {
			restOfArgs = stringArgs[2:]
		}
		switch command {
		case "add":
			handler = p.runAddCommand
		case "list":
			handler = p.runListCommand
		case "start":
			handler = p.runStartCommand
		default:
			return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, getHelp()), nil
		}
	}

	resp, isUserError, err := handler(restOfArgs, args)
	if err != nil {
		if isUserError {
			return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, fmt.Sprintf("__Error: %s__\n\nRun `/cquiz help` for usage instructions.", err.Error())), nil
		}
		p.API.LogError(err.Error())
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "An unknown error occurred. Please talk to your system administrator for help."), nil
	}

	return resp, nil
}

func (p *Plugin) runListCommand(args []string, extra *model.CommandArgs) (*model.CommandResponse, bool, error) {
	responseMessage := "No quizzes created."
	if len(p.quizzes) > 0 {
		responseMessage = "Quizzes list:\n"
		for k := range p.quizzes {
			responseMessage = fmt.Sprintf("%s\n* %s", responseMessage, k)
		}
	}
	return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, responseMessage), false, nil
}

func (p *Plugin) runAddCommand(args []string, extra *model.CommandArgs) (*model.CommandResponse, bool, error) {
	if len(args) == 0 {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "You must provide a quiz name"), true, nil
	}

	if len(args) > 1 {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Quiz name must have no spaces"), true, nil
	}

	if err := p.AddQuiz(args[0], extra.UserId); err != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, err.Error()), true, nil
	}

	if !p.IsUserAvailable(extra.UserId) {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Quiz creation started. You seem to be in the middle of a conversation with the bot, so it will contact you when you finish answering his questions."), false, nil
	}

	return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Quiz creation started. The bot will contact you to fill up the quiz."), false, nil
}

func (p *Plugin) runStartCommand(args []string, extra *model.CommandArgs) (*model.CommandResponse, bool, error) {
	if len(args) == 0 || len(args) == 1 {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "You must provide a quiz name and a channel name"), true, nil
	}

	if len(args) > 2 {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "`start` only receives 2 arguments"), true, nil
	}

	quiz, err := p.GetQuiz(args[0])
	if err != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Quiz does not exist."), false, nil
	}

	channelName := args[1]
	if channelName[0] == '~' {
		channelName = channelName[1:]
	}

	channel, appErr := p.API.GetChannelByName(extra.TeamId, channelName, false)
	if appErr != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Channel does not exist."), false, nil
	}

	go p.quizRoutine(quiz, channel.Id)
	return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Quiz started on channel."), false, nil
}
