package discordCommand

// * use static types for ensure type safety and avoid typos
type CommandType int

const (
	CmdHelp CommandType = iota
	CmdRole
	CmdAddGemini
	CmdAddOpenAI
	CmdAddClaude
	CmdAddNim
)

var commands = []CommandType{
	CmdHelp,
	CmdRole,
	CmdAddGemini,
	CmdAddOpenAI,
	CmdAddClaude,
	CmdAddNim,
}

func (c CommandType) Text() string {
	switch c {
	case CmdHelp:
		return "help"
	case CmdRole:
		return "role"
	case CmdAddGemini:
		return "add-gemini"
	case CmdAddOpenAI:
		return "add-openai"
	case CmdAddClaude:
		return "add-claude"
	case CmdAddNim:
		return "add-nim"
	default:
		return ""
	}
}

func getCmd(cmd string) CommandType {
	switch cmd {
	case "help", "/help":
		return CmdHelp
	case "role", "/role":
		return CmdRole
	case "add-gemini", "/add-gemini":
		return CmdAddGemini
	case "add-openai", "/add-openai":
		return CmdAddOpenAI
	case "add-claude", "/add-claude":
		return CmdAddClaude
	case "add-nim", "/add-nim":
		return CmdAddNim
	default:
		return -1
	}
}

func IsAddKeyCmd(name string) bool {
	switch getCmd(name) {
	case CmdAddGemini, CmdAddOpenAI, CmdAddClaude, CmdAddNim:
		return true
	}
	return false
}
