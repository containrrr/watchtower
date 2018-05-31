package container

import (
        "encoding/json"
)

// CommandInfo is the data structure used to encode information about a
// pre/post-update command.
type CommandInfo struct {
	User         string
        Privileged   bool
        Env          []string
        Cmd          []string
}

// ReadCommandInfoFromJSON takes a JSON formatted description of a
// pre/post-update as input and returns the parsed data as a CommandInfo.
func ReadCommandInfoFromJSON(commandInfoJSON string) (CommandInfo, error) {
        commandInfo := CommandInfo{}

        err := json.Unmarshal([]byte(commandInfoJSON), &commandInfo)
        if err != nil {
		return CommandInfo{}, err
	}

        return commandInfo, nil
}

// IsDefined returns true if a CommandInfo actually contains a command to
// execute or false otherwise.
func (commandInfo CommandInfo) IsDefined() bool {
        return commandInfo.Cmd != nil && len(commandInfo.Cmd) > 0
}