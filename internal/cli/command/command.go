package command

type Command interface {
	Run()
}

var commands = map[string]Command{
	"run": &Run{},
}

func New(name string) Command {
	if _, ok := commands[name]; !ok {
		panic("Invalid command :: " + name + " ::")
	}
	return commands[name]
}
