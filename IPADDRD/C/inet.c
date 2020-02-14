// man getifaddrs
#include "inet.h"

// http://man7.org/linux/man-pages/man3/getopt.3.html
// http://man7.org/linux/man-pages/man2/nanosleep.2.html
/* ipaddrd [-d] [-t seconds] [-T seconds] [-p /path/to.pid] [-r seconds] host[:port]
opts:
   -d: Debug mode. Don't daemonize, use stderr instead of syslog.
   -t: Timeout between reports. Default is 15 seconds.
   -T: Timeout before first report. Default is 25 seconds.
   -p: Write pid-file (and global lock) at this path. Default is "/run/ipaddrd.pid"
   -r: Randomly shift inter-reporting delay, max to this value in seconds. Default is 1 second.
args:
   host: Send reports to this url|ip (via UDP)
   port: Send reports to this udp port. Default is 3333.
*/
void cli(int argc, char **argv[]) // Parse CLI args at start.
{
  int opt;
  optind = 1;

//  syslog(LOG_NOTICE, "argv[1]: %s", (*argv)[1]);

  while ((opt = getopt(argc, *argv, "dt:T:p:r:h?")) != -1) {
    switch (opt) {
      case 'd':
        debug = 1;
        break;

      case 't':
        delay_report = atoi(optarg);
        if (delay_report < 1 || delay_report > 86400) {
          fprintf(stderr, "-t=%d ??? Delay between reports must be an integer from 1 to 86400 seconds\n", delay_report);
          exit(EXIT_FAILURE);
        }
        break;

      case 'T':
        delay_start = atoi(optarg);
        if (delay_start < 1 || delay_start > 86400) {
          fprintf(stderr, "-T=%d ??? Delay before start reporting must be an integer from 1 to 86400 seconds\n", delay_start);
          exit(EXIT_FAILURE);
        }
        break;

      case 'p': // Copy path
        pid_file = malloc(sizeof(char) * strlen(optarg) + 1);
        if (pid_file == NULL) {
          fprintf(stderr, "Panic: malloc((pid_file) size = 0x%x failed (no free mem?)!\n", strlen(optarg));
          exit(EXIT_FAILURE);
        }
        strcpy(pid_file, optarg);
        break;

      case 'r':
        delay_random = atoi(optarg);
        if (delay_random < 0 || delay_random > 60) {
          fprintf(stderr, "-r=%d ??? Random inter-reporting delay must be an integer from 0 to 60 seconds\n", delay_random);
          exit(EXIT_FAILURE);
        }
        break;

      case '?':
      case 'h':
      default: /* '?' */
        fprintf(stderr, "Usage: %s [-d] [-t seconds] [-T seconds] [-p /path/to.pid] [-r seconds] host[:port]\n\
opts:\n\
    -d: Debug mode. Don't daemonize, use stderr instead of syslog.\n\
    -t: Timeout between reports. Default is 15 seconds.\n\
    -T: Timeout before first report. Default is 25 seconds.\n\
    -p: Write pid-file (and global lock) at this path. Default is \"/run/ipaddrd.pid\"\n\
    -r: Randomly shift inter-reporting delay, max to this value in seconds. Default is 1 second.\n\
args:\n\
  host: Send reports to this url|ip (via UDP)\n\
  port: Send reports to this udp port. Default is 3333.\n",
              (*argv)[0]);
        exit(EXIT_FAILURE);
    }
  }

  if (optind >= argc) {
    fprintf(stderr, "Expected host[:port] after options\n");
    exit(EXIT_FAILURE);
  }

  if (argc > optind + 1) {
    fprintf(stderr, "Too many args. Expected host[:port] and no more.\n");
    exit(EXIT_FAILURE);
  }

  server.host = (*argv)[optind];
  server.port = strtok(server.host, ":");
  server.port = strtok(NULL, ":");
  if (server.port == NULL) {
    server.port = "3333";
  }

  if ((atoi(server.port) < 1) || (atoi(server.port) > 65535)) {
    fprintf(stderr, "port=%d ??? Port must be the valid int (1..65535)\n", atoi(server.port));
    exit(EXIT_FAILURE);
  }

// Default pid_file
  if (pid_file == NULL) {
    pid_file = malloc(sizeof(char) * sizeof(PID_FILE));
    if (pid_file == NULL) {
      fprintf(stderr, "Panic: malloc((pid_file) size = 0x%x failed (no free mem?)!\n", sizeof(PID_FILE));
      exit(EXIT_FAILURE);
    }
    strcpy(pid_file, PID_FILE);
  }
}


