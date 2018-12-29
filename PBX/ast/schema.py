#!/usr/bin/python -u
# -*- coding: utf-8 -*-
import cgi
from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
#import subprocess
from subprocess import Popen, PIPE
import sys
import syslog
from socket import *
from datetime import datetime

import psycopg2
import psycopg2.extras
from psycopg2.extensions import adapt

import os, errno

import urllib

from schema.sip import *
#print SipPeer('3570+702')
#print SipList('3570')
#sys.exit(0)

LISTEN_PORT = 8086
#HOST = '127.0.0.1'
HOST = '0.0.0.0'

class APIError(Exception):
  def __init__(self, value):
    self.value = value
  def __str__(self):
    return repr(self.value)


def Login(BIND, user, password):
  try:
    connect = psycopg2.connect("dbname='pbx' user='postgres' password=''")
  except:
#    syslog.syslog('SQL ERROR: Can not connect to database')
    return ['', 'SQL ERROR: Can not connect to database', '']

  cur = connect.cursor() #(cursor_factory=psycopg2.extras.DictCursor)

  sql = """select "NRec", "ReadOnly"::int, COALESCE("Description",'') from "User"
           where ("BIND" = %s)
             and ("Name" = %s)
             and "Enabled"
             and (now() between "ValidSince" and "ValidTill")
             and ("Pass" = crypt(%s, "Pass"));""" % (adapt(BIND),adapt(user),adapt(password))

#  print sql

  res = []
  try:
    cur.execute(sql)
    if (cur.rowcount > 0):
      row = cur.fetchone()
      res = row
    else:
      res = ['', 'Login failed', '']
  except psycopg2.Error, e:
#    syslog.syslog("SQL ERROR: " + str(e.pgerror))
    res = ['', "SQL ERROR: " + str(e.pgerror), '']

  cur.close()
  return(res)


def Phones(BIND):
  try:
    connect = psycopg2.connect("dbname='pbx' user='postgres' password=''")
  except:
#    syslog.syslog('SQL ERROR: Can not connect to database')
    return ['', 'SQL ERROR: Can not connect to database']

  cur = connect.cursor() #(cursor_factory=psycopg2.extras.DictCursor)

  sql = """select "Phone", "Locked"::int from "PhoneBIND"
           where ("BIND" = %s)
             and (now() between "ValidSince" and "ValidTill");""" % adapt(BIND)

#  print sql

  res = []
  try:
    cur.execute(sql)
    if (cur.rowcount > 0):
      res.append(cur.rowcount)
      for row in cur.fetchall():
       res.append("%s:%s" % (row[0], row[1]))
    else:
      res = [0, 'Phones not found']
  except psycopg2.Error, e:
#    syslog.syslog("SQL ERROR: " + str(e.pgerror))
    res = ['', "SQL ERROR: " + str(e.pgerror)]

  cur.close()
  return(res)


def Phone(BIND, Phone):
  try:
    connect = psycopg2.connect("dbname='pbx' user='postgres' password=''")
  except:
#    syslog.syslog('SQL ERROR: Can not connect to database')
    return -3

  cur = connect.cursor() #(cursor_factory=psycopg2.extras.DictCursor)

  sql = """select (NOT "Locked")::int from "PhoneBIND"
           where ("BIND" = %s)
             and ("Phone" = %s)
             and (now() between "ValidSince" and "ValidTill");""" % (adapt(BIND), adapt(Phone))

#  print sql

  res = -1
  try:
    cur.execute(sql)
    if (cur.rowcount > 0):
      res = cur.fetchone()[0]
  except psycopg2.Error, e:
    res = -2

  cur.close()
  return(res)


def Get(BIND,phone,id):
  if (id == 0):
    id = ''
  else:
    id = ".%s" % id

  path = "/pbx/www/%s/%s.cfg%s" % (BIND,phone,id)
  try:
    with open(path, 'r') as f:
      return f.read()
  except:
    return ''

def Put(BIND,phone,id,content):
  if (id == 0):
    suffix = ''
  else:
    suffix = ".%s" % id

  path = "/pbx/www/%s/%s.cfg" % (BIND,phone)

  try:
    os.makedirs("/pbx/www/%s" % BIND)
  except OSError as E:
    if E.errno == errno.EEXIST and os.path.isdir("/pbx/www/%s" % BIND):
      pass
    else:
      return [ -1 ]

  if (id < 10):
    for i in range (9, id-1, -1):
      try:
        if (i > 0):
          os.rename("%s.%s" % (path,i), "%s.%s" % (path,i+1))
        else:
          os.rename("%s" % path, "%s.1" % path)
      except OSError, e:
#        print e
        pass

  path = "%s%s" % (path,suffix)

  try:
    with open(path, 'w') as f:
      f.write(content)
    return [ 1 ]
  except:
    return [ 0 ]


