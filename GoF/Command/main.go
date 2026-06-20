package main

// idea: Đóng gói một request (yêu cầu/thao tác) thành một object (TurnOn func => LightOnCommand)
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

	// add các request (đã chuyển thành command) vào một list các hành động cần thực hiện
	queue := []Command{}

	queue = append(queue, turnOn)
	queue = append(queue, turnOff)

	for _, cmd := range queue {
		cmd.Execute()
	}
}
