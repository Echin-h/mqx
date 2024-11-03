package consumer

import (
	"fmt"

	"mqx/queue"
)

func ProcessMsg(msg queue.Message) bool {
	switch msg.FuncName {
	case queue.MsgHelloWorld:
		{
			userId, ok := msg.Data["UserId"]
			if !ok {
				return false
			}
			kbId, ok := msg.Data["KbId"]
			if !ok {
				return false
			}
			fmt.Println("recieved message:", userId, kbId)
			// add function here:  such as proficiency.CalculateProficiency(userId.(string), kbId.(string))
			err := helloworld(userId.(string), kbId.(string))
			if err != nil {
				fmt.Println("Calculate proficiency error: ", err)
				return false
			}
		}
	default:
		fmt.Println("Unknown function name: ", msg.FuncName)
	}

	return true
}

func helloworld(userId string, kbId string) error {
	fmt.Println("Hello World from ", userId, kbId)
	return nil
}
