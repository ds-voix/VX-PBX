#!/usr/bin/python -u
# -*- coding: utf-8 -*-

# Rudder (http://www.rudder-project.org/) rules transmission via REST API
# Copyright (C) 2017 Dmitry Svyatogorov ds@vo-ix.ru

#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU Affero General Public License as
#    published by the Free Software Foundation, either version 3 of the
#    License, or (at your option) any later version.
#
#    This program is distributed in the hope that it will be useful,
#    but WITHOUT ANY WARRANTY; without even the implied warranty of
#    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#    GNU Affero General Public License for more details.
#
#    You should have received a copy of the GNU Affero General Public License
#    along with this program.  If not, see <http://www.gnu.org/licenses/>.

"""
  The code below is reference for Rudder API implementation, written in python 2.7.
It is as plain as I could write, so I hope it would not be difficult to pull out what You need.
Although I hope it will be helpfull to improve Rudder's API docs https://www.rudder-project.org/rudder-api-doc/
by adding code examples. And for start implementing API autotests. And for patch API inconsisnencies, so
implemented ugly workarounds will be gone with time. Regards!
"""
"""
* Note convert_packageManagement() - the sample howto for migration between 2 similar techniques.
  It's not clear enough because of json reversing inside.
* Be carefull as there left out a number of bugs. I done the things step-by-step, and suggest You to do the same.
  Pass the step, check results, fix code if it need.
* Note the dumb 500-for-all in-text errors. API must be seriously clarified.
* Note the obvious unefficiency on long distances. Code must be run as close to DST as in could.
  The time to think about batch-writes.
* Note that all transmissions are out-of-transaction. All intermediate states must be considered inconsistent!
"""
import sys
import time
import urllib, urllib2
import ssl
import json

import argparse
parser = argparse.ArgumentParser(description='Rudder (SrcHost >> DstHost) rules transmission via REST API.')
#  parser.print_help()
parser.add_argument('-s', '--src-host', dest='SrcHost', required=True, help='Source Rudder server')
parser.add_argument('-d', '--dst-host', dest='DstHost', required=True, help='Destination Rudder server')
parser.add_argument('-k', '--src-key', dest='SrcKey', required=True, help='Source server API key')
parser.add_argument('-K', '--dst-key', dest='DstKey', required=True, help='Destination server API key')
args = parser.parse_args()

SrcHost = args.SrcHost
SrcKey = args.SrcKey

DstHost = args.DstHost
DstKey = args.DstKey

# As for 2017-02-21, rudder API ignores uuid's for PUT methods, so all transmission passes through dictionaries.
DictL = {} # Left namePath-to-UUID mapping
DictR = {} # Right namePath-to-UUID mapping
DictLR = {} # Map left UUID to right UUID

def dict_lr(DictL, DictR):
  global DictLR

  for i in DictL.keys():
    if i in DictR:
      DictLR[ DictL[i] ] = DictR[i]
  return


def put(api,data):
  global DstHost, DstKey
  context = ssl._create_unverified_context()
  url='https://%s/rudder/api/latest/%s' % (DstHost, api)
  req = urllib2.Request(url, data)
#  print "***", data
  req.add_header('X-API-Token', DstKey)
  req.add_header('Content-Type', 'application/json; charset=utf-8')
  req.get_method = lambda:"PUT"

  try:
    res = urllib2.urlopen(req, context=context)
    body = res.read()
#    length = res.info()['Content-Length']
    js = json.loads(body)
    if 'id' in js:
      return js['id']
    else:
      try:
        return js['data']['ruleCategories']['id']  # It's a cat's concert!
      except:
        print "WTF!? PUT result is:\n%s\n" % body
  except urllib2.HTTPError, e:
      body = e.read()
      try:
        js = json.loads(body)
      except:
        js={}
        js['id'] = 'NO_ANSWER_BODY!'
      if body.rfind('same name exists') > 0:
        print "* Group/rule category exists on %s" % DstHost
        return 1 # To be updated
      elif body.rfind('group with the same') > 0:
        print "  * Group exists on %s" % DstHost
        return 1 # To be updated
      elif body.rfind('parameter with the same name') > 0:
        print "  * Parameter \"%s\" exists on %s" % (js['id'], DstHost)
        return 1 # To be updated
      elif  body.rfind('already exists') > 0:
        print "  * Directive %s = %s::\"%s\" exists on %s" % (js['id'], json.loads(data)['techniqueName'], json.loads(data)['displayName'], DstHost)
        return 1 # To be updated
      elif body.rfind('rule with the same name') > 0:
        print "  * Rule exists on %s" % DstHost
        return 1 # To be updated
      else:
        print req.get_full_url()
        print req.get_method()
        print 'HTTPError = ' + str(e.code) + "\n" + str(e.hdrs) + "\n" + body
  except urllib2.URLError, e:
      print 'URLError = ' + str(e.reason)
      sys.exit()
  except Exception:
      import traceback
      print 'generic exception: ' + traceback.format_exc()
      sys.exit()

  return 0 # Failed

