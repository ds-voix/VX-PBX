As of 2019, I daresay Apache kafka is mature enough to deal with.
It's now in fact the only "shared journal" working. With known issues, but, in general, it works.

On my working area, there was accumulated the set of tasks for such a journal.
Under this folder I will put (my) solutions residing on this bus.

- "execd" is the most fundamental concept.
The interface between shared messaging bus and any "regular" software.
It although can be used as a kind of "pdsh". "Poll" model, coupled with "push" latency.
The concept is: "Each agent reviews each message on the bus." * But further processing can be filtered.

- "modules" - common modules between execd, dnsd etc.

- "kafka" contains the number of solutions to get Kafka server compliant with the project demands:
* High Availability, across the world (thus, 3+ sites)
* SSL: All communications must be encrypted. Connections must be verified against CA.
* ACL: Though access control is evidently inconvenient in current Kafka, it have to be set accurately.
  Because any shared communications are although the obvious security hole.
  Stay out of hype, tune and control each security relation. Regards.

- "zookeeper" contains the template for the shared storage config.
As for ZooKeeper v3.4, it has nothing about security. SSL starts with 3.5.
In any case, I recommend to put all about messaging inside some security perimeter.
IMO, no one in java world thinks about security. Because of it's "enterprise" nature. Security brings down KPI.
