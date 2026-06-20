package main

type Command interface {
	Execute()
}

type Remote struct {
	command Command
}

func (r *Remote) PressButton() {
	r.command.Execute()
}
