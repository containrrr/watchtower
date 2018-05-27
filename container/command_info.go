package container

import (
        "encoding/json"
)

type CommandInfo struct {
	User         string
        Privileged   bool
        Env          []string
        Cmd          []string
}

func ReadCommandInfoFromJSON(commandInfoJSON string) (CommandInfo, error) {
        commandInfo := CommandInfo{}

        err := json.Unmarshal([]byte(commandInfoJSON), &commandInfo)
        if err != nil {
		return CommandInfo{}, err
	}

        return commandInfo, nil
}

func (commandInfo CommandInfo) IsDefined() bool {
        return commandInfo.Cmd != nil && len(commandInfo.Cmd) > 0
}