package main

func main() {

	light := &Light{
		name: "Bedroom light",
	}

	on := &TurnOnCommand{
		light: light,
	}

	off := &TurnOffCommand{
		light: light,
	}

	remote := &Remote{}

	remote.command = on
	remote.PressButton()

	remote.command = off
	remote.PressButton()
}
