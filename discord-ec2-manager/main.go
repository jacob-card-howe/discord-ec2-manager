package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
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

	// Service Specific Variables
	UserServiceName string
	UserServicePort string

	// Allows for custom OTP lengths
	OTPLength int
)

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

	// Used for more granular !help messages, eventually to be used for a service healthcheck
	flag.StringVar(&UserServiceName, "svc", "", "If your server is running a specific service, you can use this flag to specify its name (optional).")
	flag.StringVar(&UserServicePort, "p", "", "If your service is running on a specific port, you can use this flag to include it in your !help message (optional).")

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
}

// Creates an EC2 instance
func MakeInstance(c context.Context, api EC2InstanceAPI, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	return api.RunInstances(c, input)
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
	case "!create":
		GenerateOTP(OTPLength)
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

			UserInstanceId = *status.Reservations[0].Instances[0].InstanceId
		}

		instanceIds = append(instanceIds, UserInstanceId)
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

		for _, r := range status.Reservations {
			for _, i := range r.Instances {

				instanceState := fmt.Sprintf("%v", *i.State)

				if strings.Contains(instanceState, "running") {
					statusMessage = fmt.Sprintf("Instance ID: `%s`\nInstance IP: `%s`\nInstance State: `%v`", *i.InstanceId, *i.PublicIpAddress, *i.State)
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
	case "!terminate":
		GenerateOTP(OTPLength)
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
				log.Println("Adding this message:", previousDiscordMessagesStructs[i].Content)
				previousDiscordMessages = append(previousDiscordMessages, previousDiscordMessagesStructs[i].Content)
			}
		}

		if previousDiscordMessages[0] == oneTimePassword && strings.Contains(previousDiscordMessages[1], "!create") {
			if UserAmiId == "" {
				UserAmiId = "ami-09e67e426f25ce0d7" // Ubuntu 20.04
			}
			if UserSecurityGroupId == "" {
				log.Println("Your EC2 instance will be created with your VPC's default security group. You may not have access to your EC2 instance!")
			} else {
				SecurityGroupIds = append(SecurityGroupIds, UserSecurityGroupId)
			}
			if UserSubnetId == "" {
				log.Println("To use !create, you MUST specify a subnet via the -sn flag when starting the bot. Please restart the bot with your included Security Group ID to continue with full functionality.")
				statusMessage = "**ERROR**: The bot was misconfigured on start up, please check your bot's error logs for more information."
			}

			content, err := ioutil.ReadFile(UserPathToScript)
			if err != nil {
				log.Println("Error reading from file:", err)
			}

			encodedUserDataContent := base64.StdEncoding.EncodeToString([]byte(content))

			input := &ec2.RunInstancesInput{
				ImageId:          aws.String(UserAmiId),
				InstanceType:     types.InstanceTypeT3aMedium,
				MinCount:         aws.Int32(1),
				MaxCount:         aws.Int32(1),
				SecurityGroupIds: SecurityGroupIds,
				SubnetId:         aws.String(UserSubnetId),
				UserData:         aws.String(encodedUserDataContent),
			}

			result, err := MakeInstance(context.TODO(), client, input)
			if err != nil {
				fmt.Println("Error creating EC2 instance:", err)
			}

			UserInstanceId = *result.Instances[0].InstanceId

			if result.Instances[0].InstanceId != nil {
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
			}

		} else if previousDiscordMessages[0] == oneTimePassword && strings.Contains(previousDiscordMessages[1], "!terminate") {
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
		} else if previousDiscordMessages[0] != oneTimePassword && (previousDiscordMessages[1] == "!create" || previousDiscordMessages[1] == "!terminate") {
			log.Println("One time password entered incorrectly. Message entered:", m.Content)
			statusMessage = "One time password entered incorrectly, please re-enter your ****`!create`** or **`!terminate`** command to re-generate your one time password."
			_, err = s.ChannelMessageSend(ChannelId, statusMessage)
			if err != nil {
				log.Println("Error sending message:", err)
			}
			return
		} else {
			log.Println("Not looking for OTP here, ignoring message.", previousDiscordMessages[1])
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
