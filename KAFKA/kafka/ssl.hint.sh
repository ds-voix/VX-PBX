#!/bin/bash
# ***
umask 077
# https://docs.confluent.io/current/security/security_tutorial.html#generating-keys-certs #

mkdir -p /etc/kafka/ssl/
cd /etc/kafka/ssl/

### Root of the trust ###
# CA key pair
 openssl req -x509 -new -keyout CA.key -days 36500 -out CA.crt -subj "/CN=ext-kafka" -passout pass:***
# -import: Make truststore, add CA
 keytool -keystore kafka.server.truststore.jks -deststoretype pkcs12 -alias kafka-CA -import -file CA.crt -storepass *** -noprompt

### !!! "CA.crt" must be trusted by clients, otherwise "verify error:num=19:self signed certificate in certificate chain"

### One key pair per host ###
# -genkey: Make keystore, add cert
# !!! "-keyalg RSA", DSA is unsupported outside of java!!!
 keytool -keystore kafka.server.xxx.jks -deststoretype pkcs12 -alias ext-kafka.xxx -validity 3650 -genkeypair -keyalg RSA -keypass *** -storepass *** -noprompt -dname "CN=ext-kafka.xxx.local"
 keytool -keystore kafka.server.yyy.jks -deststoretype pkcs12 -alias ext-kafka.yyy -validity 3650 -genkeypair -keyalg RSA -keypass *** -storepass *** -noprompt -dname "CN=ext-kafka.yyy.local"
 keytool -keystore kafka.server.zzz.jks -deststoretype pkcs12 -alias ext-kafka.zzz -validity 3650 -genkeypair -keyalg RSA -keypass *** -storepass *** -noprompt -dname "CN=ext-kafka.zzz.local"
# !!! Warning:  Different store and key passwords not supported for PKCS12 KeyStores. Ignoring user-specified -keypass value.

# -certreq: CSRs
 keytool -keystore kafka.server.xxx.jks -alias ext-kafka.xxx -certreq -file ext-kafka.xxx.csr -storepass *** -noprompt
 keytool -keystore kafka.server.yyy.jks -alias ext-kafka.yyy -certreq -file ext-kafka.yyy.csr -storepass *** -noprompt
 keytool -keystore kafka.server.zzz.jks -alias ext-kafka.zzz -certreq -file ext-kafka.zzz.csr -storepass *** -noprompt
# Signed CRTs
 openssl x509 -req -CA CA.crt -CAkey CA.key -in ext-kafka.xxx.csr -out ext-kafka.xxx.crt -days 3650 -CAcreateserial -passin pass:***
 openssl x509 -req -CA CA.crt -CAkey CA.key -in ext-kafka.yyy.csr -out ext-kafka.yyy.crt -days 3650 -CAcreateserial -passin pass:***
 openssl x509 -req -CA CA.crt -CAkey CA.key -in ext-kafka.zzz.csr -out ext-kafka.zzz.crt -days 3650 -CAcreateserial -passin pass:***

# Import root CA into keystore
 keytool -keystore kafka.server.xxx.jks -alias kafka-CA -import -file CA.crt -storepass *** -noprompt
 keytool -keystore kafka.server.yyy.jks -alias kafka-CA -import -file CA.crt -storepass *** -noprompt
 keytool -keystore kafka.server.zzz.jks -alias kafka-CA -import -file CA.crt -storepass *** -noprompt

# Import kafka certs into keystore
 keytool -keystore kafka.server.xxx.jks -alias ext-kafka.xxx -import -file ext-kafka.xxx.crt -storepass *** -noprompt
 keytool -keystore kafka.server.yyy.jks -alias ext-kafka.yyy -import -file ext-kafka.yyy.crt -storepass *** -noprompt
 keytool -keystore kafka.server.zzz.jks -alias ext-kafka.zzz -import -file ext-kafka.zzz.crt -storepass *** -noprompt


### Now, copy "kafka.server.XXX.jks" & "kafka.server.truststore.jks" to appropriate hosts (xxx,yyy,zzz)
### Edit /etc/kafka/server.properties
### On each host:
chown -R kafka:kafka /etc/kafka/
# chown root:root /etc/kafka/ssl/CA.key
chmod 400 /etc/kafka/*.properties
chmod 400 /etc/kafka/ssl/*

systemctl restart kafka.service
# Errors in SSL leads to java exceptions. Which are throwed out (traditionaly for java progers).
less /var/log/kafka/controller.log
less /var/log/kafka/server.log
# [kafka.cluster.Partition] INFO [Partition knot.test-0 broker=1] Expanding ISR from 1,2 to 1,2,3 (kafka.cluster.Partition)

### Check SSL after kafka config patch & restart
openssl s_client -connect ext-kafka.xxx.local:9093




### ZooKeeper ###
mkdir -p /etc/zookeeper/ssl/
cp /etc/kafka/ssl/*.jks /etc/zookeeper/ssl/
chown -R zookeeper:zookeeper /etc/zookeeper/ssl
chmod 400 /etc/zookeeper/ssl/*

### config
#### SSL ### https://zookeeper.apache.org/doc/r3.5.5/zookeeperAdmin.html
# !!! ZooKeeper must be upgraded to 3.5.latest! 3.4 has no SSL at all !!!
#sslQuorum=true
#serverCnxnFactory=org.apache.zookeeper.server.NettyServerCnxnFactory
#ssl.quorum.trustStore.location=/etc/zookeeper/ssl/kafka.server.truststore.jks
#ssl.quorum.trustStore.password=***
#ssl.quorum.keyStore.location=/etc/zookeeper/ssl/kafka.server.xxx.jks
#ssl.quorum.keyStore.password=***
