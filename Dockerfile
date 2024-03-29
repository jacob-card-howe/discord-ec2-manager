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
ENV USER_SERVICE="my-service"
ENV USER_PORT="57420"
ENV SERVICE_CHECK_PORT="7777"
ENV IAM_ARN=""
ENV IAM_NAME=""
ENV KEY_NAME=""
ENV OTP_LENGTH=6
ENV INSTANCE_TYPE="t3.medium"

RUN go build
CMD ./discord-ec2-manager -t $BOT_TOKEN -c $CHANNEL_ID -i $INSTANCE_ID -sg $SECURITY_GROUP_ID -a $AMI_ID -sn $SUBNET_ID -u $PATH_TO_USERDATA -tk $TAG_KEY -tv $TAG_VALUE -svc $USER_SERVICE -sp $USER_PORT -scp $SERVICE_CHECK_PORT -ia $IAM_ARN -in $IAM_NAME -k $KEY_NAME -it $INSTANCE_TYPE -o $OTP_LENGTH