// Collect packed ip's into (further udp) frame.
int ipaddr(unsigned char **frame, unsigned char **frame_pos, uint16_t *ip_count) // Returns 1 in case of out-of-buffer
{
  int res = 1;
  struct ifaddrs *ifaddr, *ifa;
  int family, s, n;
  char host[NI_MAXHOST];

  const struct sockaddr *sa;

  if (getifaddrs(&ifaddr) == -1) {
    perror("getifaddrs");
    exit(EXIT_FAILURE);
  }

  /* Walk through linked list, maintaining head pointer so we
     can free list later */

  for (ifa = ifaddr, n = 0; ifa != NULL; ifa = ifa->ifa_next, n++) {
    if (ifa->ifa_addr == NULL)
      continue;

    sa = ifa->ifa_addr;
    family = ifa->ifa_addr->sa_family;

    if (family == AF_INET || family == AF_INET6) {
      if (!( ifa->ifa_flags & IFF_POINTOPOINT)) { // Except tunnels
        if (family == AF_INET) { // IPv4, 4 bytes
          const struct sockaddr_in *sinp = (const struct sockaddr_in *) sa;
          uint32_t ipv4 = sinp->sin_addr.s_addr;
          if ( (ipv4 & 0xff) != 127 ) {
            if ( (*frame_pos - *frame) > (BUFFER_SIZE - sizeof(struct ipv4_frame)) ) { // Why so many ip's???
              goto finally;
            }
            char head = (ifa->ifa_flags & IFF_UP > 0) << 1;
            head |= (ifa->ifa_flags & IFF_BROADCAST > 0);
            head <<= 1;
            head |= (ifa->ifa_flags & IFF_PROMISC > 0);
            head <<= 1;
            struct ipv4_frame * IPADDR;
            IPADDR = (struct ipv4_frame *) *frame_pos;
            IPADDR->head = head;
            IPADDR->ip = ipv4;
            *frame_pos += sizeof(struct ipv4_frame);
            (*ip_count) ++;
          }
        }

        if (family == AF_INET6) { // IPv6, 16 bytes
          const struct sockaddr_in6 *sin6p = (const struct sockaddr_in6 *) sa;
          uint32_t (*IPv6)[4] = (uint32_t (*)[4]) & sin6p->sin6_addr;

//          if ( (strncmp(ipv6, "\xfe\x80", 2) != 0) && (ip_3 == 0) ) {
          if (!(((*IPv6)[0] | (*IPv6)[1] | (*IPv6)[2]) == 0 && (*IPv6)[3] == 0x01000000) && !((*IPv6)[0] == 0x000080fe)) {
            if ( (*frame_pos - *frame) > (BUFFER_SIZE - sizeof(struct ipv6_frame)) ) { // Why so many ip's???
              goto finally;
            }
            char head = (ifa->ifa_flags & IFF_UP > 0) << 1;
            head |= (ifa->ifa_flags & IFF_BROADCAST > 0);
            head <<= 1;
            head |= (ifa->ifa_flags & IFF_PROMISC > 0);
            head <<= 1;
            head |= 1; // Last bit = 1 for IPv6 header
            struct ipv6_frame * IPADDR;
            IPADDR = (struct ipv6_frame *) *frame_pos;
            IPADDR->head = head;
            IPADDR->ip[0] = (*IPv6)[0];
            IPADDR->ip[1] = (*IPv6)[1];
            IPADDR->ip[2] = (*IPv6)[2];
            IPADDR->ip[3] = (*IPv6)[3];
            *frame_pos += sizeof(struct ipv6_frame);
            (*ip_count) ++;
          }
        }
      }
    }

    continue;
  }
  res = 0;

finally:
  freeifaddrs(ifaddr);
  return res;
}

