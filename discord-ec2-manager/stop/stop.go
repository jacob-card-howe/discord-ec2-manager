package stop

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

var (
	instanceSpecified bool
	UserInstanceId    string
	instancesToStop   []string
	input             *ec2.StopInstancesInput
)

type EC2InstanceAPI interface {
	StopInstances(ctx context.Context,
		params *ec2.StopInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
}

func StopInstance(c context.Context, api EC2InstanceAPI, input *ec2.StopInstancesInput) (*ec2.StopInstancesOutput, error) {
	return api.StopInstances(c, input)
}

func StopEc2Instance(messageContentSlice []string, instanceIds []string, client *ec2.Client) (statusMessage string) {
	if len(instanceIds) < 1 {
		log.Println("There are no instances stored in the bot's memory. Use !create to add an instance to the instanceIds slice.")
		statusMessage = "There was an error fetching the status of your EC2 instance, please check the bot's error logs for more information."
		return
	}

	log.Println("Instances in Memory for !status:", instanceIds)

	if len(messageContentSlice) > 1 {
		for i := 1; i < len(messageContentSlice); i += 2 {
			switch messageContentSlice[i] {
			case "-i":
				instanceSpecified = true
				if messageContentSlice[i+1] != "-i" {
					UserInstanceId = messageContentSlice[i+1]
					instancesToStop = append(instancesToStop, UserInstanceId)
				}
			}
		}
	}

	if instanceSpecified {
		input = &ec2.StopInstancesInput{
			InstanceIds: instancesToStop,
		}
	} else {
		input = &ec2.StopInstancesInput{
			InstanceIds: instanceIds,
		}
	}

	_, err := client.StopInstances(context.TODO(), input)
	if err != nil {
		log.Println("Error stopping EC2 instance:", err)
		statusMessage = "**ERROR**: There was an error trying to stop your EC2 instance. Please see your bot's error logs for more information."
		return
	} else {
		statusMessage = "Stopping EC2 instance..."
		return
	}

}