def Test(BIND,phone):
  root = "[* %s %s] ; TEST" % (phone, BIND)
  path = "/pbx/www/%s/%s.cfg" % (BIND,phone)

  pipe = Popen("""(echo "%s"
cat "%s") | schema """ % (root, path),
    shell=True, stdin=PIPE, stdout=PIPE, stderr=PIPE)

  stdout, stderr = pipe.communicate()
  result = pipe.returncode

  pipe.stdin.close()
  pipe.wait()

  if (result > 0):
    return [ [-result], stderr ]


  pipe = Popen("""(echo "%s"
cat "%s") | schema | upsert -TCb '%s'""" % (root, path, BIND),
    shell=True, stdin=PIPE, stdout=PIPE, stderr=PIPE)

  stdout1, stderr1 = pipe.communicate()
  result = pipe.returncode

  pipe.stdin.close()
  pipe.wait()

#  print result, stderr, stdout
  return [ [result], "%s\n\n%s" % (stdout, stderr1) ]


def Apply(BIND,phone):
  root = "[* %s %s] ; TEST" % (phone, BIND)
  path = "/pbx/www/%s/%s.cfg" % (BIND,phone)

  pipe = Popen("""(echo "%s"
cat "%s") | schema | upsert -TCb '%s' | psql -U postgres pbx""" % (root, path, BIND),
    shell=True, stdin=PIPE, stdout=PIPE, stderr=PIPE)

  stdout, stderr = pipe.communicate()
  result = pipe.returncode

  err = ''
  for f in stderr.splitlines():
    err += "%s\n" % f
    if (f[:13] == 'ОШИБКА:'):
      result = -1
      break


  pipe.stdin.close()
  pipe.wait()

#  print result, stderr, stdout
  return [ [result], err ]


def Delete(BIND,phone):
  root = "[* %s %s] ; TEST" % (phone, BIND)
  path = "/pbx/www/%s/%s.cfg" % (BIND,phone)

  pipe = Popen("""(echo "%s"
cat "%s") | schema | upsert -TPb '%s' | psql -U postgres pbx""" % (root, path, BIND),
    shell=True, stdin=PIPE, stdout=PIPE, stderr=PIPE)

  stdout, stderr = pipe.communicate()
  result = pipe.returncode

  err = ''
  for f in stderr.splitlines():
    err += "%s\n" % f
    if (f[:13] == 'ОШИБКА:'):
      result = -1
      break


  pipe.stdin.close()
  pipe.wait()

#  print result, stderr, stdout
  return [ [result], err ]


def Drop(BIND,phone):
  path = "/pbx/www/%s/%s.cfg" % (BIND,phone)

  for i in range (10, 1, -1):
    try:
      os.unlink("%s.%s" % (path,i))
    except OSError, e:
      pass

  try:
    os.unlink(path)
    return [ 1 ]
  except:
    return [ 0 ]


