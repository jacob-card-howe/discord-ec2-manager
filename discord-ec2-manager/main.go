package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
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

	"github.com/jacob-howe/discord-ec2-manager/discord-ec2-manager/create"
	"github.com/jacob-howe/discord-ec2-manager/discord-ec2-manager/status"
	"github.com/jacob-howe/discord-ec2-manager/discord-ec2-manager/terminate"
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

	// Run Instances Input for Key Pair Name Check
	runInstancesInput *ec2.RunInstancesInput

	// One Time Password
	oneTimePassword string

	// Status Message
	statusMessage string

	// Instance ID
	instanceIds []string

	// SG IDs
	SecurityGroupIds []string

	// Used to keep track of recent messages in the bot's discord channel
	previousDiscordMessages []string

	validUserSubnetId bool
)

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

		if strings.Contains(previousDiscordMessages[0], "!status") {
			log.Println("Converting most recent message to a slice...")
			messageContentSlice := strings.Fields(previousDiscordMessages[0])

			log.Println("Running GetEc2InstanceStatus...")
			statusMessage = status.GetEc2InstanceStatus(messageContentSlice, instanceIds, UserTagKey, UserTagValue, ServiceCheckPort, UserServiceName, UserServicePort, client)
			_, err = s.ChannelMessageSend(ChannelId, statusMessage)
			if err != nil {
				log.Println("Error sending message:", err)
				return
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

			statusMessage, UserInstanceId, UserTagKey, UserTagValue, UserServiceName, UserServicePort, ServiceCheckPort = create.CreateEc2Instance(messageContentSlice, flagArray, client)
			_, err = s.ChannelMessageSend(ChannelId, statusMessage)
			if err != nil {
				log.Println("Error sending message:", err)
			}

			instanceIds = append(instanceIds, UserInstanceId)
		}

		if previousDiscordMessages[0] == oneTimePassword && strings.Contains(previousDiscordMessages[1], "!terminate") {

			// Breaks !create message into an array of strings
			messageContentSlice := strings.Fields(previousDiscordMessages[1])

			statusMessage, UserInstanceId = terminate.TerminateEc2Instance(messageContentSlice, instanceIds, client)
			_, err = s.ChannelMessageSend(ChannelId, statusMessage)
			if err != nil {
				log.Println("Error sending message:", err)
				return
			}
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
