package status

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

var (
	// EC2 Instance Stuff
	UserInstanceId string
	UserTagKey     string
	UserTagValue   string

	ServiceCheckPort string

	UserServiceName string
	UserServicePort string

	instanceSpecified      bool
	instancesToCheckStatus []string

	input *ec2.DescribeInstancesInput
)

type EC2InstanceAPI interface {
	DescribeInstances(ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// Creates an EC2 instance
func GetInstances(c context.Context, api EC2InstanceAPI, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return api.DescribeInstances(c, input)
}

func GetEc2InstanceStatus(messageContentSlice []string, instanceIds []string, UserTagKey string, UserTagValue string, ServiceCheckPort string, UserServiceName string, UserServicePort string, client *ec2.Client) (statusMessage string) {

	if len(instanceIds) < 1 {
		log.Println("There are no instances stored in the bot's memory. Use !create to add an instance to the instanceIds slice.")
		statusMessage = "There was an error fetching the status of your EC2 instance, please check the bot's error logs for more information."
		return
	}

	log.Println("Instances in Memory:", instanceIds)

	if len(messageContentSlice) > 1 {
		for i := 1; i < len(messageContentSlice); i += 2 {
			switch messageContentSlice[i] {
			case "-i":
				instanceSpecified = true
				if messageContentSlice[i+1] != "-i" {
					UserInstanceId = messageContentSlice[i+1]
					instancesToCheckStatus = append(instancesToCheckStatus, UserInstanceId)
				}
			}
		}
	}

	if instanceSpecified {
		log.Println("Instance flag was specified, setting InstanceIds to instancesToCheckStatus:", instancesToCheckStatus)
		input = &ec2.DescribeInstancesInput{
			InstanceIds: instancesToCheckStatus,
		}
	} else {
		log.Println("Instance flag was NOT specified, setting InstanceIds to instanceIds:", instanceIds)
		input = &ec2.DescribeInstancesInput{
			InstanceIds: instanceIds,
		}
	}

	log.Printf("Getting status using %v as input", input.InstanceIds)
	status, err := GetInstances(context.TODO(), client, input)

	if err != nil {
		log.Println("Error getting status:", err)
		statusMessage = "There was an error fetching the status of your EC2 instance, please check the bot's error logs for more information."
		return
	}

	for _, r := range status.Reservations {
		for _, i := range r.Instances {
			instanceState := fmt.Sprintf("%v", *i.State)
			if strings.Contains(instanceState, "running") {
				if ServiceCheckPort == "" {
					statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`", *i.InstanceId, *i.PublicIpAddress, *i.State)
					return
				} else {
					instanceIpAndPort := fmt.Sprint("http://", *i.PublicIpAddress, ":", ServiceCheckPort)

					resp, err := http.Get(instanceIpAndPort)
					if err != nil {
						log.Println("Error sending GET request to instance: ", err)
						statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`\n`%s` status cannot be checked right now. See your bot's error logs for more information.", *i.InstanceId, *i.PublicIpAddress, *i.State, UserServiceName, "inactive", UserServicePort)
						return
					}

					respStatus := string(resp.Status)

					if strings.Contains(respStatus, "200") {
						statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`\n`%s` is currently `%s` on port `%s`", *i.InstanceId, *i.PublicIpAddress, *i.State, UserServiceName, "active", UserServicePort)
						return
					} else {
						statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`\n`%s` is currently `%s` on port `%s`", *i.InstanceId, *i.PublicIpAddress, *i.State, UserServiceName, "inactive", UserServicePort)
						return
					}
				}

			} else {
				statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance State: `%v`", *i.InstanceId, *i.State)
				return
			}
		}
	}
	log.Println(statusMessage)
	return
}
