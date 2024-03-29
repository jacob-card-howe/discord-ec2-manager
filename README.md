# discord-ec2-manager
A go-powered Discord bot capable of creating, starting, stopping, and terminating AWS EC2 instances.

## Build Statuses
[![Build Go Binary](https://github.com/jacob-card-howe/discord-ec2-manager/actions/workflows/go-build.yml/badge.svg)](https://github.com/jacob-card-howe/discord-ec2-manager/actions/workflows/go-build.yml)
[![Build Docker Image](https://github.com/jacob-card-howe/discord-ec2-manager/actions/workflows/build-docker-image.yml/badge.svg)](https://github.com/jacob-card-howe/discord-ec2-manager/actions/workflows/build-docker-image.yml)

## Introduction
About once a year my friends and I all get this crazy urge to play Minecraft with one another. We play the hell out of it for about a week and then drop it to do other things. The `discord-ec2-manager` project is the spawn of my obessive over-engineering for the love of Minecraft. 

But Minecraft isn't this project's only claim to fame! No, you see this project has all sorts of tricks up its sleeves. It is capable of creating / terminating AWS EC2 instances (protected, of course, by a pseudo-randomly generated one time password outputted to the bot's logs), it can start and stop an existing server via a string passed in through the bot's executable, and it can even accept custom tag names and values. 

Below you'll find information on how you can use the bot both locally and via Docker, as well as a description of both the bot's command line arguments, and Discord commands.
___

## Requirements
* [Go](https://golang.org/dl/) version `1.16` or higher (if you're compiling the executable yourself)
* [An AWS account](https://aws.amazon.com/) with damn near full `EC2` permissions, or access to a Role that has said `EC2` permissions
* Access to a [Discord Bot](https://discord.com/developers/applications/)
___

## Running `discord-ec2-manager` Locally
There will always be at _least_ three required arguments while running `discord-ec2-manager` locally. Below you'll find a breakdown of available arguments and _their_ requirements below:

<details>
    <summary> Click this dropdown to see a full list of parameter flags!</summary>

### `-t` Discord Bot Token (**REQUIRED**)
The `t` flag sets your Discord Bot Token. There is no default value, and the flag accepts a string as input. For more information on how to generate a Discord Bot Token, [check out this article](https://www.freecodecamp.org/news/create-a-discord-bot-with-python/) by [freecodecamp.org](https://freecodecamp.org)
___

### `-c` Discord Server Channel ID (**REQUIRED**)
The `-c` flag sets your Discord Channel ID, i.e. where the bot will listen for / post new messages. There is no default value, and the flag accepts a string as input. For more information on how to enable developer mode on your Discord client, [check out this article](https://www.howtogeek.com/714348/how-to-enable-or-disable-developer-mode-on-discord/) by [howtogeek.com](https://howtogeek.com).
___

### `-i` AWS EC2 Instance ID (_**Optional**_*)
The `-i` flag sets the EC2 Instance ID of the EC2 instance you want to manage via Discord. This flag is optional, however, it is optional *only* if you do not intend on using the `!create` Discord bot command. There is no default value and the flag accepts a string as input.

**`-i` Example via CLI:**
`.\discord-ec2-manager.exe -t "My Discord Bot Token" -c "My Discord Channel ID" -i "i-abcde1234fghijkl"`
___

### `-sg` AWS EC2 Instance Security Group ID (Optional)
The `-sg` flag sets the EC2 Instance Security Group that you'd like to attach to your EC2 instance upon using the `!create` Discord bot command. The flag will default to your VPC's default security group and accepts a string as input. **NOTE** if you're using the `-i` parameter flag, the `-sg` flag will do nothing as it is **only** used in conjunction with the `!create` Discord bot command.
___

### `-a` AWS EC2 Instance AMI ID (Optional)
The `-a` flag sets the EC2 Instance's Amazon Machine Image (AMI) that you'd like to attach to your EC2 instance upon using the `!create` Discord bot command. The flag defaults to `ami-09e67e426f25ce0d7`, which is an Ubuntu 20.04 image located in `us-east-1`, and accepts a string as an input. **NOTE** if you're using the `-i` parameter flag, the `-a` flag will do nothing as it is **only** used in conjunction with the `!create` Discord bot command.
___

### `-sn` AWS EC2 Subnet ID (_**Optional**_*)
**IF YOU ARE _NOT_ USING THE `-i` PARAMETER FLAG, THE `-sn` FLAG IS A REQUIRED ARGUMENT**. The `-sn` flag sets the EC2 Instance's Subnet that you'd like to create it in upon using the `!create` Discord bot command. The flag does not have a default and accepts a string as an input. 
___

### `-u` Absolute Path to AWS EC2 User Data Script (Optional)
The `-u` flag allows you to enter in the absolute path of your `user data` script. There is no default value, but the flag accepts a string as an input. 

**`-u` Example via CLI:**
`.\discord-ec2-manager.exe -t "Discord Bot Token" -c "Discord Channel ID" -sg "sg-1234abcde1234" -a "ami-abcde1234abcde" -sn "subnet-1234abcde" -u "C:\Users\my_user\Desktop\userdata.sh"`
___

### `-tk` AWS EC2 Tag Key (Optional)
The `-tk` flag allows you to set your custom tag's key to be whatever you want. The default value is `Name` and the flag accepts a string as an input.
___

### `-tv` AWS EC2 Tag Value (Optional)
The `-tv` flag allows you to give your custom tag a value of whatever you want. The default value is `Created by Discord` and the flag accepts a string as an input.
___

### `-svc` Custom Service Name Value (Optional)
The `-svc` flag allows you to enter in a custom service name that is running on your `discord-ec2-manager` managed EC2 instance. This flag accepts a string as an input.
___

### `-sp` Custom Service Port Value (Optional)
The `-sp` flag allows you to enter in a custom port that your service (denoted by the `-svc` flag) is running on in your `discord-ec2-manager` managed EC2 instance. This flag accepts a string as an input.
___

### `-scp` Custom Service Check Port Value (Optional)
The `-scp` flag allows you to enter in a custom port that your service's healthcheck is running on. When used, `discord-ec2-manager` will send an HTTP GET request to its managed EC2 instance over the port specified by `-scp`. This flag accepts a string as an input.
___

### `-ia` EC2 IAM Instance Profile ARN (Optional)
The `-ia` flag allows you to enter in the ARN of an IAM Role that you would like to attach to your EC2 instance on its creation. **Cannot be used if using the `-in` flag.** The flag does not have a default value, and accepts a string as an input.
___

### `-in` EC2 IAM Instance Profile Name (Optional)
The `-in` flag allows you to enter in the Name of the IAM Role that you would like to attach to your EC2 instance on its creation. **Cannot be used if using the `-ia` flag.** The flag does not have a default value, and accepts a string as an input.
___

### `-k` EC2 Instance Key Pair (Optional)
The `-k` flag allows you to enter in the Name of the key pair that you would like to attach to your EC2 instance on its creation. This, in conjunction with the rules you specify in your EC2 Instance's Security Group, will allow for remote access to your EC2 instance. The flag does not have a default value, and accepts a string as an input.
___

### `-it` EC2 Instance Type (Optional)
The `-it` flag allows you to configure the Type and Size of your EC2 instance on its creation. The flag defaults to `t3a.medium` and accepts a string as an input. 
___

### `-o` One Time Password Length (Optional)
With the `-o` flag, you're able to set the One Time Password's length. The default value is `6` and the flag accepts an integer as an input.

</details>

___


## Running `discord-ec2-manager` via ECS Fargate
This section will talk about some of the stuff you need to consider when spinning up the bot in AWS' ECS Fargate as a Docker container.

First and foremost, you'll want to build the Docker image by running `docker build -t discord-ec2-manager .` in the root of `discord-ec2-manager/` on your local device. If you're passing in a `user data` script (useful for the `!create` bot command), you'll want to make sure to include it in your `discord-ec2-manager/discord-ec2-manager` directory, and pass in the path to your file via `-e PATH_TO_USERDATA=`

Upload the image you've just built locally on your machine to AWS' Elastic Container Repository (ECR) service [by following AWS' documentation](https://docs.aws.amazon.com/AmazonECR/latest/userguide/docker-push-ecr-image.html) and read up on how to deploy it to ECS Fargate [on AWS' documentation page](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/AWS_Fargate.html).

👋🏻 **HEADS UP**: If you're running this in ECS, you may encounter issues if you pass in an empty string for `INSTANCE_ID`. I have `INSTANCE_ID` set to `i-actualgarbage` and things seem to be working fine for me up in AWS Land! 


## Discord Commands
This section will cover the commands available to you once the bot running and a member of your Discord server.

### `!create`
This command will generate a one time password (found in your bot's error logs). If your next message matches the OTP found in your bot's error logs, it will create a new EC2 instance with the tags, security group ID, and in the subnet you provided either via your bot's argument flags on start up **OR** via your bot's argument flags in your `!create` Discord message. Additionally, if you use the `-u` flag (either at start up or in your `!create` Discord message) to include a path to a User Data script, your EC2 instance will run those commands on intial boot.

**Example `!create` Discord Message:** `!create -sn subnet-1234abcde5678 -sg sg-1234abcde5678 -ami ami-1234abcde5678 -tk MyCustomTagKey -tv MyCustomTagValue -u /absolute/path/to/userdata.sh -svc MyServiceName -sp 1234 -scp 7777`
___

### `!terminate`
This command will generate a one time password (found in your bot's error logs). If your next message matches the OTP found in your bot's error logs, it will terminate all `discord-ec2-manager` managed EC2 instances. You can target specific instances with a `-i` parameter flag tailing your `!terminate` command in Discord.

**Example `!terminate` Discord Message:** `!terminate -i i-1234abcde5678`
___

### `!start`
This command will take a `stopped` EC2 instance and start it. 
___

### `!stop`
This command will take a `running` EC2 instance and stop it.
___

### `!status`
This command will return three pieces of information:
1. Your EC2 Instance's Instance ID (i-stringofcharacters)
1. Your EC2 Instance's Public IP Address (if public IP address is not nil)
1. Your EC2 Instance's State (`pending`, `running`, `stopped`, etc.)
1. Information regarding your service's name and service port (if `-svc` and `-sp` flags were used)
1. Your service's current status (if `-scp` flag was used, and service is serving a valid HTTP endpoint)
___

### `!help`
This command will tell you all about what each of the commands do on the Discord bot.
___

## Bonus Stuff
If you'd like a `user data` script that'll start Minecraft on your server's launch and subsequent reboots, as well as automatically stop the EC2 instance when no one is connected to the server via TCP port 25565, check out my [`mc-server`](https://github.com/jacob-howe/mc-server) project. 

## Additional Links / Resources
### Documentaion
* [`discordgo` Documentation via pkg.go.dev](https://pkg.go.dev/github.com/bwmarrin/discordgo)
* [`aws-sdk-go-v2/aws` Documentation via pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws)
* [`aws-sdk-go-v2/config` Documentation via pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config)
* [`aws-sdk-go-v2/service/ec2` Documentation via pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2)

### External Links
* [Discord Developer Portal](https://discord.com/developers/applications/)
* [Golang Download](https://golang.org/dl/)
* [Amazon Web Services (AWS) CLI Download](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html)
* [Amazon Web Services (AWS) Console](https://aws.amazon.com/console/)
* [_Pushing a Docker Image_ (AWS ECR Documentation)](https://docs.aws.amazon.com/AmazonECR/latest/userguide/docker-push-ecr-image.html)

### Go Modules
* [`discordgo` by bwmarrin](https://github.com/bwmarrin/discordgo)
* [`aws-sdk-go-v2` by aws](https://github.com/aws/aws-sdk-go-v2)
