There is placed a number of valuable asterisk patches we done.

app_queue: 7 new properties are implemented, to accomodate it for wery long call queues.
  Added "sayposition" argument to Queue application.
  Special thanks to Mark Spencer for the excellent readable code.
My patches are not so clean, sorry.

chan_sip: "tel:" uri implementation, based on https://issues.asterisk.org/jira/browse/ASTERISK-17179

cdr_pgsql: dumb patch for dumb code. Hardcoded SQL-string max-size is too low to store call-pass trace in cdr.