// Prepare the frame to be sent via udp. << pthread_create( void *myThreadFun(void *vargp) )
void *ipframe(void * param) // Returns 1 in case of out-of-buffer
{
  int res = 1;
  struct _fp *fp = (struct _fp *) param;

  DIR *d;
  char * d_path;
  int len = NAME_MAX;
  struct dirent *dir;
  int fd;

  // "root" netns
  if ( res = ipaddr(&(fp->frame), &(fp->frame_pos), &(fp->ip_count)) ) goto finally;
  if (debug)
      fprintf(stdout, "ipframe: root ip_count=%d\n", fp->ip_count);

  d = opendir("/var/run/netns");

  if (d) { // iterate through namespaces
    d_path = malloc(sizeof(char) * len);
    if (d_path == NULL) {
      printf("Panic: malloc(\"/var/run/netns/...\") failed (no free mem?)!\n");
      exit(EXIT_FAILURE);
    }

    while ((dir = readdir(d)) != NULL) {
      if(dir -> d_type == DT_REG) { // http://www.gnu.org/software/libc/manual/html_node/Directory-Entries.html
        if (sizeof("/var/run/netns") + strlen( dir->d_name) > len) { // WTF??? Why it is so long?
          int new_len = sizeof("/var/run/netns") + strlen( dir->d_name);
          d_path = realloc(d_path, new_len);
          if (d_path == NULL) {
            printf("Panic: realloc(\"/var/run/netns/...\") size = 0x%x failed (no free mem?)!\n", new_len);
            exit(EXIT_FAILURE);
          }
          len = new_len;
        }
        sprintf(d_path, "%s/%s", "/var/run/netns", dir->d_name);

        fd = open(d_path, O_RDONLY); /* Get file descriptor for namespace */
        if (fd == -1) { // Unable to get netns descriptor? Though, not fatal...
          continue;
        }

        if (setns(fd, 0) == 0) { /* Join that namespace */
          close(fd);
          if ( res = ipaddr(&(fp->frame), &(fp->frame_pos), &(fp->ip_count)) ) {
            closedir(d); // !!!
            free(d_path);
            goto finally;
          }
        if (debug)
          fprintf(stdout, "ipframe: \"%s\" ip_count=%d\n", dir->d_name, fp->ip_count);
        }
      }
    }
    closedir(d);
    free(d_path);
  }
  res = 0;

finally:
  fp->result = res;
}

void send_frame(unsigned char **frame, int frame_total)
{
  struct addrinfo hints; // http://man7.org/linux/man-pages/man3/getaddrinfo.3.html
  struct addrinfo *result, *rp;
  int sfd;

  memset(&hints, '\0', sizeof(struct addrinfo));
  hints.ai_family = AF_UNSPEC;    /* Allow IPv4 or IPv6 */
  hints.ai_socktype = SOCK_DGRAM; /* Datagram socket */
  hints.ai_flags = AI_ADDRCONFIG; // IPv4/6 addresses are returned only if at least one IPv4/6 address configured
//           hints.ai_flags = 0;
//           hints.ai_protocol = 0;          /* Any protocol */

  int s = getaddrinfo(server.host, server.port, &hints, &result);
  if (s != 0) {
    if (debug) {
      fprintf(stderr, "send_frame: getaddrinfo(\"%s:%s\"): %s\n", server.host, server.port, gai_strerror(s));
    } else {
      syslog(LOG_ERR, "send_frame: getaddrinfo(\"%s:%s\"): %s\n", server.host, server.port, gai_strerror(s));
    }
    return;
  }

  /* getaddrinfo() returns a list of address structures.
     Try each address until we successfully connect(2).
     If socket(2) (or connect(2)) fails, we (close the socket
     and) try the next address. */
  for (rp = result; rp != NULL; rp = rp->ai_next) {
    sfd = socket(rp->ai_family, rp->ai_socktype,
                 rp->ai_protocol);
    if (sfd == -1) continue;

    int err;
    if (err = connect(sfd, rp->ai_addr, rp->ai_addrlen) != -1)
      break;                  /* Success */
    else
      if (debug)
        fprintf(stderr, "send_frame: connect() error: %s %x\n", gai_strerror(err), errno);

    close(sfd);
  }

  if ((rp == NULL) && debug) {               /* No address succeeded */
    fprintf(stderr, "send_frame: Could not connect to server\n");
    goto finally;
  }

  int written = write(sfd, *frame, frame_total);
  close(sfd);

  if ((written != frame_total) && debug) {
    fprintf(stderr, "send_frame: partial/failed write\n");
    goto finally;
  }

finally:
  freeaddrinfo(result);
}


