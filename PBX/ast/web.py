#!/usr/bin/python
# -*- coding: utf-8 -*-
# TODO: action=rl&check=1

import cgi
from os import curdir, sep
from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
import subprocess
import sys

import syslog

import psycopg2
import psycopg2.extras
from psycopg2.extensions import adapt

BIND='4664'

def BlackList(BIND,Line,CID,Description,enable):
  syslog.syslog('BlackList(%s, %s, %s, %s, %s)' % (BIND, Line, CID, Description, enable))
  try:
    connect = psycopg2.connect("dbname='pbx' user='asterisk' password=''")
  except:
    syslog.syslog('SQL ERROR: Can not connect to database')
    return(-1)

  cur = connect.cursor(cursor_factory=psycopg2.extras.DictCursor)

  if (enable):
    sql = """INSERT INTO "BlackList"("BIND","Line","CID","Description") VALUES (%s, %s, %s, %s)""" % (adapt(BIND), adapt(Line), adapt(CID), adapt(Description))
  else:
    sql = """DELETE FROM "BlackList" WHERE ("BIND" = %s) and ("Line" = %s) and ("CID" = %s)""" % (adapt(BIND), adapt(Line), adapt(CID))

  syslog.syslog(sql)

  try:
    cur.execute(sql)
    connect.commit()
    res = cur.rowcount
    cur.close()
    return(res)
  except psycopg2.Error, e:
    syslog.syslog("SQL ERROR: " + str(e.pgerror))

  return(-2)

def BlackListCheck(BIND,Line,CID):
  syslog.syslog('BlackListCheck(%s, %s, %s)' % (BIND, Line, CID))
  try:
    connect = psycopg2.connect("dbname='pbx' user='asterisk' password=''")
  except:
    syslog.syslog('SQL ERROR: Can not connect to database')
    return(-1)

  cur = connect.cursor(cursor_factory=psycopg2.extras.DictCursor)

  sql = """SELECT "Line" FROM "BlackList" WHERE ("BIND" = %s) and ("Line" = %s) and ("CID" = %s)""" % (adapt(BIND), adapt(Line), adapt(CID))

  syslog.syslog(sql)

  try:
    cur.execute(sql)
    res = cur.rowcount
    cur.close()
    return(res)
  except psycopg2.Error, e:
    syslog.syslog("SQL ERROR: " + str(e.pgerror))

  return(-2)


# SELECT src,accountcode,"x-record" FROM cdr WHERE uniqueid='1399458365.2605';
def BL(BIND,uniqueid,enable):
  syslog.syslog('BL(%s, %s, %s)' % (BIND, uniqueid, enable))
  try:
    cdr = psycopg2.connect("dbname='cdr' user='asterisk' password=''")
  except:
    syslog.syslog('SQL ERROR: Can not connect to database')
    return(-1)

  cur = cdr.cursor(cursor_factory=psycopg2.extras.DictCursor)
  sql = """SELECT src,accountcode,"x-record" FROM cdr WHERE uniqueid=%s;""" % adapt(uniqueid)
#  syslog.syslog(sql)

  try:
    res = 0
    cur.execute(sql)
    if (cur.rowcount > 0):
      row=cur.fetchone()
      syslog.syslog(row['src'])
      if (enable & (BlackListCheck(BIND, row['accountcode'], row['src']) > 0)):
        return 1
      else:
        res = BlackList(BIND, row['accountcode'], row['src'], row['x-record'], enable)

    cur.close()
    return(res)

  except psycopg2.Error, e:
    syslog.syslog("SQL ERROR: " + str(e.pgerror))



def DND(BIND,cid,enable,check):
  syslog.syslog('DND(%s, %s, %s, %s)' % (BIND, cid, enable, check))
  try:
    connect = psycopg2.connect("dbname='pbx' user='asterisk' password=''")
  except:
    syslog.syslog('SQL ERROR: Can not connect to database')
    return(-1)

  cur = connect.cursor(cursor_factory=psycopg2.extras.DictCursor)

  if not(check):
    sql = """UPDATE "Exten" SET "DND" = %s where ("BIND" = %s) and ("Exten" = %s);""" % (adapt(enable), adapt(BIND), adapt(cid))
#  syslog.syslog(sql)

    try:
      cur.execute(sql)
      connect.commit()
      res = cur.rowcount
      cur.close ()
      return(res)
    except psycopg2.Error, e:
      syslog.syslog("SQL ERROR: " + str(e.pgerror))
  else:
    sql = """SELECT "DND" FROM "Exten" where ("BIND" = %s) and ("Exten" = %s);""" % (adapt(BIND), adapt(cid))
    try:
      cur.execute(sql)
      if (cur.rowcount < 1):
        return -1
      row = cur.fetchone()

      syslog.syslog("%s" % row['DND'])
      cur.close ()
      if row['DND']:
        return 1
      else:
        return 0
    except psycopg2.Error, e:
      syslog.syslog("SQL ERROR: " + str(e.pgerror))

  return(-2)