def post(api, uuid, data):
  global DstHost, DstKey
  context = ssl._create_unverified_context()
  url='https://%s/rudder/api/latest/%s/%s' % (DstHost, api, uuid)
  req = urllib2.Request(url, data)
#  print "***", data
  req.add_header('X-API-Token', DstKey)
  req.add_header('Content-Type', 'application/json; charset=utf-8')
  req.get_method = lambda:"POST"

  try:
    res = urllib2.urlopen(req, context=context)
#    length = res.info()['Content-Length']
    return 0 # OK
  except urllib2.HTTPError, e:
      body = e.read()
      print req.get_full_url()
      print req.get_method()
      print 'HTTPError = ' + str(e.code) + "\n" + str(e.hdrs) + "\n" + body
  except urllib2.URLError, e:
      print 'URLError = ' + str(e.reason)
      sys.exit()
  except Exception:
      import traceback
      print 'generic exception: ' + traceback.format_exc()
      sys.exit()

  return -1 # Failed

def groups(CDict, js, parent = ''):
  category = CDict[parent]
  for i in js:
    print "   ", i['id'], i['displayName']
    new = {'category' : category,
#           'id' : i['id'], # Silently ignored :(~
           'displayName' : i['displayName'],
           'description' : i['description'],
           'dynamic' : i['dynamic'],
           'query' : i['query'],
           'enabled' : i['enabled']
          }

#    print json.dumps(new, indent=2, separators=(',', ': '))
    uuid = put('groups', json.dumps(new))
    if len(str(uuid)) == 36: # New uuid
      CDict["%s||%s" % (parent, i['displayName'])] = uuid
    elif uuid == 1: # Update
      uuid = CDict["%s||%s" % (parent, i['displayName'])]
      post('groups', uuid, json.dumps(new))
#      print "  ~%s updated" % uuid
    else:
      print "  UNKNOWN ERROR %s with group \"%s\"" % (uuid, i['displayName'])

  return

def categories(CDict, js, parent = ''):
  if 'categories' in js:
    if parent == 'Root of the group and group categories':
      category = 'GroupRoot'
    elif (parent != '') & (parent in CDict):
      category = CDict[parent]
    else:
      category = 'GroupRoot'

    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')
    print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent
    if js['id'] != 'GroupRoot':
      new = {'parent' : category,
             'name' : js['name'],
             'description' : js['description']
            }
      uuid = put('groups/categories?prettify=true', json.dumps(new) )
      if len(str(uuid)) == 36: # New uuid
        CDict[parent] = uuid
        print "* CDict[%s]" % parent, " = ", uuid
      elif uuid == 1: # Update
# CDict must be filled, write "step 0" first
#        uuid = CDict["%s||%s" % (parent, i['name'])]
#        post('groups/categories', uuid, json.dumps(new))
        print "  ~%s to be updated, but no update code right now" % uuid
      else:
        print "  UNKNOWN ERROR %s with group category \"%s\"" % (uuid, js['name'])

    for i in js['categories']:
      categories(CDict, i, parent)

  return

def categories_groups(CDict, js, parent = ''):

  if 'categories' in js:
    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')
    print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent, "parent_id=%s" % CDict[parent]

    if 'groups' in js:
      if js['id'] != 'SystemGroups': # Embedded groups are unmutable
        groups(CDict, js['groups'], parent)

    for i in js['categories']:
      categories_groups(CDict, i, parent)

  return


def category_dict(CDict, js, parent = ''):
  parent = ("%s|%s" % (parent, js['name'])).lstrip('|')

  if 'categories' in js:
    CDict[parent] = js['id']
#    print parent, " >> ", CDict[parent]

    if 'groups' in js:
      for i in js['groups']:
        CDict["%s||%s" % (parent, i['displayName'])] = i['id']
#        print "  group", i['displayName'], " = ", i['id']

    for i in js['categories']:
      category_dict(CDict, i, parent)

  return

