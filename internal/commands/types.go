package commands

type CommandType string

const (
	PostTransaction CommandType = "POST_TRANSACTION"
)

type Entry struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
	Type      string `json:"type"`
}

type Payload struct {
	Reference string  `json:"reference"`
	Entries   []Entry `json:"entries"`
}
type Command struct {
	Type    CommandType `json:"command_type"`
	Payload Payload     `json:"payload"`
}

func NewCommand(cmdType CommandType, payload Payload) Command {
	return Command{
		Type:    cmdType,
		Payload: payload,
	}
}
