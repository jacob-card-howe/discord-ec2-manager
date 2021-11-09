package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/bwmarrin/discordgo"
)

// Used alongside GenerateOTP to create a 6 digit psuedorandom OTP to block !create / !terminate from being used by non-admins
const otpChars = "1234567890"

// Used to accept CLI Parameters
var (

	// Discord Bot Variables
	Token     string
	ChannelId string

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

	// Service Specific Variables
	UserServiceName string
	UserServicePort string

	// IAM Role Variables
	UserIamArn         string
	UserIamProfileName string

	// Service Check (Healthcheck) Variables
	ServiceCheckPort string

	// Allows for custom OTP lengths
	OTPLength int
)

// Run Instances Input for Key Pair Name Check
var runInstancesInput *ec2.RunInstancesInput

// One Time Password
var oneTimePassword string

// Status Message
var statusMessage string

// Instance ID
var instanceIds []string

// SG IDs
var SecurityGroupIds []string

// Used to keep track of recent messages in the bot's discord channel
var previousDiscordMessages []string

var validUserSubnetId bool

// Initializes the Discord Part of the App for DiscordGo module
func init() {
	// Discord Bot stuff if you have an existing EC2 instance
	flag.StringVar(&Token, "t", "", "Your Bot's Token (required).")
	flag.StringVar(&ChannelId, "c", "", "Your Discord Channel ID that you want messages to post in (required).")

	// Optional, but needed for !start, !stop and !status unless you're using !create to build a new EC2 instance
	flag.StringVar(&UserInstanceId, "i", "", "The EC2 Instance ID you want to control via !status, !start, and !stop via your Discord server (optional).")

	// Stuff for !create
	flag.StringVar(&UserSecurityGroupId, "sg", "", "The Security Group ID you want to attach to your EC2 instance on !create commands (optional).")
	flag.StringVar(&UserAmiId, "a", "", "The AMI ID you want to launch your EC2 instance with on !create (optional).")
	flag.StringVar(&UserSubnetId, "sn", "", "The Subnet ID you want to launch your EC2 instance with on !create (required if using !create).")
	flag.StringVar(&UserPathToScript, "u", "", "The absolute path to your userdata.sh script (optional).")
	flag.StringVar(&UserTagKey, "tk", "Name", "The key of the tag you'd like to assign your EC2 instance (optional).")
	flag.StringVar(&UserTagValue, "tv", "Created by Discord", "The value of the tag you'd like to assign your EC2 instance (optional).")
	flag.StringVar(&UserKeyName, "k", "", "The name of the key pair you'd like to assign to your EC2 instance for remote access (optional).")
	flag.StringVar(&UserInstanceType, "it", "t3a.medium", "The type, and size, of the EC2 instance you'd like to create. Defaults to t (optional).")

	// IAM Instance Profiles
	flag.StringVar(&UserIamArn, "ia", "", "The ARN of the IAM Instance Profile you'd like to attach to your EC2 instance on !create commands (optional).")
	flag.StringVar(&UserIamProfileName, "in", "", "The Name of the IAM Instance Profile you'd like to attach to your EC2 instance on !create commands (required if using -ia flag)")

	// Used for more granular !help messages, eventually to be used for a service healthcheck
	flag.StringVar(&UserServiceName, "svc", "", "If your server is running a specific service, you can use this flag to specify its name (optional).")
	flag.StringVar(&UserServicePort, "sp", "", "If your service is running on a specific port, you can use this flag to include it in your !help message (optional).")
	flag.StringVar(&ServiceCheckPort, "scp", "", "If your service is running a health check, you can specify what port (on the EC2 instance) to send requests to (optional).")

	// Stuff for GenerateOTP
	flag.IntVar(&OTPLength, "o", 6, "The length of the OTP you'd like to generate (optional).")

	flag.Parse()
}

// EC2InstanceAPI defines the interface for the RunInstances, CreateTags, and TerminateInstances functions.
type EC2InstanceAPI interface {
	RunInstances(ctx context.Context,
		params *ec2.RunInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)

	CreateTags(ctx context.Context,
		params *ec2.CreateTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)

	TerminateInstances(ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)

	AssociateIamInstanceProfile(ctx context.Context,
		params *ec2.AssociateIamInstanceProfileInput,
		optFns ...func(*ec2.Options)) (*ec2.AssociateIamInstanceProfileOutput, error)
}

