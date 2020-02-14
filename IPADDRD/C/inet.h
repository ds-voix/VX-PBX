#define _GNU_SOURCE     /* To get defns of NI_MAXSERV and NI_MAXHOST */
#include <arpa/inet.h>
#include <sys/socket.h>
#include <netdb.h>
#include <ifaddrs.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <linux/if_link.h>

#include <string.h>

#include <sys/ioctl.h>
#include <net/if.h>

// readdir("/run/netns/")
#include <dirent.h>
#include <linux/limits.h>

// http://man7.org/linux/man-pages/man2/setns.2.html
#include <fcntl.h>
#include <sched.h>

// Pack ip addresses into udp datagram.
// BITMASK "head" = (0,0,0,0,IFF_UP,IFF_BROADCAST,IFF_PROMISC,is_IPv6)
struct ipv4_frame {
  unsigned char head;
  uint32_t ip;
} __attribute__((packed));

struct ipv6_frame {
  unsigned char head;
  uint32_t ip[4];
} __attribute__((packed));

// > (200 x IPv6 addresses in 4kB)
#define BUFFER_SIZE (4096)

struct server_addr { // >> getaddrinfo(const char *node, const char *service, ...)
  unsigned char * host;
  unsigned char * port;
};

struct server_addr server;

#include <errno.h>
#include <pthread.h>

struct _fp {
  unsigned char * frame;
  unsigned char * frame_pos;
  uint16_t ip_count;
  int result;
};

// crc32
#include <zlib.h>

// https://stackoverflow.com/questions/17954432/creating-a-daemon-in-linux
#include <signal.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <syslog.h>

// http://linux.die.net/man/3/uuid_generate
#include <uuid/uuid.h>

// Global mutex
#include <sys/file.h>
int fd_lock;

// "volatile" prevents "optimize out from code" for variable checks
volatile sig_atomic_t terminated = 0; // Terminate execution on signal

// CLI
int debug = 0;
unsigned long delay_report = 15;
unsigned long delay_start = 25;
unsigned long delay_random = 1; // All delays are in seconds
// can this #define facilitate further sizeof()?
#define PID_FILE "/var/run/ipaddrd.pid"
unsigned char * pid_file = NULL;