### 4. Parameters
def parameters(js):
  for i in js:
    uuid = put('parameters', json.dumps(i))
    if len(str(uuid)) >1: # New uuid
      print "New parameter \"%s\" = \"%s\"" % (i['id'], i['value'])
    elif uuid == 1: # Update
      uuid = i.pop('id')
      post('parameters', uuid, json.dumps(i))
    else:
      print "  UNKNOWN ERROR %s with parameter \"%s\"" % (uuid, i['id'])

  return

### 5. Directives
def convert_packageManagement(i): # Rebuild "aptPackageInstallation" to "packageManagement"
  n = {} # Assemble new directive
  n['id'] = i['id']
  n['displayName'] = i['displayName']
  n['shortDescription'] = i['shortDescription']
  n['longDescription'] = i['longDescription']
  n['techniqueName'] = 'packageManagement'
  n['parameters'] = {}
  n['parameters']['section'] = {}

  p = n['parameters']['section']
  p['name'] = 'sections'
  p['sections'] = []

  for j in i['parameters']['section']['sections']:
    d = {} # Collect old directive variables
    print j['section']['name']
    for k in j['section']['vars']:
      d[ k['var']['name'] ] = k['var']['value']
    for k in j['section']['sections']:
      for l in k['section']['vars']:
        d[ l['var']['name'] ] = l['var']['value']

    print d

### section "Package"
    s = {}
    s['name'] = 'Package'
    s['vars'] = []
    s['sections'] = []

    v = {}
    v['name'] = 'PACKAGE_LIST'
    v['value'] = d['APT_PACKAGE_DEBLIST']
    s['vars'].append({'var' : v})

    v = {}
    v['name'] = 'PACKAGE_STATE' # << APT_PACKAGE_DEBACTION
    if d['APT_PACKAGE_DEBACTION'] == 'add':
      v['value'] = 'present'
    elif d['APT_PACKAGE_DEBACTION'] == 'update':
      v['value'] = 'present'
    else:
      v['value'] = 'absent'
    s['vars'].append({'var' : v})

### ### section "Package architecture"
    s1 = {}
    s1['name'] = 'Package architecture'
    s1['vars'] = []

    v = {}
    v['name'] = 'PACKAGE_ARCHITECTURE'
    v['value'] = 'default'
    s1['vars'].append({'var' : v})
    v = {}
    v['name'] = 'PACKAGE_ARCHITECTURE_SPECIFIC'
    v['value'] = ''
    s1['vars'].append({'var' : v})

    s['sections'].append({'section' : s1})

### ### section "Package manager"
    s1 = {}
    s1['name'] = 'Package manager'
    s1['vars'] = []

    v = {}
    v['name'] = 'PACKAGE_MANAGER'
    v['value'] = 'default'
    s1['vars'].append({'var' : v})

    s['sections'].append({'section' : s1})

### ### section "Package version"
    s1 = {}
    s1['name'] = 'Package version'
    s1['vars'] = []

    v = {}                        # APT_PACKAGE_VERSION_DEFINITION = default|specific
    v['name'] = 'PACKAGE_VERSION' # any|specific|latest
    if d['APT_PACKAGE_VERSION_DEFINITION'] == 'default':
      if d['APT_PACKAGE_DEBACTION'] == 'add':
        v['value'] = 'any'
      else:
        v['value'] = 'latest'
    else:
      v['value'] = 'specific'

    s1['vars'].append({'var' : v})
    v = {}
    v['name'] = 'PACKAGE_VERSION_SPECIFIC' # << APT_PACKAGE_VERSION
    v['value'] = d['APT_PACKAGE_VERSION']
    s1['vars'].append({'var' : v})

    s['sections'].append({'section' : s1})

### ### section "Post-modification script"
    s1 = {}
    s1['name'] = 'Post-modification script'
    s1['vars'] = []

    v = {}
    v['name'] = 'PACKAGE_POST_HOOK_COMMAND'
    v['value'] = ''
    s1['vars'].append({'var' : v})

    s['sections'].append({'section' : s1})

###
    p['sections'].append({'section' : s})
### END LOOP

  n['priority'] = i['priority']
  n['enabled'] = i['enabled']
  n['system'] = i['system']
#    n['policyMode'] = i['policyMode']

  return n

def directive_dict(CDict, js):
  for i in js:
    CDict["DIRECTIVE::%s||%s" % (i['techniqueName'], i['displayName'])] = i['id']
  return

def directives(js):
  global DictR
  addDict = 0

  for i in js:
    i.pop('techniqueVersion') # Assume to use the latest one
#    print json.dumps(i, indent=2, separators=(',', ': '))
    if i['techniqueName'] == 'aptPackageInstallation_1':
      print "!!!Bad directive %s::%s = \"%s\"" % (i['techniqueName'], i['id'], i['displayName'])