""" REST HTTP server (GET,PUT)
"""
class MyHandler(BaseHTTPRequestHandler):
  def do_GET(self):
    try:
      params = {}
      result = ''
      res = []
      response = 200
      body = ''

      try:
        query = self.path.split('?')[1].split('&')
        for r in query:
          try:
            params[ r.split('=')[0] ] = r.split('=')[1]
          except:
            continue
        print params

        action = params["action"]
        BIND = params["bind"]
        BIND = "".join(i for i in BIND if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        user = params["user"]
        user = "".join(i for i in user if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        password = urllib.unquote(params["pass"])

        res = Login(BIND, user, password)

        if (res[0] == ''): action = 'login' # auth is mandatory
        ro = res[1] # ReadOnly
      except:
        action = 'NonExistent'
        res = [ '', 'action&bind&user&pass are required!' ]

      if (action == 'login'): # Check whether auth succeeds
        if (res[0] == ''):
          response = 403

      elif (action == 'phones'): # List all phones, available for USER
        res = Phones(BIND)

      elif (action == 'get'): # Get current (id=0) or backup config
        try:
          phone = params["phone"]
          phone = "".join(i for i in phone if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        except:
          raise APIError(412) # "Precondition Failed"

        if (Phone(BIND,phone) < 1): # Foreign phone!!!
          raise APIError(412) # "Precondition Failed"

        try:
          id = int(params["id"])
          if (id > 10): id = 10
          if (id < 0): id = 0
        except:
          id = 0

        res = [ "%s;%s" % (phone, id) ]
        body = Get(BIND,phone,id)

      elif (action == 'test'): # Compilation test for current config
        try:
          phone = params["phone"]
          phone = "".join(i for i in phone if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        except:
          raise APIError(412) # "Precondition Failed"

        if (Phone(BIND,phone) < 1): # Foreign phone!!!
          raise APIError(412) # "Precondition Failed"

        [res, body] = Test(BIND,phone)

      elif (action == 'sip'): # SIP peers monitoring
        try:
          cmd = params["cmd"]
        except:
          raise APIError(412) # "Precondition Failed"

        if (cmd == 'list'): # Show all peers, stored for this BIND, together with current status
          [res, body] = SipList(BIND)
        elif (cmd == 'log'): # Get log for particular peer
          try:
            sip = params["sip"]
          except:
            raise APIError(412) # "Precondition Failed"
          [res, body] = SipLog(BIND,sip)
        else:
          response = 412 # "Precondition Failed"

      elif res[1]: # ReadOnly
        response = 403

      elif (action == 'apply'): # Merge changes into PBX
        try:
          phone = params["phone"]
          phone = "".join(i for i in phone if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        except:
          raise APIError(412) # "Precondition Failed"

        if (Phone(BIND,phone) < 1): # Foreign|locked phone!!!
          raise APIError(412) # "Precondition Failed"

        [res, body] = Apply(BIND,phone)

      elif (action == 'delete'): # Delete current config from PBX
        try:
          phone = params["phone"]
          phone = "".join(i for i in phone if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        except:
          raise APIError(412) # "Precondition Failed"

        if (Phone(BIND,phone) < 1): # Foreign|locked phone!!!
          raise APIError(412) # "Precondition Failed"

        [res, body] = Delete(BIND,phone)

      elif (action == 'drop'): # Drop from PBX all objects for this USER's PHONE
        try:
          phone = params["phone"]
          phone = "".join(i for i in phone if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        except:
          raise APIError(412) # "Precondition Failed"

        if (Phone(BIND,phone) < 1): # Foreign|locked phone!!!
          raise APIError(412) # "Precondition Failed"

        [res, body] = Drop(BIND,phone)
      else:
        response = 404

      for r in res:
        result += "%s;" % r
      result = result.rstrip(';')

      self.send_response(response)
      self.send_header("Result", result)
      self.send_header("Content-type", "text/plain")
      self.send_header('Content-Length', len(body))
      self.end_headers()
      self.wfile.write(body)
      self.wfile.close()

    except APIError, e:
      self.send_response(e.value)
      self.end_headers()
    except Exception, e:
      self.send_response(500)
      print e
      if not e:
        self.wfile.write("%s") % e
      self.end_headers()
#      pass

  def do_PUT(self):
    try:
      params = {}
      result = ''
      res = []
      response = 200
      body = ''

      try:
        query = self.path.split('?')[1].split('&')
        for r in query:
          try:
            params[ r.split('=')[0] ] = r.split('=')[1]
          except:
            continue
        print params

        action = params["action"]
        BIND = params["bind"]
        BIND = "".join(i for i in BIND if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        user = params["user"]
        user = "".join(i for i in user if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        password = urllib.unquote(params["pass"])

        res = Login(BIND, user, password)

        if (res[0] == ''): action = 'login' # auth is mandatory
        ro = res[1] # ReadOnly
      except:
        action = 'NonExistent'
        res = [ '', 'action&bind&user&pass are required!' ]

      if (action == 'login'): # Check whether auth succeeds
        if (res[0] == ''):
          response = 403

      elif res[1]: # ReadOnly
        response = 403

      elif (action == 'put'): # Push config, shifting old (id=1..9)
        try:
          l = int(self.headers.getheader('Content-Length'))
        except:
          raise APIError(411) # "Length Required"

        if l > 128000: # Max 1000 lines x 128 chars
          raise APIError(413) # "Request Entity Too Large"

        try:
          phone = params["phone"]
          phone = "".join(i for i in phone if i not in "/\\| ~!$%^&(){}[]*#?<>\t\n'`\";:,")
        except:
          raise APIError(412) # "Precondition Failed"

        if (Phone(BIND,phone) < 1): # Foreign phone!!!
          raise APIError(412) # "Precondition Failed"

        try:
          id = int(params["id"])
          if (id > 10): id = 10
          if (id < 0): id = 0
        except:
          id = 0

        file = self.rfile.read(l)
        cnt = 0
        for f in file.splitlines():
          cnt += 1
          if (cnt > 1000): raise APIError(413)

        res = Put(BIND,phone,id,file)

      else:
        response = 404

      for r in res:
        result += "%s;" % r
      result = result.rstrip(';')

      self.send_response(response)
      self.send_header("Result", result)
#      self.send_header("Content-type", "text/plain")
#      self.send_header('Content-Length', len(body))
      self.end_headers()
#      self.wfile.write(body)
      self.wfile.close()

    except APIError, e:
      self.send_response(e.value)
      self.end_headers()
    except Exception, e:
      self.send_response(500)
      print e
      if not e:
        self.wfile.write("%s") % e
      self.end_headers()
#      pass


  def do_POST(self):
    self.send_error(404,"Not Found")

  def do_DEL(self):
    self.send_error(404,"Not Found")

#print Login('VOIX','punk','fuck')
#print Get('VOIX','8123303822',0)
#print Phone('VOIX','8123303822')
#sys.exit(0)

if __name__ == "__main__":
     try:
         syslog.openlog("SCHEMA", syslog.LOG_PID|syslog.LOG_CONS, syslog.LOG_LOCAL0)
         server = HTTPServer((HOST, LISTEN_PORT), MyHandler)
         print "started httpserver..."
         syslog.syslog('SCHEMA interface started')
         server.serve_forever()

     except KeyboardInterrupt:
         print "^C received, shutting down server"
         server.socket.close()
