package main

type Command interface {
	Execute()
	Undo()
}

type RemoteControl struct {
	command Command
}

func (r *RemoteControl) SetCommand(
	cmd Command,
) {
	r.command = cmd
}

func (r *RemoteControl) PressButton() {
	r.command.Execute()
}

func (r *RemoteControl) PressUndo() {

	if r.command != nil {
		r.command.Undo()
	}
}
