- src/main.go: execd source was done monolithic. Because I think it's "all about one".
- execd.conf: The sample config. Default locations is "/etc/execd/execd.conf"
- msg.json: Example on how to compose the message. Push ("produce") it into journal ("topic"):
    kafkacat -P -b ext-kafka.xxx.local:9093 -X "security.protocol=ssl" -X "ssl.ca.location=/etc/ssl/certs/" -X topic.request.required.acks=all -t knot.test msg.json

