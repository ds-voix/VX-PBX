#!/usr/bin/python
import os, datetime, subprocess, re, thread
import pyinotify

# https://www.mauras.ch/speed-up-rudder-new-nodes-detection.html
# apt-get install python-pyinotify

command = "/opt/rudder/bin/cf-agent -K -b sendInventoryToCmdb -f /var/rudder/cfengine-community/inputs/promises.cf"
report_re = re.compile('R:.*')

wm = pyinotify.WatchManager() # Watch Manager
mask = pyinotify.IN_DELETE | pyinotify.IN_CREATE # watched events

def exec_proc(file, dummy1):
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=None, shell=True)
    output = process.communicate()
    date = datetime.datetime.now()
#    report_line = report_re.search(output[0])
#    print "%s: %s - %s" % (date, file, report_line.group())

class EventHandler(pyinotify.ProcessEvent):
    def process_IN_CREATE(self, event):
        date = datetime.datetime.now()
        print "%s: %s - %s created" % (date, event.path, event.name)

        if "incoming" in event.path:
            dummy_tup = event.name, 'null'
            thread.start_new_thread(exec_proc, dummy_tup)

    def process_IN_DELETE(self, event):
        date = datetime.datetime.now()
        print "%s: %s - %s deleted" % (date, event.path, event.name)


handler = EventHandler()
notifier = pyinotify.Notifier(wm, handler)
wdd = wm.add_watch('/var/rudder/inventories/', mask, rec=True)

notifier.loop()
