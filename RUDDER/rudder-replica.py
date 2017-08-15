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
  The code below is next step of "api.py", the reference for Rudder API implementation, written in python 2.7.
It's now compliant with Rudder 3.2.10 .. 4.1.5. A number of the most stupid places was refactored.

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

"""
  Replication inheritance by setting tags in "Description" fields:
::public:: -X- ::private::
::public:: -X- ::protected::
::public:: ->- '' >> ::public::
::public:: ->- ::master:: >> ::master::
::public:: ->- ::slave:: >> ::slave::

::protected:: -X- ::private::
::protected:: -X- ::protected::
::protected:: ->- '' >> ''
::protected:: ->- ::public:: >> ::public::
::protected:: ->- ::master:: >> ::master::
::protected:: ->- ::slave:: >> ::slave::

::master:: -X- ::private::
::master:: -X- ::protected::
::master:: ->- '' >> ::slave::
::master:: ->- ::public:: >> ::slave::
::master:: ->- ::master:: >> ::slave::
::master:: ->- ::slave:: >> ::slave::

  Thus, ::public:: tag cocks inheritance with no depth limit,
  ::private:: breaks inheritance (no any replica to ::private::),
  ::master:: provides one-level inheritance to ::slave::
  ::slave:: terminates inheritance.
  ::protected:: protects object against rewriting, but behaves like ::public:: on source, except of remaining unchanged description.

"""
"""
ToDo:
 *  Make persistent dictionary to deal with object rename|delete actions.
 *  Don't try to insert existing objects.
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
parser.add_argument('-E', '--empty', dest='EMPTY_OK', action='store_true', default=False, help='Although replicate records without valid labels in Description fields')
parser.add_argument('-T', '--tag', dest='TAG', action='append', help='Only replicate records containing this tag (filter), m.b. multiple')

parser.add_argument('-P', '--parameters', dest='PARAMETERS', action='store_true', default=False, help='Replicate parameters')
parser.add_argument('-G', '--groups', dest='GROUPS', action='store_true', default=False, help='Replicate groups')
parser.add_argument('-D', '--directives', dest='DIRECTIVES', action='store_true', default=False, help='Replicate directives')
parser.add_argument('-R', '--rules', dest='RULES', action='store_true', default=False, help='Replicate rules')
parser.add_argument('-A', '--all', dest='ALL', action='store_true', default=False, help='Replicate all')

args = parser.parse_args()

SrcHost = args.SrcHost
SrcKey = args.SrcKey

DstHost = args.DstHost
DstKey = args.DstKey

TAGS = args.TAG

EMPTY_OK = args.EMPTY_OK
PARAMETERS = args.PARAMETERS
GROUPS = args.GROUPS
DIRECTIVES = args.DIRECTIVES
RULES = args.RULES
ALL = args.ALL

# As for 2017-02-21, rudder API ignores uuid's for PUT methods, so all transmission passes through dictionaries.
DictL = {} # Left namePath-to-UUID mapping
DictL['description'] = {}
DictR = {} # Right namePath-to-UUID mapping
DictR['description'] = {}
DictLR = {} # Map left UUID to right UUID


def dict_lr(DictL, DictR):
  global DictLR

  for i in DictL.keys():
    if i in DictR:
      if i != 'description':
        DictLR[ DictL[i] ] = DictR[i]
  return


def tags(description):
  global TAGS

  if TAGS is None:
    return True
  elif len(TAGS) == 0:
    return True

  for i in TAGS:
    if (description.find(i) >= 0):
      return True

  return False


def get(host, token, api):
  url='https://%s/rudder/api/latest/%s?prettify=true' % (host, api)
  req = urllib2.Request(url)
  req.add_header('X-API-Token', token)

  context = ssl._create_unverified_context()
  try:
    res = urllib2.urlopen(req, context=context)

    jq = res.read()
    return json.loads(jq)
  # Stop processing when unable to process GET method
  except urllib2.HTTPError, e:
    print "Error getting %s from %s" % (api, host)
    print url
    print "%s" % e.read()
    sys.exit()
  except urllib2.URLError, e:
    print "Error getting %s from %s" % (api, host)
    print 'URLError = ' + str(e.reason)
    sys.exit()
  except Exception:
    import traceback
    print 'Generic exception: ' + traceback.format_exc()
    sys.exit()


def put(api, data):
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
        if 'directives' in js['data']:
#          print js['data']['directives'][0]['id']
          return js['data']['directives'][0]['id']  # It's a cat's concert!
        elif 'ruleCategories' in js['data']:
          return js['data']['ruleCategories']['id']  # It's a cat's concert!
      except:
        print "WTF!? PUT result is:\n%s\n" % body
        sys.exit(0)
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
      elif  body.rfind('already exists') > 0: #  Directive
        print "  * %s = %s::\"%s\" exists on %s" % (js['errorDetails'], json.loads(data)['techniqueName'], json.loads(data)['displayName'], DstHost)
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
#    print data
    res = urllib2.urlopen(req, context=context)
#    body = res.read()
#    print body
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
  if not (parent in CDict):
    print "* Skip sync for group \"%s\": branch filtered" % parent
    return

  category = CDict[parent]
  for i in js:
    description = i['description']
    master = (i['description'].find('::master::') >= 0)
    if master:
      description = '::slave::'
    elif (i['description'].find('::protected::') >= 0):
      description = ''

    if tags(i['description']) & ((i['description'].find('::public::') >= 0) | (i['description'].find('::protected::') >= 0) | master | (EMPTY_OK & (i['description'].find('::private::') < 0) & (i['description'].find('::slave::') < 0))):
      print "   ", i['id'], i['displayName']
      new = {'category' : category,
             'id' : i['id'], # Silently ignored :(~
             'displayName' : i['displayName'],
             'description' : description,
             'dynamic' : i['dynamic'],
             'query' : i['query'],
             'enabled' : i['enabled']
            }

#      print json.dumps(new, indent=2, separators=(',', ': '))
      uuid = put('groups', json.dumps(new))
      if len(str(uuid)) == 36: # New uuid
        CDict["%s||%s" % (parent, i['displayName'])] = uuid
      elif uuid == 1: # Update
        uuid = CDict["%s||%s" % (parent, i['displayName'])]
        description = CDict['description']["%s||%s" % (parent, i['displayName'])]

        if (description.find('::private::') >= 0) | (description.find('::protected::') >= 0):
          continue # Don't touch private groups
        elif (i['description'].find('::protected::') >= 0):
          new['description'] = description # ::protected:: does not affect description
        elif (description.find('::master::') >= 0) & (not master):
          new['description'] = description # Don't change master description if ::public:: on the left
        elif description.find('::slave::') >= 0:
          new['description'] = description # Don't change slave description

        new.pop('category')
        new.pop('id')
        post('groups', uuid, json.dumps(new))
#        print "  ~%s updated" % uuid
      else:
        print "  UNKNOWN ERROR %s with group \"%s\"" % (uuid, i['displayName'])
    else:
      print "* Skip sync for group \"%s\": group filtered" % parent
  return


# Sync group categories tree.
def group_categories(CDict, js, parent = ''):
  if 'categories' in js:
    if parent == 'Root of the group and group categories':
      category = 'GroupRoot'
    elif (parent != '') & (parent in CDict):
      category = CDict[parent]
    else:
      category = 'GroupRoot'

    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')

    description = js['description']
    master = (js['description'].find('::master::') >= 0)
    if master:
      js['description'] = '::slave::'
    elif (js['description'].find('::protected::') >= 0):
      description = ''

    if js['id'] != 'GroupRoot':
      if tags(js['description']) & ((js['description'].find('::public::') >= 0) | (js['description'].find('::protected::') >= 0) | master | (EMPTY_OK & (js['description'].find('::private::') < 0) & (js['description'].find('::slave::') < 0))):
        print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent
        if (js['description'].find('::protected::') >= 0):
          js['description'] = ''
        new = {'parent' : category,
               'name' : js['name'],
               'description' : description
              }
        uuid = put('groups/categories?prettify=true', json.dumps(new) )

        if len(str(uuid)) == 36: # New uuid
          CDict[parent] = uuid
          print "* New group cat. CDict[%s]" % parent, " = ", uuid

        elif uuid == 1: # Update
          uuid = CDict[parent]
          description = CDict['description'][parent]

          if (description.find('::private::') >= 0) | (description.find('::protected::') >= 0):
            return # Don't touch private branches
          elif (js['description'].find('::protected::') >= 0):
            new['description'] = description # ::protected:: does not affect description
          elif (description.find('::master::') >= 0) & (not master):
            js['description'] = description # Don't change master description if ::public:: on the left
          elif description.find('::slave::') >= 0:
            js['description'] = description # Don't change slave description

          post('groups/categories', uuid, json.dumps({'name': js['name'], 'description': js['description']}))

        else:
          print "  UNKNOWN ERROR %s with group category \"%s\"" % (uuid, js['name'])

    for i in js['categories']:
      group_categories(CDict, i, parent)

  return


# Sync groups in tree.
def categories_groups(CDict, js, parent = ''):
  if 'categories' in js:
    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')
    print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent   #, "parent_id=%s" % CDict[parent]

    if ('groups' in js):
      if js['id'] != 'SystemGroups': # Embedded groups are unmutable
        groups(CDict, js['groups'], parent)

    for i in js['categories']:
      categories_groups(CDict, i, parent)

  return


def category_dict(name, CDict, js, object_description = 'description', parent = ''): # Categories for name = (groups, rules)
  parent = ("%s|%s" % (parent, js['name'])).lstrip('|')

  if 'categories' in js:
    CDict[parent] = js['id']
    CDict['description'][parent] = js['description']
#    print '>> ', parent, " >> ", CDict[parent], ' == ',  CDict['description'][parent]

    if name in js:
      for i in js[name]:
        CDict["%s||%s" % (parent, i['displayName'])] = i['id']
        CDict['description']["%s||%s" % (parent, i['displayName'])] = i[object_description]
#        print "  rule", i['displayName'], " = ", i['id']

    for i in js['categories']:
      category_dict(name, CDict, i, object_description, parent)

  return


### 4. Parameters
def parameters(js1, js2):
  P = {} # Existing right-side parameters
  for i in js2:
    P[ i['id'] ] = i['description']

  for i in js1:
    description = i['description']
    master = (i['description'].find('::master::') >= 0)
    if master:
      description = '::slave::'
    elif (i['description'].find('::protected::') >= 0):
      description = ''

    if tags(i['description']) & ((i['description'].find('::public::') >= 0) | (i['description'].find('::protected::') >= 0) | master | (EMPTY_OK & (i['description'].find('::private::') < 0) & (i['description'].find('::slave::') < 0))):
      j = i   # Make copy for put
      j['description'] = description
      uuid = put('parameters', json.dumps(j))

      if len(str(uuid)) >1: # New uuid
        print "New parameter \"%s\" = \"%s\"" % (i['id'], i['value'])

      elif uuid == 1: # Update
        uuid = i.pop('id')
        if (P[uuid].find('::private::') >= 0) | (P[uuid].find('::protected::') >= 0):
          continue # Don't touch private parameters
        elif (i['description'].find('::protected::') >= 0):
          i['description'] = P[uuid]  # ::protected:: does not affect description
        elif (P[uuid].find('::master::') >= 0) & (not master):
          i['description'] = P[uuid] # Don't change master description if ::public:: on the left
        elif P[uuid].find('::slave::') >= 0:
          i['description'] = P[uuid] # Don't change slave description

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
    CDict['description']["DIRECTIVE::%s||%s" % (i['techniqueName'], i['displayName'])] = i['longDescription']
    # Directive conversion dongle
    if i['techniqueName'] == 'aptPackageInstallation_1':
      i['techniqueName'] = 'packageManagement'
      CDict["DIRECTIVE::%s||%s" % (i['techniqueName'], i['displayName'])] = i['id']
      CDict['description']["DIRECTIVE::%s||%s" % (i['techniqueName'], i['displayName'])] = i['longDescription']
  return


def directives(CDict, js):
  addDict = 0

  for i in js:
    description = i['longDescription']
    master = (i['longDescription'].find('::master::') >= 0)
    if master:
      description = '::slave::'
    elif (i['longDescription'].find('::protected::') >= 0):
      description = ''

    if tags(i['longDescription']) & ((i['longDescription'].find('::public::') >= 0) | (i['longDescription'].find('::protected::') >= 0) | master | (EMPTY_OK & (i['longDescription'].find('::private::') < 0) & (i['longDescription'].find('::slave::') < 0))):
#      i.pop('techniqueVersion') # Assume to use the latest one # Crashes rudder in case of significant technique mismatch
#      print json.dumps(i, indent=2, separators=(',', ': '))
      if i['techniqueName'] == 'aptPackageInstallation_1':
        print "!!!Bad directive %s::%s = \"%s\"" % (i['techniqueName'], i['id'], i['displayName'])
#        print json.dumps(i, indent=2, sort_keys=False, separators=(',', ': '))
        i = convert_packageManagement(i)
        addDict = 1
      elif i['techniqueName'] == 'aptPackageInstallation':
        pass
      else:
        pass

#      print json.dumps(i, indent=2, sort_keys=False, separators=(',', ': '))
      j = i   # Make copy for put
      j['longDescription'] = description
      uuid = put('directives', json.dumps(j))
      if len(str(uuid)) == 36: # New uuid
        CDict["DIRECTIVE::%s||%s" % (i['techniqueName'], i['displayName'])] = uuid
        print "New directive %s::%s = \"%s\"" % (i['techniqueName'], i['id'], i['displayName'])
      elif uuid == 1: # Update
        uuid = i.pop('id')
        description = CDict['description']["DIRECTIVE::%s||%s" % (i['techniqueName'], i['displayName'])]

        if (description.find('::private::') >= 0) | (description.find('::protected::') >= 0):
          continue # Don't touch private directives
        elif (i['longDescription'].find('::protected::') >= 0):
          i['longDescription'] = description # ::protected:: does not affect description
        elif (description.find('::master::') >= 0) & (not master):
          i['longDescription'] = description # Don't change master description if ::public:: on the left
        elif description.find('::slave::') >= 0:
          i['longDescription'] = description # Don't change slave description

        post('directives', uuid, json.dumps(i))
      else:
        print "  UNKNOWN ERROR %s with directive \"%s\"" % (uuid, i['id'])

      if addDict > 0:
        CDict["DIRECTIVE::%s||%s" % ('aptPackageInstallation_1', i['displayName'])] = uuid

  return


### 6. Rule categories
def r_categories(CDict, js, parent = ''):
#  CDict['Rules'] = 'rootRuleCategory'
#  print ">>> PARENT = %s" % parent
  if 'categories' in js:
    if (parent != '') & (parent in CDict):
      category = CDict[parent]
    else:
      category = 'rootRuleCategory'

    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')
    print parent

    description = js['description']
    master = (js['description'].find('::master::') >= 0)
    if master:
      js['description'] = '::slave::'
    elif (js['description'].find('::protected::') >= 0):
      description = ''

    if js['id'] != 'rootRuleCategory':
      print js['id'], js['description']
      if tags(js['description']) & ((js['description'].find('::public::') >= 0) | (js['description'].find('::protected::') >= 0) | master | (EMPTY_OK & (js['description'].find('::private::') < 0) & (js['description'].find('::slave::') < 0))):
        print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent
#        print "PARENT = \"%s\"" % category
        uuid = put('rules/categories?prettify=true', json.dumps({'name': js['name'], 'parent': category, 'description': description}) )

        if len(str(uuid)) == 36: # New uuid
          CDict[parent] = uuid
          print "* New rule cat. CDict[%s]" % parent, " = ", uuid
        elif uuid == 1: # Update
          uuid = CDict[parent]
          description = CDict['description'][parent]

          if (description.find('::private::') >= 0) | (description.find('::protected::') >= 0):
            return # Don't touch private branches
          elif (js['description'].find('::protected::') >= 0):
            js['description'] = description # ::protected:: does not affect description
          elif (description.find('::master::') >= 0) & (not master):
            js['description'] = description # Don't change master description if ::public:: on the left
          elif description.find('::slave::') >= 0:
            js['description'] = description # Don't change slave description

          post('rules/categories', uuid, json.dumps({'name': js['name'], 'description': js['description']}))
#          print "Updated description = %s for %s" % (js['description'], uuid) # This API doesn't work
        else:
          print "  UNKNOWN ERROR %s with rules category \"%s\"" % (uuid, js['id'])

    for i in js['categories']:
      r_categories(CDict, i, parent)

  return


### 8. Rules
def rules(CDict, js, parent = ''):
  global DictLR   # (directives & targets) to be translated!!!
  global DictL    # Reference for error handling

  if not (parent in CDict):
    print "* Skip sync for group \"%s\": branch filtered" % parent
    return

  category = CDict[parent]
  for i in js:
    description = i['longDescription']
    master = (i['longDescription'].find('::master::') >= 0)
    if master:
      description = '::slave::'
    elif (i['longDescription'].find('::protected::') >= 0):
      description = ''

    if tags(i['longDescription']) & ((i['longDescription'].find('::public::') >= 0) | (i['longDescription'].find('::protected::') >= 0) | master | (EMPTY_OK & (i['longDescription'].find('::private::') < 0) & (i['longDescription'].find('::slave::') < 0))):
      print "   ", i['id'], i['displayName']

      for x, j in enumerate(i['directives']):
        try:
          i['directives'][x] = DictLR[j]
        except:
          print "Fatal: missing DictLR for directive \"%s\"" % i['directives'][x]   # ['displayName']
          for k, v in DictL.iteritems():   # Find key by value (uuid)
            if v == i['directives'][x]:
              print k
          sys.exit(1)

#      print ">>> exclude: %s" % i['targets'][0]['exclude']['or']
#      print ">>> include: %s" % i['targets'][0]['include']['or']
#      print "@@@ ", i['targets']
      exclude = []
      for x, j in enumerate(i['targets'][0]['exclude']['or']):
        if j.find('group:') == 0:
          s = j.split(':')[1]
          if s in DictLR:
            exclude.append("group:%s" % DictLR[s])
          else:
            print "* Skip exclude for group \"%s\": group filtered" % s
        else:
          print "* Exclude special group \"%s\"" % i['targets'][0]['exclude']['or'][x]
          exclude.append(i['targets'][0]['exclude']['or'][x])

      i['targets'][0]['exclude']['or'] = exclude

      include = []
      for x, j in enumerate(i['targets'][0]['include']['or']):
        if j.find('group:') == 0:
          s = j.split(':')[1]
          if s in DictLR:
            include.append("group:%s" % DictLR[s])
          else:
            print "* Skip include for group \"%s\": group filtered" % s
        else:
          print "* Include special group \"%s\"" % i['targets'][0]['include']['or'][x]
          include.append(i['targets'][0]['include']['or'][x])

      i['targets'][0]['include']['or'] = include

      new = {'category' : category,
             'id' : i['id'],
             'displayName' : i['displayName'],
             'shortDescription' : i['shortDescription'],
             'longDescription' : description,
             'enabled' : i['enabled'],
             'directives' : i['directives'],
             'targets' : i['targets'] ### Must be converted to dst-groups uuid's
            }

#      print json.dumps(new, indent=2, separators=(',', ': '))
      uuid = put('rules', json.dumps(new))
      if len(str(uuid)) == 36: # New uuid
        CDict["%s||%s" % (parent, i['displayName'])] = uuid
      elif uuid == 1: # Update
        uuid = CDict["%s||%s" % (parent, i['displayName'])]
        description = CDict['description']["%s||%s" % (parent, i['displayName'])]

        if (description.find('::private::') >= 0) | (description.find('::protected::') >= 0):
          continue # Don't touch private groups
        elif (i['longDescription'].find('::protected::') >= 0):
          new['longDescription'] = description # ::protected:: does not affect description
        elif (description.find('::master::') >= 0) & (not master):
          new['longDescription'] = description # Don't change master description if ::public:: on the left
        elif description.find('::slave::') >= 0:
          new['longDescription'] = description # Don't change slave description

        new.pop('category')
        new.pop('id')
        post('rules', uuid, json.dumps(new))
        print "  ~%s updated" % uuid
      else:
        print "  UNKNOWN ERROR %s with rule \"%s\"" % (uuid, i['id'])

  return


def categories_rules(CDict, js, parent = ''):
  if 'categories' in js:
    parent = ("%s|%s" % (parent, js['name'])).lstrip('|')
    print js['id'], "\"%s\"" % js['name'], "parent=%s" % parent   #, "parent_id=%s" % CDict[parent]

    if 'rules' in js:
      rules(CDict, js['rules'], parent)

    for i in js['categories']:
      categories_rules(CDict, i, parent)

  return


def GROUPS_DICT():
  global DictL
  global DictR

  print "Preparing groups dictionaries..."
  js1 = get(SrcHost, SrcKey, 'groups/tree')
  category_dict('groups', DictL, js1['data']['groupCategories']) # Left group categories

  js2 = get(DstHost, DstKey, 'groups/tree')
  category_dict('groups', DictR, js2['data']['groupCategories'])
  return js1


def DIRECTIVES_DICT():
  global DictL
  global DictR

  print "Preparing directives dictionaries..."
  js1 = get(SrcHost, SrcKey, 'directives')
  directive_dict(DictL, js1['data']['directives'])

  js2 = get(DstHost, DstKey, 'directives')
  directive_dict(DictR, js2['data']['directives'])
  return js1

### def MAIN ###
if GROUPS | ALL:
  ### ##1. Src group categories >> Dst
  print "##1. Src group categories >> Dst"
  js1 = GROUPS_DICT()

  group_categories(DictR, js1['data']['groupCategories'])
  print ""

  ### ##2. Create/update dst groups
  print "##2. Create/update dst groups"
  categories_groups(DictR, js1['data']['groupCategories'])
  print ""


### ##3. Create/update dst parameters
if PARAMETERS | ALL:
  print "##3. Create/update dst parameters"
  js1 = get(SrcHost, SrcKey, 'parameters')
  js2 = get(DstHost, DstKey, 'parameters')

  parameters(js1['data']['parameters'], js2['data']['parameters'])
  print ""


### ##4. Create/update dst directives
if DIRECTIVES | ALL:
  print "##4. Create/update dst directives"
  if not (GROUPS | ALL):
    GROUPS_DICT()

  js1 = DIRECTIVES_DICT()

  directives(DictR, js1['data']['directives'])
  print ""


if RULES | ALL:
  if not (GROUPS | ALL):
    GROUPS_DICT()

  if not (DIRECTIVES | ALL):
    DIRECTIVES_DICT()

  ### ##5. Src rule categories >> Dst
  print "##5. Src rule categories >> Dst (if absent)"
  js1 = get(SrcHost, SrcKey, 'rules/tree')
  category_dict('rules', DictL, js1['data']['ruleCategories'], 'longDescription')

  js2 = get(DstHost, DstKey, 'rules/tree')
  print "***"
  category_dict('rules', DictR, js2['data']['ruleCategories'], 'longDescription')

  r_categories(DictR, js1['data']['ruleCategories'])
  print ""

  ### ##6. L<>R group/rule uuid's
  print "##6. L<>R group/rule uuid's"
  dict_lr(DictL, DictR)
  #print "### DICT ###"
  #for i in DictLR.keys():
  #  print i, " = ", DictLR[i]
  print ""

  ### ##7. Create/update dst rules
  print "##7. Create/update dst rules"
  categories_rules(DictR, js1['data']['ruleCategories'])
  print ""