void *lock(void * param)
{
  int *fd_lock = (int *) param;
  flock(*fd_lock, LOCK_EX);
}

int try_lock()
{
  int fd_lock = open(pid_file, O_CREAT | O_RDWR, 0600);
  if (fd_lock < 0) {
    fprintf(stderr, "Unable to create/open pid-file using path: \"%s\"\n", pid_file);
    exit(EXIT_FAILURE);
  }

  struct timespec ts;
  if (clock_gettime(CLOCK_REALTIME, &ts) == -1) {
    exit(EXIT_FAILURE);
  }
  ts.tv_sec += 1;

  pthread_t tid; // Thread ID
  pthread_attr_t attr; // Thread attrs
  pthread_attr_init(&attr);

  pthread_create(&tid, &attr, lock, &fd_lock);
  if (pthread_timedjoin_np(tid, NULL, &ts) != 0) {
    return -1;
  }
  return fd_lock;
}


static void daemonize()
{
  fd_lock = try_lock(); // Allow only one running instance
  if (fd_lock < 0) {
    fprintf(stderr, "Another instance already running!\n");
    exit(EXIT_FAILURE);
  }

  pid_t pid;

  /* Fork off the parent process */
  pid = fork();

  /* An error occurred */
  if (pid < 0)
    exit(EXIT_FAILURE);

  /* Success: Let the parent terminate */
  if (pid > 0)
    exit(EXIT_SUCCESS);

  /* On success: The child process becomes session leader */
  if (setsid() < 0)
    exit(EXIT_FAILURE);

  /* Catch, ignore and handle signals */
  signal(SIGCHLD, SIG_IGN);
  signal(SIGHUP, SIG_IGN);

  /* Fork off for the second time*/
  pid = fork();

  /* An error occurred */
  if (pid < 0)
    exit(EXIT_FAILURE);

  /* Success: Let the parent terminate */
  if (pid > 0)
    exit(EXIT_SUCCESS);

  dprintf(fd_lock, "%d\n", getpid()); // output to a file descriptor fd instead of to a stdio stream

  /* Set new file permissions */
  umask(0);

  /* Change the working directory to the root directory */
  /* or another appropriated directory */
  chdir("/");

  /* Close all open file descriptors */
  for (int x = sysconf(_SC_OPEN_MAX); x >= 0; x--) {
    if (x != fd_lock)
      close (x);
  }
}


void term(int signum)
{
  if (!debug)
    syslog(LOG_NOTICE, "Daemon terminated by signal %d \"%s\"", signum, strsignal(signum));
  else
    fprintf(stdout, "Debug: terminated by signal %d \"%s\"\n", signum, strsignal(signum));

  terminated = 1;
}


