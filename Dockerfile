FROM golang:1.16.3-alpine3.13

WORKDIR /app
COPY /discord-ec2-manager/ .
RUN go mod download
ENV BOT_TOKEN=""
ENV CHANNEL_ID=""
ENV INSTANCE_ID="i-defaultvalue"
ENV SECURITY_GROUP_ID="sg-defaultvalue"
ENV AMI_ID="ami-09e67e426f25ce0d7"
ENV SUBNET_ID="subnet-defaultvalue"
ENV PATH_TO_USERDATA="default/path"
ENV TAG_KEY="Name"
ENV TAG_VALUE="Created by Discord"
ENV OTP_LENGTH=6
RUN go build
CMD ./discord-ec2-manager -t $BOT_TOKEN -c $CHANNEL_ID -i $INSTANCE_ID -sg $SECURITY_GROUP_ID -a $AMI_ID -sn $SUBNET_ID -u $PATH_TO_USERDATA -tk $TAG_KEY -TAG_VALUE -o $OTP_LENGTH