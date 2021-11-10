package start

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

var (
	instanceSpecified bool
	instancesToStart  []string

	input *ec2.StartInstancesInput
)

type EC2InstanceAPI interface {
	StartEc2Instance(ctx context.Context,
		params *ec2.StartInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
}

// Creates an EC2 instance
func StartInstances(c context.Context, api EC2InstanceAPI, input *ec2.StartInstancesInput) (*ec2.StartInstancesOutput, error) {
	return api.StartEc2Instance(c, input)
}

func StartEc2Instance(messageContentSlice []string, instanceIds []string, client *ec2.Client) (statusMessage string, UserInstanceId string) {
	if len(instanceIds) < 1 {
		log.Println("There are no instances stored in the bot's memory. Use !create to add an instance to the instanceIds slice.")
		statusMessage = "There was an error fetching the status of your EC2 instance, please check the bot's error logs for more information."
		return
	}

	log.Println("Instances in Memory for !start:", instanceIds)

	if len(messageContentSlice) > 1 {
		for i := 1; i < len(messageContentSlice); i += 2 {
			switch messageContentSlice[i] {
			case "-i":
				instanceSpecified = true
				if messageContentSlice[i+1] != "-i" {
					UserInstanceId = messageContentSlice[i+1]
					instancesToStart = append(instancesToStart, UserInstanceId)
				}
			}
		}
	}

	if instanceSpecified {
		input = &ec2.StartInstancesInput{
			InstanceIds: instancesToStart,
		}
	} else {
		input = &ec2.StartInstancesInput{
			InstanceIds: instanceIds,
		}
	}

	_, err := client.StartInstances(context.TODO(), input)
	if err != nil {
		log.Println("Error starting EC2 instance:", err)
		statusMessage = "**ERROR**: There was an error trying to start your EC2 instance. Please see your bot's error logs for more information."
		return
	} else {
		statusMessage = "Starting EC2 instance...\nUse **`!status`** to track the status of your server as it comes online!"
		return
	}
}
