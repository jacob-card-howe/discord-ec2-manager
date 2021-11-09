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
	UserTagKey string
	UserTagValue string

	ServiceCheckPort string

	UserServiceName string
	UserServicePort string

	instanceIds []string
)

func GetEc2InstanceStatus(instanceIds []string, UserTagKey string, UserTagValue string, ServiceCheckPort string, UserServiceName string, UserServicePort string, client *ec2.Client) (statusMessage string) {

	log.Println(instanceIds)

	if len(instanceIds) < 1 {
		log.Println("There are no instances stored in the bot's memory. Use !create to add an instance to the instanceIds slice.")
		statusMessage = "There was an error fetching the status of your EC2 instance, please check the bot's error logs for more information."
		return
	}

	status, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	})

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
					log.Println("===== LINE 78 =====")
					statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`", *i.InstanceId, *i.PublicIpAddress, *i.State)
					return
				} else {
					log.Println("===== LINE 82 =====")
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
				log.Println("===== LINE 104 =====")
				statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance State: `%v`", *i.InstanceId, *i.State)
				return
			}
		}
	}
	log.Println(statusMessage)
	return
}