FROM golang:1.16.3-alpine3.13

WORKDIR /app
COPY /discord-ec2-manager/ .
RUN go mod download
ENV BOT_TOKEN="defaultvalue"
ENV CHANNEL_ID="12345678910"
ENV INSTANCE_ID="i-defaultvalue"
RUN go build
CMD ./discord-ec2-manager -t $BOT_TOKEN -c $CHANNEL_ID -i $INSTANCE_ID