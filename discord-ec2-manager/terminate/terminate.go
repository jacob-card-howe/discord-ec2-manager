package terminate

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

var (
	instanceIds    []string
	UserInstanceId string
)

type EC2InstanceAPI interface {
	TerminateInstances(ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
}

// Terminates an EC2 instance
func TerminateInstance(c context.Context, api EC2InstanceAPI, input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return api.TerminateInstances(c, input)
}

func TerminateEc2Instance(messageContentSlice []string, UserInstanceId string, client *ec2.Client) (statusMessage string) {
	if len(messageContentSlice) > 1 {
		for i := 1; i < len(messageContentSlice); i += 2 {
			switch messageContentSlice[i] {
			case "-i":
				if messageContentSlice[i+1] != "-i" {
					UserInstanceId = messageContentSlice[i+1]
					instanceIds = append(instanceIds, UserInstanceId)
					break
				}
			}
		}
	} else {
		instanceIds = append(instanceIds, UserInstanceId)
	}

	input := &ec2.TerminateInstancesInput{
		InstanceIds: instanceIds,
	}

	_, err := TerminateInstance(context.TODO(), client, input)
	if err != nil {
		log.Println("Error terminating instance:", err)
		statusMessage = "There was an error terminating your EC2 instance, please see the console logs for more info."
		return
	}

	log.Println("OTP entered correctly, terminating EC2 instance")
	statusMessage = "One time password entered correctly, terminating EC2 instance."
	return
}
