#!/usr/bin/python -u
# -*- coding: utf-8 -*-
import cgi
from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
#import subprocess
from subprocess import Popen, PIPE
import sys
import syslog
from socket import *

import psycopg2
import psycopg2.extras
from psycopg2.extensions import adapt

HOST = '127.0.0.1'
PORT = 5038
USER = 'clear_sip'
PASS = 'sip_clear'

def SipPeers():
  try:
    # Login to AMI
    ast = socket(AF_INET, SOCK_STREAM)
  #  ast.settimeout(1)
    ast.connect((HOST, PORT))
    data = ""
    while "\r\n" not in data:
      data += ast.recv(1500)
    #print repr(data)

    params = ["Action: login",
              "Events: off",
              "Username: %s" % USER,
              "Secret: %s" % PASS]

    ast.send("\r\n".join(params) + "\r\n\r\n")
    # receive login response
    data = ""
    while "\r\n\r\n" not in data:
      data += ast.recv(1024)
  #    print data
    data = ""

    params = ["Action: SIPpeers",
#              "Peer:: %s" % Name,
              "ActionID: 001"]

    ast.send("\r\n".join(params) + "\r\n\r\n")
    # receive answer
    while "PeerlistComplete" not in data:
      data += ast.recv(1024)
#      print data
#    data = ""

    ast.send("Action: Logoff\r\n\r\n")
    ast.close()
  except error, E:
#    syslog.syslog("AMI ERROR: " + str(E))
    return "ERROR AMI: %s" % (str(E))

#  syslog.syslog("OK QueueStatus: %s" % Queue)
  return data

def SipPeer(Name):
  try:
    # Login to AMI
    ast = socket(AF_INET, SOCK_STREAM)
  #  ast.settimeout(1)
    ast.connect((HOST, PORT))
    data = ""
    while "\r\n" not in data:
      data += ast.recv(1500)
    #print repr(data)

    params = ["Action: login",
              "Events: off",
              "Username: %s" % USER,
              "Secret: %s" % PASS]

    ast.send("\r\n".join(params) + "\r\n\r\n")
    # receive login response
    data = ""
    while "\r\n\r\n" not in data:
      data += ast.recv(1024)
  #    print data
    data = ""

    params = ["Action: SIPshowpeer",
              "Peer: %s" % Name,
              "ActionID: 001"]

    ast.send("\r\n".join(params) + "\r\n\r\n")
    # receive answer
    while "\r\n\r\n" not in data:
      data += ast.recv(1024)
#      print data
#    data = ""

    ast.send("Action: Logoff\r\n\r\n")
    ast.close()
  except error, E:
#    syslog.syslog("AMI ERROR: " + str(E))
    return "ERROR AMI: %s" % (str(E))

#  syslog.syslog("OK QueueStatus: %s" % Queue)
  return data

def SipList(BIND):
  try:
    connect = psycopg2.connect("dbname='pbx' user='postgres' password=''")
  except:
    return ['', 'SQL ERROR: Can not connect to database', '']

  cur = connect.cursor() #(cursor_factory=psycopg2.extras.DictCursor)

  sql = """select name, fullcontact from sip
           where ("BIND" = %s) order by name;""" % adapt(BIND)

#  print sql

  res = []
  body = ''
  try:
    cur.execute(sql)
    if (cur.rowcount > 0):
      res.append(cur.rowcount)
      body = 'name;fullcontact;status\n'
      for row in cur.fetchall():
        data = SipPeer(row[0]).splitlines()
        status = ''
        for l in data:
          if (l[:7] == 'Status:'):
            status = l[8:]
            break
        body += "%s;%s;%s\n" % (row[0], row[1], status)
    else:
      res = [0, 'SIP accounts not found']
  except psycopg2.Error, e:
    res = ['', "SQL ERROR: " + str(e.pgerror)]

  cur.close()
  return(res, body)


def SipLog(BIND, sip):
#
  pipe = Popen("""/usr/local/sbin/sip-log '%s+%s'""" % (BIND, sip), shell=True, stdin=PIPE, stdout=PIPE, stderr=PIPE)

  stdout, stderr = pipe.communicate()
  result = pipe.returncode

  pipe.stdin.close()
  pipe.wait()

  return [ "%s" % result, stdout ]
