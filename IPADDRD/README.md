Huh, telemetry is the global trend.
But this service stands aside from privacy draining. It's about early IP addresses duplicates detection.

I know no one "silver bullet" working on all topologies. This one implies the intranet with m.b. thouthands hosts across the number of vlan's.
There can appear duplicates. E.g. due to split-brain in CARP, or defective IP management (some parts of openStack) and so on.
Therefore, while I'm responsible for such a network, I have no correct view outside of hosts.

But, I can install on each _(in fact, "most of")_ host the micro-tool.
Just to send (time-to-time) the actual list of IPv4|IPv6 toward some collector.
* This tool must be really *micro*. So, C is the ceiling (while asm is better…).
_Under glibc, it consumes from 200k to 2M RSS (depending on linux settings/glibc bugs), for nothing._
* But collector (server side) could be any (golang is enough).
* The processing can at all be the shell script.
