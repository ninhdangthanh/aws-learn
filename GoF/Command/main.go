package main

func main() {

	// Receiver
	light := &Light{
		name: "Bedroom Light",
	}

	// Command
	turnOn :=
		&LightOnCommand{
			light: light,
		}

	turnOff :=
		&LightOffCommand{
			light: light,
		}

	// Invoker
	remote := &RemoteControl{}

	remote.SetCommand(turnOn)

	remote.PressButton()
	// Bedroom Light is ON

	remote.PressUndo()
	// Bedroom Light is OFF

	remote.SetCommand(turnOff)

	remote.PressButton()
	// Bedroom Light is OFF

	remote.PressUndo()
	// Bedroom Light is ON

	remote.SetCommand(
		&FanStartCommand{
			fan: &Fan{speed: 18},
		},
	)

	remote.PressButton()
	remote.PressUndo()
}