def TRNF(BIND,cid,did):
  syslog.syslog('TRNF(%s, %s, %s)' % (BIND, cid, did))
  try:
    connect = psycopg2.connect("dbname='pbx' user='asterisk' password=''")
  except:
    syslog.syslog('SQL ERROR: Can not connect to database')
    return(-1)

  cur = connect.cursor(cursor_factory=psycopg2.extras.DictCursor)
  sql = """UPDATE "Exten" SET "TransferCall" = %s where ("BIND" = %s) and ("Exten" = %s);""" % (adapt(did), adapt(BIND), adapt(cid))
#  syslog.syslog(sql)

  try:
    cur.execute(sql)
    connect.commit()
    res = cur.rowcount
    cur.close ()
    return(res)
  except psycopg2.Error, e:
    syslog.syslog("SQL ERROR: " + str(e.pgerror))

  return(-2)

class MyHandler(BaseHTTPRequestHandler):
  def do_GET(self):
    self.send_error(404,"Not Found")


  def do_POST(self):
    try:
      ctype, pdict = cgi.parse_header(self.headers.getheader("content-type"))
      if ctype == "multipart/form-data":
        query = cgi.parse_multipart(self.rfile, pdict)

      form = cgi.FieldStorage(
        fp=self.rfile,
        headers=self.headers,
        environ={'REQUEST_METHOD':'POST',
                 'CONTENT_TYPE':self.headers['Content-Type'],
                })

      result = 0
      response = 200
      action = form.getvalue("action", "")
      if (action == 'dial'):
        cid = form.getvalue("cid", "")
        did = form.getvalue("did", "")
        uuid = form.getvalue("uuid", "")

        syslog.syslog('dial %s %s %s %s' % (cid, did, BIND, uuid))
        pipe = subprocess.Popen("/ast/call.pl %s %s %s %s" % (cid, did, BIND, uuid), shell=True, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        pipe.stdin.close()
        pipe.wait()
      elif (action == 'bxfer'):
        linkedid = form.getvalue("linkedid", "")
        leg = form.getvalue("leg", "A")
        if (leg != 'A'):
          leg='B'
        did = form.getvalue("did", "")

        syslog.syslog('bxfer %s %s %s' % (linkedid, leg, did))
        pipe = subprocess.Popen("""(/usr/sbin/asterisk -rx 'group show channels' | awk ' $2~"%s" && $3~"Leg%s" {print $1}' | xargs -I XXX /usr/sbin/asterisk -rx "channel redirect XXX transfer,%s,1" 2>&1) | grep 'redirected' """ % (linkedid, leg, did), shell=True, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        data = pipe.communicate()[0]
        result = pipe.returncode
        if (result > 1):
          response = 400

        pipe.stdin.close()
        pipe.wait()
      elif (action == 'dnd'):
        cid = form.getvalue("cid", "")
        enable = form.getvalue("enable", "0")
        check = form.getvalue("check", "0")
        result = DND(BIND,cid,(enable=="1"),(check=="1"))
        if (result < 0):
          response = 400
      elif (action == 'trnf'):
        cid = form.getvalue("cid", "")
        did = form.getvalue("did", "")
        TRNF(BIND,cid,did)
      elif (action == 'bl'):
        uniqueid = form.getvalue("uniqueid", "")
        enable = form.getvalue("enable", "")
        check = form.getvalue("check", "0")
        if (check=="1"):
          cid = form.getvalue("cid", "")
          did = form.getvalue("did", "")
          result = BlackListCheck(BIND,did,cid)
          if (result < 0):
            response = 400
        else:
          result = BL(BIND,uniqueid,(enable=="1"))
          if (result < 0):
            response = 400
      elif (action == 'rl'):
        cid = form.getvalue("cid", "")
        enable = form.getvalue("enable", "")
        check = form.getvalue("check", "0")
        if (check=="1"):
          result = BlackListCheck(BIND,'agent',cid)
          if (result < 0):
            response = 400
        else:
          result = BlackList(BIND,'agent',cid,'AGENT CALLS',(enable=="1"))
          if (result < 0):
            response = 400

      self.send_response(response)
      self.send_header("Result", result)
      self.end_headers()

# Transfer
# asterisk -rx 'group show channels' | awk ' $2~"1396455216.3609" && $3~"LegA" {print $1}' | xargs -I XXX asterisk -rx "channel redirect XXX transfer,9583253,1"
# asterisk -rvvv | stdbuf -oL -eL grep uniqueid | stdbuf -oL -eL egrep -o '[0-9]+\.[0-9]+' | xargs -I XXX echo "curl --data 'action=bxfer&linkedid=XXX&leg=A&did=060' 188.227.101.17:8080
    except Exception, e:
      self.send_response(500)
      if not e:
        self.wfile.write("%s") % e
      self.end_headers()
#      pass


if __name__ == "__main__":
     try:
         syslog.openlog("HTTP", syslog.LOG_PID|syslog.LOG_CONS, syslog.LOG_LOCAL0)
         server = HTTPServer(("", 8080), MyHandler)
         print "started httpserver..."
         syslog.syslog('API STARTED')
         server.serve_forever()

     except KeyboardInterrupt:
         print "^C received, shutting down server"
         server.socket.close()
