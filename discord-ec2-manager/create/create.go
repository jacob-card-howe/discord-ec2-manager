package create

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var (
	// EC2 Specific Variables
	UserInstanceId      string
	UserSecurityGroupId string
	UserAmiId           string
	UserSubnetId        string
	UserPathToScript    string
	UserTagKey          string
	UserTagValue        string
	UserKeyName         string
	UserInstanceType    string

	// IAM Role Variables
	UserIamArn         string
	UserIamProfileName string

	// Service Check (Healthcheck) Variables
	ServiceCheckPort string

	// Service Specific Variables
	UserServiceName string
	UserServicePort string

	// Slice for Security Groups
	SecurityGroupIds []string

	// Slice for Instance IDs
	instanceIds []string

	// Misc.
	validUserSubnetId bool
	runInstancesInput *ec2.RunInstancesInput
)

// EC2InstanceAPI defines the interface for the RunInstances, CreateTags, and TerminateInstances functions.
type EC2InstanceAPI interface {
	RunInstances(ctx context.Context,
		params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)

	CreateTags(ctx context.Context,
		params *ec2.CreateTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
}

// Creates an EC2 instance
func MakeInstance(c context.Context, api EC2InstanceAPI, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	return api.RunInstances(c, input)
}

// Creates tags fo created EC2 instance
func CreateTag(c context.Context, api EC2InstanceAPI, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return api.CreateTags(c, input)
}

func CreateEc2Instance(messageContentSlice []string, flagArray []string, client *ec2.Client) (statusMessage string, UserInstanceId string) {
	log.Println("Checking for required flags...")
	if len(messageContentSlice) > 1 {
		for i := 1; i < len(messageContentSlice); i += 2 {
			if messageContentSlice[i] != "-ami" && messageContentSlice[i] != "-sn" && messageContentSlice[i] != "-sg" {
				log.Printf("%s is not a required flag, skipping...", messageContentSlice[i])
				break
			} else {
				switch messageContentSlice[i] {
				case "-ami":
					for j := 0; j < len(flagArray); j++ {
						if messageContentSlice[i+1] != flagArray[j] {
							log.Println("Custom Amazon Machine Image ID found:", messageContentSlice[i+1])
							UserAmiId = messageContentSlice[i+1]
							break
						}
					}
				case "-sn":
					for j := 0; j < len(flagArray); j++ {
						if messageContentSlice[i+1] != flagArray[j] {
							log.Println("Subnet ID found:", messageContentSlice[i+1])
							UserSubnetId = messageContentSlice[i+1]
							validUserSubnetId = true
							break
						} else {
							log.Println("To use !create you MUST specify a subnet via the -sn flag either when starting the bot, or via your !create Discord Message. Please either restart your bot OR resend your !create Discord Message with the -sn flag and a valid Subnet ID.")
							validUserSubnetId = false
							return
						}
					}
				}

				if UserAmiId == "" {
					UserAmiId = "ami-09e67e426f25ce0d7" // Ubuntu 20.04
				}
			}
		}
	} else if UserSubnetId != "" && UserAmiId != "" {
		log.Println("Subnet ID and AMI ID were either previously set, or set via the bot's environment variables.")
		validUserSubnetId = true
	} else {
		log.Println("Missing parameters!")
		return
	}

	if validUserSubnetId {
		for i := 1; i < len(messageContentSlice); i += 2 {
			switch messageContentSlice[i] {
			case "-sg": // Security Group Flag
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserSecurityGroupId = messageContentSlice[i+1]
					} else {
						log.Println("Invalid Security Group ID:", messageContentSlice[i+1])
						statusMessage = fmt.Sprintf("Invalid Security Group ID: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-tk": // Tag Key Flag
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserTagKey = messageContentSlice[i+1]
					} else {
						// TODO: Build in logic to determine invalid key based on AWS standards
						log.Println("Invalid Tag Key:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Tag Key: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-tv": // Tag Value Flag
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserTagValue = messageContentSlice[i+1]
					} else {
						// TODO: Build in logic to determine invalid key values based on AWS standards
						log.Println("Invalid Tag Value", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Tag Value: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-u": // User Data Path Flag
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserPathToScript = messageContentSlice[i+1]
					} else {
						log.Println("Invalid Path to User Data Script:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Path to User Data Script: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-svc": // Service Name Flag
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserServiceName = messageContentSlice[i+1]
					} else {
						log.Println("Invalid Service Name:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Service Name: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-sp": // Service Port Flag
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserServicePort = messageContentSlice[i+1]
						break
					} else {
						log.Println("Invalid Service Port:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Service Port: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-scp": // Service Check Port Flag (Healthcheck)
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						ServiceCheckPort = messageContentSlice[i+1]
					} else {
						log.Println("Invalid Service Check Port:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Service Check Port: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-ia": // EC2 Instance Role ARN
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserIamArn = messageContentSlice[i+1]
					} else {
						log.Println("Invalid IAM ARN:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid IAM ARN: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-in": // EC2 Instance Role ARN
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserIamProfileName = messageContentSlice[i+1]
					} else {
						log.Println("Invalid IAM Name:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid IAM Name: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-k": // EC2 Instance Key Pair Name
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserKeyName = messageContentSlice[i+1]
					} else {
						log.Println("Invalid Key Pair Name:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Key Pair Name: %s", messageContentSlice[i+1])
						return
					}
				}
			case "-it": // EC2 Instance Key Pair Name
				for j := 0; j < len(flagArray); j++ {
					if messageContentSlice[i+1] != flagArray[j] {
						UserInstanceType = messageContentSlice[i+1]
					} else {
						log.Println("Invalid Instance Type:", messageContentSlice[i+1])

						statusMessage = fmt.Sprintf("Invalid Instance Type: %s", messageContentSlice[i+1])
						return
					}
				}
			}
		}

		if UserIamArn != "" && UserIamProfileName != "" {
			log.Println("Error, cannot use -in and -ia flags together. Please run the !create command again with only one flag specified.")
			statusMessage = "Error, cannot use -in and -ia flags together. Please run the `!create` command again with only one flag specified."
			return
		}

		content, err := ioutil.ReadFile(UserPathToScript)
		if err != nil {
			log.Println("Error reading from file, instance will launch without userdata.sh:", err)
		}

		encodedUserDataContent := base64.StdEncoding.EncodeToString([]byte(content))

		if UserKeyName == "" {
			runInstancesInput = &ec2.RunInstancesInput{
				ImageId:          aws.String(UserAmiId),
				InstanceType:     types.InstanceType(UserInstanceType),
				MinCount:         aws.Int32(1),
				MaxCount:         aws.Int32(1),
				SecurityGroupIds: SecurityGroupIds,
				SubnetId:         aws.String(UserSubnetId),
				UserData:         aws.String(encodedUserDataContent),
				IamInstanceProfile: &types.IamInstanceProfileSpecification{
					Arn:  aws.String(UserIamArn),
					Name: aws.String(UserIamProfileName),
				},
			}
		} else {
			runInstancesInput = &ec2.RunInstancesInput{
				ImageId:          aws.String(UserAmiId),
				InstanceType:     types.InstanceType(UserInstanceType),
				MinCount:         aws.Int32(1),
				MaxCount:         aws.Int32(1),
				SecurityGroupIds: SecurityGroupIds,
				SubnetId:         aws.String(UserSubnetId),
				UserData:         aws.String(encodedUserDataContent),
				IamInstanceProfile: &types.IamInstanceProfileSpecification{
					Arn:  aws.String(UserIamArn),
					Name: aws.String(UserIamProfileName),
				},
				KeyName: aws.String(UserKeyName),
			}
		}

		result, err := MakeInstance(context.TODO(), client, runInstancesInput)
		if err != nil {
			fmt.Println("Error creating EC2 instance:", err)
			statusMessage = fmt.Sprint("There was an error creating your EC2 instance:", err)
			return
		}

		UserInstanceId = *result.Instances[0].InstanceId

		if result.Instances[0].InstanceId != nil {
			log.Println("OTP entered correctly, instance created:", UserInstanceId)
			statusMessage = fmt.Sprintf("One time password entered correctly, your EC2 instance has been created!\nInstance ID: `%s`", UserInstanceId)
			instanceIds = append(instanceIds, UserInstanceId)
		}

		tagInput := &ec2.CreateTagsInput{
			Resources: []string{UserInstanceId},
			Tags: []types.Tag{
				{
					Key:   aws.String(UserTagKey),
					Value: aws.String(UserTagValue),
				},
			},
		}

		_, err = CreateTag(context.TODO(), client, tagInput)
		if err != nil {
			log.Println("Error tagging resources:", err)
			return
		}

	} else {
		return
	}

	return
}