#      print json.dumps(i, indent=2, sort_keys=False, separators=(',', ': '))
      i = convert_packageManagement(i)
      addDict = 1
    elif i['techniqueName'] == 'aptPackageInstallation':
      pass
    else:
      pass

#    print json.dumps(i, indent=2, sort_keys=False, separators=(',', ': '))
    uuid = put('directives', json.dumps(i))
    if len(str(uuid)) == 36: # New uuid
      print "New directive %s::%s = \"%s\"" % (i['techniqueName'], i['id'], i['displayName'])
    elif uuid == 1: # Update
      uuid = i.pop('id')
      post('directives', uuid, json.dumps(i))
    else:
      print "  UNKNOWN ERROR %s with directive \"%s\"" % (uuid, i['id'])

    if addDict > 0:
      DictR["DIRECTIVE::%s||%s" % ('aptPackageInstallation_1', i['displayName'])] = uuid

  return

### 6. Rule categories
def r_categories(CDict, js, parent = ''):
  CDict['Rules'] = 'rootRuleCategory'

#  print ">>> PARENT = %s" % parent
  if 'categories' in js:
    if (parent != '') & (parent in CDict):
      category = CDict[parent]
    else:
      category = 'rootRuleCategory'

    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')
    print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent
    if js['id'] != 'rootRuleCategory':
#      print "PARENT = \"%s\"" % category
      uuid = put('rules/categories?prettify=true', json.dumps({'name': js['name'], 'parent': category, 'description ': js['description']}) )
      if len(str(uuid)) == 36: # New uuid
        CDict[parent] = uuid
#        print "CDict[%s] = %s" % (parent, uuid)
      elif uuid == 1: # Update
        uuid = CDict[parent]
        post('rules/categories', uuid, json.dumps({'name': js['name'], 'description ': js['description']}))
        print "Updated description = %s for %s" % (js['description'], uuid) # This API doesn't work
      else:
        print "  UNKNOWN ERROR %s with rules category \"%s\"" % (uuid, js['id'])
    for i in js['categories']:
#      print '###'
#      print json.dumps(i, indent=2, separators=(',', ': '))
      r_categories(CDict, i, parent)

  return

### 7. Dst rule categories dictionary
def r_category_dict(CDict, js, parent = ''):
  parent = ("%s|%s" % (parent, js['name'])).lstrip('|')

  if 'categories' in js:
    CDict[parent] = js['id']
    print parent, " >> ", CDict[parent]

    if 'rules' in js:
      for i in js['rules']:
        CDict["%s||%s" % (parent, i['displayName'])] = i['id']
        print "  rule", i['displayName'], " = ", i['id']

    for i in js['categories']:
      r_category_dict(CDict, i, parent)

  return

### 8. Rules
def rules(CDict, js, parent = ''):
  global DictLR # (directives & targets) to be translated!!!
  category = CDict[parent]
  for i in js:
    print "   ", i['id'], i['displayName']

    for x, j in enumerate(i['directives']):
      i['directives'][x] = DictLR[j]

    print ">>> exclude: %s" % i['targets'][0]['exclude']['or']
    print ">>> include: %s" % i['targets'][0]['include']['or']
    for x, j in enumerate(i['targets'][0]['exclude']['or']):
      if j.find('group:') == 0:
        s = j.split(':')[1]
        i['targets'][0]['exclude']['or'][x] = "group:%s" % DictLR[s]
        print "@@@ ", i['targets']
    for x, j in enumerate(i['targets'][0]['include']['or']):
      if j.find('group:') == 0:
        s = j.split(':')[1]
        i['targets'][0]['include']['or'][x] = "group:%s" % DictLR[s]
        print "@@@ ", i['targets']

    new = {'category' : category,
           'id' : i['id'],
           'displayName' : i['displayName'],
           'shortDescription' : i['shortDescription'],
           'longDescription' : i['longDescription'],
           'enabled' : i['enabled'],
           'directives' : i['directives'],
           'targets' : i['targets'] ### Must be converted to dst-groups uuid's
          }

#    print json.dumps(new, indent=2, separators=(',', ': '))
    uuid = put('rules', json.dumps(new))
    if len(str(uuid)) == 36: # New uuid
      CDict["%s||%s" % (parent, i['displayName'])] = uuid
    elif uuid == 1: # Update
      uuid = CDict["%s||%s" % (parent, i['displayName'])]
      post('rules', uuid, json.dumps(new))
      print "  ~%s updated" % uuid
    else:
      print "  UNKNOWN ERROR %s with rule \"%s\"" % (uuid, i['id'])

  return