// Creates an EC2 instance
func MakeInstance(c context.Context, api EC2InstanceAPI, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	return api.RunInstances(c, input)
}

// Associates IAM Role with EC2 Instance
func AttachIamRole(c context.Context, api EC2InstanceAPI, input *ec2.AssociateIamInstanceProfileInput) (*ec2.AssociateIamInstanceProfileOutput, error) {
	return api.AssociateIamInstanceProfile(c, input)
}

// Creates tags fo created EC2 instance
func CreateTag(c context.Context, api EC2InstanceAPI, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return api.CreateTags(c, input)
}

// Terminates an EC2 instance
func TerminateInstance(c context.Context, api EC2InstanceAPI, input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return api.TerminateInstances(c, input)
}

// Generates a one time password
func GenerateOTP(length int) (string, error) {
	buffer := make([]byte, length)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}

	otpCharsLength := len(otpChars)
	for i := 0; i < length; i++ {
		buffer[i] = otpChars[int(buffer[i])%otpCharsLength]
	}

	oneTimePassword = string(buffer)

	log.Println("Your One Time Password:", oneTimePassword)

	return string(buffer), nil
}

// Listens for new messages, starts a timer / loop to send messages to discord anytime there's a new RSS message
func messageCreated(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Checks to see if the message that was created is one of the following:
	// !create 	  -- Creates new EC2 instance, outputs "2FA" code to bot's error logs so only admins can create EC2 instances
	// !status 	  -- Checks the status of the EC2 instance
	// !start  	  -- Starts the EC2 instance
	// !stop   	  -- Stops the EC2 instance
	// !terminate -- Terminates (deletes) the EC2 instance, outputs "2FA" code to bot's error logs so only admins can terminate instances
	// !help   	  -- Sends a discord message with all command information

	// Bails out of the script if the new message is from this bot
	if m.Author.ID == s.State.User.ID {
		return
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Println("Error loading config:", err)
	}

	client := ec2.NewFromConfig(cfg)

	switch m.Content {
	case "!status":
		if UserInstanceId == "" {
			tagName := "tag:" + UserTagKey

			status, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				InstanceIds: instanceIds,
				Filters: []types.Filter{
					{
						Name:   aws.String(tagName),
						Values: []string{UserTagValue},
					},
				},
			})
			if err != nil {
				log.Println("Error getting status:", err)
			}

			if *status.Reservations[0].Instances[0].InstanceId != "" {
				UserInstanceId = *status.Reservations[0].Instances[0].InstanceId
			} else {
				statusMessage = "You don't have any running or stopped instances! Get started by running the `!create` command."
				_, err = s.ChannelMessageSend(ChannelId, statusMessage)
				if err != nil {
					log.Println("Error sending message:", err)
				}

				return
			}
		}

		instanceIds = append(instanceIds, UserInstanceId)

		status, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
			InstanceIds: instanceIds,
		})

		if err != nil {
			log.Println("Error getting status:", err)
		}

		for _, r := range status.Reservations {
			for _, i := range r.Instances {

				instanceState := fmt.Sprintf("%v", *i.State)

				if strings.Contains(instanceState, "running") {
					if ServiceCheckPort == "" {
						statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`", *i.InstanceId, *i.PublicIpAddress, *i.State)
					} else {
						instanceIpAndPort := fmt.Sprint("http://", *i.PublicIpAddress, ":", ServiceCheckPort)

						resp, err := http.Get(instanceIpAndPort)
						if err != nil {
							log.Println("Error sending GET request to instance: ", err)
							statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`\n`%s` status cannot be checked right now. See your bot's error logs for more information.", *i.InstanceId, *i.PublicIpAddress, *i.State, UserServiceName, "inactive", UserServicePort)
						}

						respStatus := string(resp.Status)

						if strings.Contains(respStatus, "200") {
							statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`\n`%s` is currently `%s` on port `%s`", *i.InstanceId, *i.PublicIpAddress, *i.State, UserServiceName, "active", UserServicePort)
						} else {
							statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`\n`%s` is currently `%s` on port `%s`", *i.InstanceId, *i.PublicIpAddress, *i.State, UserServiceName, "inactive", UserServicePort)
						}
					}

				} else {
					statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance State: `%v`", *i.InstanceId, *i.State)
				}
			}
		}
		log.Println(statusMessage)

		_, err = s.ChannelMessageSend(ChannelId, statusMessage)
		if err != nil {
			log.Println("Error sending message:", err)
		}
	case "!start":
		if UserInstanceId == "" {
			tagName := "tag:" + UserTagKey

			status, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				InstanceIds: instanceIds,
				Filters: []types.Filter{
					{
						Name:   aws.String(tagName),
						Values: []string{UserTagValue},
					},
				},
			})
			if err != nil {
				log.Println("Error getting status:", err)
			}

			UserInstanceId = *status.Reservations[0].Instances[0].InstanceId
		}

		instanceIds = append(instanceIds, UserInstanceId)

		_, err := client.StartInstances(context.TODO(), &ec2.StartInstancesInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			log.Println("Error starting EC2 instance:", err)
			_, err = s.ChannelMessageSend(ChannelId, "**ERROR**: There was an error trying to start your EC2 instance. Please see your bot's error logs for more information.")
			if err != nil {
				log.Println("Error sending message:", err)
			}
		} else {
			_, err = s.ChannelMessageSend(ChannelId, "Starting EC2 instance...\nUse **`!status`** to track the status of your server as it comes online!")
			if err != nil {
				log.Println("Error sending message:", err)
			}
		}
	case "!stop":
		if UserInstanceId == "" {
			tagName := "tag:" + UserTagKey

			status, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
				InstanceIds: instanceIds,
				Filters: []types.Filter{
					{
						Name:   aws.String(tagName),
						Values: []string{UserTagValue},
					},
				},
			})
			if err != nil {
				log.Println("Error getting status:", err)
			}

			UserInstanceId = *status.Reservations[0].Instances[0].InstanceId
		}

		instanceIds = append(instanceIds, UserInstanceId)

		_, err := client.StopInstances(context.TODO(), &ec2.StopInstancesInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			log.Println("Error stopping EC2 instance:", err)
			statusMessage = "There was an error stopping your EC2 instance. Please check your bot's error logs for more information."
			_, err = s.ChannelMessageSend(ChannelId, statusMessage)
			if err != nil {
				log.Println("Error sending message:", err)
			}
		} else {
			_, err = s.ChannelMessageSend(ChannelId, "Stopping EC2 instance...")
			if err != nil {
				log.Println("Error sending message:", err)
			}
		}
	case "!help":
		if UserServiceName != "" && UserServicePort != "" {
			helpMessage := fmt.Sprintf("**`!create`** -- Creates a brand new EC2 instances\n**`!status`** -- Checks the status of the EC2 instance, checks for public IP address\n**`!start`** -- Starts your EC2 instance\n**`!stop`** -- Stops your EC2 instance\n**`!terminate`** -- Terminates (deletes) your EC2 instance\n**`!help`** -- Displays commands and what they do :smile:\n\n`%s` is running on port `%s`", UserServiceName, UserServicePort)
			_, err := s.ChannelMessageSend(ChannelId, helpMessage)
			if err != nil {
				log.Println("Error sending message:", err)
			}
		} else if UserServiceName != "" && UserServicePort == "" {
			helpMessage := fmt.Sprintf("**`!create`** -- Creates a brand new EC2 instances\n**`!status`** -- Checks the status of the EC2 instance, checks for public IP address\n**`!start`** -- Starts your EC2 instance\n**`!stop`** -- Stops your EC2 instance\n**`!terminate`** -- Terminates (deletes) your EC2 instance\n**`!help`** -- Displays commands and what they do :smile:\n\nYour EC2 instance is running `%s`.", UserServiceName)
			_, err := s.ChannelMessageSend(ChannelId, helpMessage)
			if err != nil {
				log.Println("Error sending message:", err)
			}
		} else {
			helpMessage := fmt.Sprintf("**`!create`** -- Creates a brand new EC2 instances\n**`!status`** -- Checks the status of the EC2 instance, checks for public IP address\n**`!start`** -- Starts your EC2 instance\n**`!stop`** -- Stops your EC2 instance\n**`!terminate`** -- Terminates (deletes) your EC2 instance\n**`!help`** -- Displays commands and what they do :smile:")
			_, err := s.ChannelMessageSend(ChannelId, helpMessage)
			if err != nil {
				log.Println("Error sending message:", err)
			}
		}
	default:
		// Clears out previous message array
		previousDiscordMessages = nil

		log.Println("Getting last 3 messages...")
		previousDiscordMessagesStructs, err := s.ChannelMessages(ChannelId, 3, "", "", "")
		if err != nil {
			log.Println("Error getting previous messages:", err)
		}

		if len(previousDiscordMessagesStructs) > 0 {
			for i := 0; i < len(previousDiscordMessagesStructs); i++ {
				previousDiscordMessages = append(previousDiscordMessages, previousDiscordMessagesStructs[i].Content)
			}
		}

		if strings.Contains(previousDiscordMessages[0], "!create") {
			GenerateOTP(OTPLength)
			return
		}

		if strings.Contains(previousDiscordMessages[0], "!terminate") {
			GenerateOTP(OTPLength)
			return
		}

		if previousDiscordMessages[0] == oneTimePassword && strings.Contains(previousDiscordMessages[1], "!create") {

			// Breaks !create message into an array of strings
			messageContentSlice := strings.Fields(previousDiscordMessages[1])

			// Sets flags we're looking for in this command
			flagArray := []string{"-sn", "-sg", "-ami", "-tk", "-tv", "-u", "-svc", "-sp", "-scp", "-ia", "-in", "-k", "-it"}

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
				break
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}

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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}

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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
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

								_, err = s.ChannelMessageSend(ChannelId, statusMessage)
								if err != nil {
									log.Println("Error sending message:", err)
								}
								return
							}
						}
					}
				}

				if UserIamArn != "" && UserIamProfileName != "" {
					log.Println("Error, cannot use -in and -ia flags together. Please run the !create command again with only one flag specified.")
					statusMessage = "Error, cannot use -in and -ia flags together. Please run the `!create` command again with only one flag specified."

					_, err = s.ChannelMessageSend(ChannelId, statusMessage)
					if err != nil {
						log.Println("Error sending message:", err)
					}

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

					_, err = s.ChannelMessageSend(ChannelId, statusMessage)
					if err != nil {
						log.Println("Error sending message:", err)
					}
					return
				}

				UserInstanceId = *result.Instances[0].InstanceId

				if result.Instances[0].InstanceId != nil {
					log.Println("OTP entered correctly, instance created:", UserInstanceId)
					statusMessage = fmt.Sprintf("One time password entered correctly, your EC2 instance has been created!\nInstance ID: `%s`", UserInstanceId)
					instanceIds = append(instanceIds, UserInstanceId)
				}
				_, err = s.ChannelMessageSend(ChannelId, statusMessage)
				if err != nil {
					log.Println("Error sending message:", err)
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

		}

		if previousDiscordMessages[0] == oneTimePassword && strings.Contains(previousDiscordMessages[1], "!terminate") {

			// Breaks !create message into an array of strings
			messageContentSlice := strings.Fields(previousDiscordMessages[1])
			instanceIds = nil

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
			}

			log.Println("One time password entered correctly, terminating EC2 instance.")
			statusMessage = "One time password entered correctly, terminating EC2 instance."
			_, err = s.ChannelMessageSend(ChannelId, statusMessage)
			if err != nil {
				log.Println("Error sending message:", err)
			}
			UserInstanceId = ""
		}

		return
	}
}

func main() {
	// Creating Discord Session Using Provided Bot Token
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Println("Error creating Discord Session:", err)
		return
	}

	// Registers the messageCreated function as a callback for a MessageCreated Event
	dg.AddHandler(messageCreated)

	// Sets the intentions of the bot, read through the docs
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Println("Error opening websocket connection:", err)
		return
	} else {
		log.Println("Discord websocket connection opened successfully")
	}

	// Wait here until CTRL+C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Sends message to ChannelId that the bot is shutting down
	dg.ChannelMessageSend(ChannelId, "Robot shutting down...")

	// Cleanly close down the Discord session.
	dg.Close()
}