int main(int argc, char *argv[])
{
  cli(argc, &argv);
  if (debug)
    fprintf(stdout, "Debug: delay_report=%d delay_start=%d delay_random=%d pid_file=\"%s\"\n", delay_report, delay_start, delay_random, pid_file);

  struct _fp fp;
  struct timespec delay_time;

  fp.frame = malloc(sizeof(char) * BUFFER_SIZE);
  if (fp.frame == NULL) {
    fprintf(stderr, "Panic: malloc(UDP frame) failed (no free mem?)!\n");
    exit(EXIT_FAILURE);
  }

  if (!debug)
    daemonize();

  // Handle (SIGTERM|SIGINT) https://airtower.wordpress.com/2010/06/16/catch-sigterm-exit-gracefully/
  struct sigaction action;
  memset(&action, 0, sizeof(struct sigaction));
  action.sa_handler = term;
  sigaction(SIGTERM, &action, NULL);
  sigaction(SIGHUP, &action, NULL);
  sigaction(SIGINT, &action, NULL);

  char hostname[HOST_NAME_MAX + 1];
  hostname[HOST_NAME_MAX] = '\0'; // Fuser
  gethostname(hostname, HOST_NAME_MAX);
  u_int hostname_len = strlen(hostname);
//  printf("hostname: \"%s\"\n", hostname);

  // Session UUID
  uuid_t uuid; // typedef unsigned char uuid_t[16];
  uuid_generate(uuid);
  // ??? M.b. no need in: RNG is already initialized while building uuid.
  srand(*(unsigned long *) & uuid); // Init RNG from the head of obtained uuid.

  // unparse (to string)
  char uuid_str[37];      // ex. "1b4e28ba-2fa1-11d2-883f-0016d3cca427" + "\0"
  uuid_str[36] = '\0'; // Fuser
  uuid_unparse_lower(uuid, uuid_str);

  if (!debug) {
    openlog("ipaddrd", LOG_PID, LOG_DAEMON);
    syslog(LOG_NOTICE, "Daemon started, uuid=%s", uuid_str);
  } else
    fprintf(stdout, "Debug: started, uuid=%s, server=%s:%s\n", uuid_str, server.host, server.port);

  delay_time.tv_sec = delay_start;
  delay_time.tv_nsec = 0;
  nanosleep(&delay_time, NULL); // http://man7.org/linux/man-pages/man2/nanosleep.2.html

  while (!terminated) {
    fp.ip_count = 0;
    fp.result = 0;
    fp.frame_pos = fp.frame + sizeof(uint16_t) + sizeof(uint16_t); // Current write position. Header will be (frame_length, ip_count).

  // !!! ipframe() must be executed in cloned thread, because of setns() is one-way ticket.
  //  int res = ipframe(&frame, &frame_pos);
  // http://man7.org/linux/man-pages/man2/clone.2.html
  //  CLONE_FILES|CLONE_IO|CLONE_PTRACE| (CLONE_THREAD|CLONE_SIGHAND|CLONE_VM) ... shit, why just not to use pthread?
    pthread_t tid; // Thread ID
    pthread_attr_t attr; // Thread attrs
    pthread_attr_init(&attr);

    pthread_create(&tid, &attr, ipframe, &fp);
    pthread_join(tid, NULL); // Wait

    int frame_size = fp.frame_pos - fp.frame;
    int frame_total = frame_size + hostname_len + 1 + sizeof(uuid_t);
    if ( frame_total > BUFFER_SIZE ) {
      fp.frame = realloc(fp.frame, (frame_total + sizeof(unsigned long))); // + *Full* len(crc32) https://github.com/madler/zlib/blob/master/crc32.c
      if (fp.frame == NULL) {
        fprintf(stderr, "Panic: realloc((UDP frame) size = 0x%x failed (no free mem?)!\n", frame_total);
        exit(EXIT_FAILURE);
      }
      fp.frame_pos = fp.frame + frame_size;
    }
    *(fp.frame_pos) = fp.result; // = 1 in case of not all ip's got into the buffer.
    fp.frame_pos ++;
    memcpy(fp.frame_pos, hostname, hostname_len);
    fp.frame_pos += hostname_len;

    // Header: (length, ip_count)
    uint16_t (*head)[2] = (uint16_t (*)[2]) fp.frame;
    (*head)[0] = (uint16_t) (frame_total + sizeof(uint32_t)); // + uuid + *Real* len(crc32) = 4bytes
    (*head)[1] = fp.ip_count;

    memcpy(fp.frame_pos, uuid, sizeof(uuid_t));
    fp.frame_pos += sizeof(uuid_t);

    unsigned long *crc = (unsigned long *) fp.frame_pos;
    *crc = crc32(0L, Z_NULL, 0); // https://refspecs.linuxbase.org/LSB_3.0.0/LSB-Core-generic/LSB-Core-generic/zlib-crc32-1.html
    *crc = crc32(*crc, fp.frame, frame_total);

    send_frame(&(fp.frame), (frame_total + sizeof(uint32_t))); // + *Real* len(crc32) = 4bytes
    if (debug)
      fprintf(stdout, "send_frame: ip_count = %d, frame_total = %d\n", fp.ip_count, frame_total + sizeof(uint32_t));

    unsigned long delay_ns = delay_random * (rand() % 1000000000);
    delay_time.tv_sec = delay_report + (delay_ns / 1000000000);
    delay_time.tv_nsec = delay_ns % 999999999;
    if (debug)
      fprintf(stdout, "nanosleep: delay = %d.%d seconds\n", delay_time.tv_sec, delay_time.tv_nsec);
    nanosleep(&delay_time, NULL);
  }

  // SIGTERM|SIGINT|SIGHUP
  free(fp.frame);

  if (!debug)
    closelog();

  close(fd_lock);
  unlink(pid_file);
  exit(EXIT_SUCCESS);
}