def categories_rules(CDict, js, parent = ''):
  if 'categories' in js:
    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')
    print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent, "parent_id=%s" % CDict[parent]

    if 'rules' in js:
      rules(CDict, js['rules'], parent)

    for i in js['categories']:
      categories_rules(CDict, i, parent)

  return


### §1. Src group categories >> Dst (if absent)
print "§1. Src group categories >> Dst (if absent)"
url='https://%s/rudder/api/latest/groups/tree?prettify=true' % SrcHost
req = urllib2.Request(url)
req.add_header('X-API-Token', SrcKey)
#req.add_header('Accept', 'application/json')

context = ssl._create_unverified_context()
res = urllib2.urlopen(req, context=context)
#length = res.info()['Content-Length']

jq = res.read()
js1 = json.loads(jq)

category_dict(DictL, js1['data']['groupCategories']) # Left group categories
#SKIP#
categories(DictR, js1['data']['groupCategories'])
print ""


### §2. Dst group categories dictionary {name : id}, because id differs
print "§2. Dst group categories dictionary {name : id}, because id differs"
url='https://%s/rudder/api/latest/groups/tree?prettify=true' % DstHost
req = urllib2.Request(url)
req.add_header('X-API-Token', DstKey)

context = ssl._create_unverified_context()
res = urllib2.urlopen(req, context=context)

jq = res.read()
js2 = json.loads(jq)

category_dict(DictR, js2['data']['groupCategories'])
#print CDict
print ""


### §3. Create/update dst groups
print "§3. Create/update dst groups"
#SKIP#
categories_groups(DictR, js1['data']['groupCategories'])
print ""


### 4. Create/update dst parameters
print "§4. Create/update dst parameters"
url='https://%s/rudder/api/latest/parameters?prettify=true' % SrcHost
req = urllib2.Request(url)
req.add_header('X-API-Token', SrcKey)

context = ssl._create_unverified_context()
res = urllib2.urlopen(req, context=context)

jq = res.read()
js1 = json.loads(jq)

#SKIP#
parameters(js1['data']['parameters'])
print ""


### §5. Create/update dst directives
print "§5. Create/update dst directives"
url='https://%s/rudder/api/latest/directives?prettify=true' % SrcHost
req = urllib2.Request(url)
req.add_header('X-API-Token', SrcKey)

context = ssl._create_unverified_context()
res = urllib2.urlopen(req, context=context)

jq = res.read()
js1 = json.loads(jq)
#print jq
#sys.exit()

directive_dict(DictL, js1['data']['directives'])
directives(js1['data']['directives'])

url='https://%s/rudder/api/latest/directives?prettify=true' % DstHost
req = urllib2.Request(url)
req.add_header('X-API-Token', DstKey)

context = ssl._create_unverified_context()
res = urllib2.urlopen(req, context=context)

jq = res.read()
js2 = json.loads(jq)

directive_dict(DictR, js2['data']['directives'])
print ""


### 6. Src rule categories >> Dst (if absent)
print "§6. Src rule categories >> Dst (if absent)"
url='https://%s/rudder/api/latest/rules/tree?prettify=true' % SrcHost
req = urllib2.Request(url)
req.add_header('X-API-Token', SrcKey)

context = ssl._create_unverified_context()
res = urllib2.urlopen(req, context=context)

jq = res.read()
js1 = json.loads(jq)

#print jq
#sys.exit()

r_category_dict(DictL, js1['data']['ruleCategories'])
print ""


### §7. Dst rule categories dictionary {name : id}, because id differs
print "§7. Dst rule categories dictionary {name : id}, because id differs"
url='https://%s/rudder/api/latest/rules/tree?prettify=true' % DstHost
req = urllib2.Request(url)
req.add_header('X-API-Token', DstKey)

context = ssl._create_unverified_context()
res = urllib2.urlopen(req, context=context)

jq = res.read()
js2 = json.loads(jq)

r_category_dict(DictR, js2['data']['ruleCategories'])
r_categories(DictR, js1['data']['ruleCategories'])
print ""

### §8. L<>R group/rule uuid's
print "§8. L<>R group/rule uuid's"
dict_lr(DictL, DictR)
#print "### DICT ###"
#for i in DictLR.keys():
#  print i, " = ", DictLR[i]
print ""


### §9. Create/update dst rules
print "§9. Create/update dst rules"
categories_rules(DictR, js1['data']['ruleCategories'])
print ""
