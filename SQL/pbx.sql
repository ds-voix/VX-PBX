/*
 VX-PBX bone @ postgresql
 Copyright (C) 2009-2015 Dmitry Svyatogorov ds@vo-ix.ru

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License as
    published by the Free Software Foundation, either version 3 of the
    License, or (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

-- Some significant constants must be stored here
CREATE TABLE "CONST" (
 "Name" text PRIMARY KEY,                 -- Name to find
 "BIND" text NULL,                        -- Lock constants to those with the same binding or NULL-binded ones
 "Value" text,                            -- Some value to be stored
 "Description" text NULL                  -- Optional comments
);

-- Place for cross-reboot variables
CREATE TABLE "VAR" (
 "Name" text PRIMARY KEY,                 -- Name to find
 "BIND" text NULL,                        -- Lock variables to those with the same binding or NULL-binded ones
 "Value" text,                            -- Some value to be stored
 "Description" text NULL                  -- Optional comments
);

-- Lists of allowed city|country prefixes
CREATE TABLE "PrefixList" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "BIND" text NULL,
 "Description" text NULL                  -- Optional comments
);

-- City|country prefixes for PrefixList
CREATE TABLE "Prefixes" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "PrefixList" bigint NOT NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 "Code" text NOT NULL,                    -- City|country code
 "Length" smallint DEFAULT 0 NOT NULL,    -- Check the rest number length if >0
 "Description" text NULL                  -- Optional comments
);

-- Lists for define serial|group calls
CREATE TABLE "CallList" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "BIND" text NULL,
 "Serial" boolean DEFAULT FALSE NOT NULL, -- Whether to make serial call
 "Timeout" int DEFAULT 15 NOT NULL,  -- Dialing timeout
 "Description" text NULL                  -- Optional comments
);

-- Numbers for CallList
CREATE TABLE "Calls" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "CallList" bigint NOT NULL REFERENCES "CallList" ON UPDATE CASCADE ON DELETE CASCADE,
 "Exten" text NOT NULL,
 "Type" text DEFAULT 'SIP' NOT NULL,      -- Where to look for ('SIP' 'IAX' 'DAHDI/g1')
 "Order" smallint DEFAULT 0 NOT NULL,     -- Processing order
 "Timeout" int NULL,                      -- Specific dialing timeout
 "Description" text NULL                  -- Optional comments
);

-- The way to implement dynamic sorting on static lists
CREATE TABLE "DynamicSort" (
 "List" text,                       -- Name for sorting group
 "NRec" bigint,                     -- Referenced by this NRec in some table
 "Order" bigint DEFAULT 0 NOT NULL  -- Current position
);

-- Schedules for time-depending actions
-- Lists for define Schedule
CREATE TABLE "Schedule" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "BIND" text NULL,
 "NOT" boolean NOT NULL DEFAULT False,    -- Invert match
 "Description" text NULL                  -- Optional comments
);

-- Common-purposed schedules
CREATE TABLE "Schedules" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "Schedule" bigint NOT NULL REFERENCES "Schedule" ON UPDATE CASCADE ON DELETE CASCADE,
 "NOT" boolean NOT NULL DEFAULT False,    -- Invert match
 "TimeRange" text DEFAULT '*' NOT NULL,   -- '11:05-15:30'|*
 "DaysOfWeek" text DEFAULT '*' NOT NULL,  -- 'mon'|'sun-sat'|'*'
 "DaysOfMonth" text DEFAULT '*' NOT NULL, -- '12'|'1-31'|'*'
 "Month" text DEFAULT '*' NOT NULL,       -- 'jan'|'mar-aug'|'*'
 "Year" text DEFAULT '*' NOT NULL,        -- Optional year number - not in asterisk time spec
 "Description" text NULL                  -- Optional comments
);

-- Internal extensions list
CREATE TABLE "Exten" (
 "Exten" text,                            -- Uniquie number, i.e. '28525', or 'bar@foo' in case of internal refs
 "BIND" text NULL,                        -- Lock available extensions to those with the same binding or NULL-binded ones
 "Type" text DEFAULT 'SIP' NOT NULL,      -- Where to look for ('SIP' 'IAX' 'DAHDI/g1')
 "Enabled" boolean DEFAULT TRUE NOT NULL, -- If no, then 'temporary unavailable' or just hangup
 "Context" text NULL,                     -- If defined, then Goto(context,s,1)
 "MailTo" text NULL,                      -- Send VM|FAX|etc. here
 "MonitorTo" text NULL,                   -- If defined then do call monitoring and store files here
 -- Which calls are accepted:
 /* -1 = nothing except '112'
    0 = aliaces only
    1 = +internal
    2 = +partners
    3 = +city
    4 = +zone
    5 = +country
    6 = +international
    7 = no restrictions at all
 */
 "CallLevel" smallint DEFAULT 4 NOT NULL,
 "CallLimit" smallint DEFAULT 1 NOT NULL, -- Limit simultaneous incoming calls
 -- Allow only short city numbers from this list, if defined
 "ServiceList" bigint NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 -- Lists of allowed prefixes for 5,6 call levels
 "PrefixList5" bigint NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 "PrefixList6" bigint NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 -- Allow federal calls to listed zones at level 5
 "ZoneList" bigint NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 -- List for serial|group call implementing (if defined)
 "CallList" bigint NULL REFERENCES "CallList" ON UPDATE CASCADE ON DELETE CASCADE,
 "DND" boolean default FALSE NOT NULL,    -- Do Not Disturb (always busy for incoming calls)
 "Delay" int DEFAULT 0 NOT NULL,          -- Auto-DND pending Delay seconds after last call completed
 "TransferCall" text NULL,                -- Unconditional transfer
 "SpawnCalls" text NULL,                  -- Direct spawn of simultaneous calls: "ext1[:timeout1],ext2[:timeout2]...,extN[:timeoutN]"
 "TransferOnBusy" text NULL,              -- Transfer, if busy
 "TransferOnTimeout" text NULL,           -- Transfer, if no answer
 -- Implement transfers based on sheduling list
 "TransferSchedule" bigint NULL REFERENCES "Schedule" ON UPDATE CASCADE ON DELETE CASCADE,
 "FeatureTransfer" boolean DEFAULT TRUE NOT NULL, -- Use t option when Dial()
 "FeaturePark" boolean DEFAULT TRUE NOT NULL,     -- Use k option when Dial()
 -- Access to IVR options
 "IVRTransfer" boolean DEFAULT TRUE NOT NULL,
 "IVRMail" boolean DEFAULT TRUE NOT NULL,
 "Timeout" int DEFAULT 90 NOT NULL,       -- Dial() timeout
 -- Search alias in this group. If 'DISABLED', prohibit aliaces except 'GLOBAL'
 "Alias" text DEFAULT 'DISABLED',
 -- Permit automatic dialing function. Must even not be proposed, but... ;)
 "AutoDial" boolean DEFAULT FALSE NOT NULL,
 "MOH" text DEFAULT '' NOT NULL,          -- MOH class
 "RouteTag" text DEFAULT NULL,            -- Set this tag and try to reach unavailable exten through routing schema (if NOT NULL)
 "RouteGroup" text DEFAULT NULL,          -- The same as Route's "CallGroup". Thus, limit TOTAL (IN+OUT) simultaneous calls
 "Description" text NULL                  -- Optional comments
);

-- Reverse calls list
CREATE TABLE "CallBack" (
 "BIND" text NULL,
 "Exten" text NOT NULL,                   -- Incoming call to number
 "CID" text NOT NULL,                     -- CallerID number from incoming
 "CallerID" text NOT NULL,                -- CallerID number for outgoing
 "Channel" text NOT NULL,                 -- Do reverse call to ('DAHDI/g1/8XXXxxxxxxx')
 "WaitTime" smallint DEFAULT 15 NOT NULL, -- Timeout for Dial()
 -- Specify context,extension at priority from which CallBack  starts
 "Context" text DEFAULT 'callback' NOT NULL,
 "Extension" text NOT NULL,
 "Priority" text DEFAULT '1' NOT NULL,
 "AlwaysDelete" text DEFAULT 'yes' NOT NULL, -- There may be troubles whith 'no'...
 "Archive" text DEFAULT 'yes' NOT NULL,   -- Save last call in ./outgoing_done/
 "Timeout" smallint DEFAULT 1 NOT NULL,   -- Wait between hangup and CallBack placing
 "Description" text NULL,                 -- Optional comments
 PRIMARY KEY ("Exten","CID")
);

-- Functions list: do Macro(parameters) if match
CREATE TABLE "Func" (
 "Exten" text NOT NULL,                   -- Incoming call to number
 "CID" text DEFAULT 'ALL' NOT NULL,       -- CallerID number from incoming, set to 'ALL' for remove this constraint
 "BIND" text NULL,                        -- Lock available functions to those with the same binding or NULL-binded ones
 "Macro" text NOT NULL,                   -- Macro name to be called
 "P1" text NULL,                          -- Parameters, if some are needed
 "P2" text NULL,                          -- Parameters, if some are needed
 "P3" text NULL,                          -- Parameters, if some are needed
 "P4" text NULL,                          -- Parameters, if some are needed
 "Description" text NULL                 -- Optional comments
-- PRIMARY KEY ("Exten","BIND","CID")
);

-- Aliases for egress|ingress calls.
CREATE TABLE "Aliases" (
 "Exten" text DEFAULT 'GLOBAL' NOT NULL,  -- Alias owner extension, or group name
 "BIND" text NULL,                        -- Lock aliases to those with the same binding or NULL-binded ones
 "Cell" text NOT NULL,                    -- Cell number, by default from 1 to 99
 "Alias" text NOT NULL,                   -- Call this number
 "Bypass" boolean DEFAULT FALSE NOT NULL, -- Whether to check caller prvelegues (Exten.CallLevel)
 "Egress" boolean DEFAULT TRUE NOT NULL,  -- Implement on egress calls (default behaviour)
 "Ingress" boolean DEFAULT FALSE NOT NULL,-- Implement on ingress calls (make virt.offices etc.)
 "Label" text NULL,                       -- Path to voice label './exten-cell.gsm'
 "Description" text NULL                 -- Optional comments
-- PRIMARY KEY ("BIND","Exten","Cell")
);

-- IAX|SIP|DAHDI partners list
CREATE TABLE "Friends" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "BIND" text NULL,                        -- Lock available friends to those with the same binding or NULL-binded ones
 -- Pull of numbers to be placed onto the channel
 "FromExten" bigint NOT NULL,
 "ToExten" bigint NOT NULL,
 -- Channel specification for doing Dial(), i.e. 'IAX2/MIRAN'
 "Channel" text DEFAULT 'IAX2' NOT NULL,
 -- Currently, Asterisk has no regex  replacements, so I propose the features below...
 "Cut" smallint DEFAULT 0 NOT NULL,       -- Cut first N digits from calling number
 "Prefix" text NULL,                      -- Add this prefix ..
 "Suffix" text NULL,                      -- .. and|or this suffix
 "Offset" int DEFAULT 0 NOT NULL,         -- Add|substract this value (make arithmetical offset)
 "Description" text NULL                  -- Optional comments
);

-- List of federal codes for local zone (DEF + number pool)
CREATE TABLE "Zone" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "Zone" int NOT NULL,                     -- City code (i.e., 812 for SPb or 495 for Msk)
 "DEF" int NOT NULL,                      -- DEF code, i.e. '921'
 -- Pull of numbers are placed in local zone
 "FromExten" int DEFAULT 0 NOT NULL,
 "ToExten" int DEFAULT 9999999 NOT NULL,
 "Description" text NULL                  -- Optional comments
);

-- Channels through which to route outgoing calls
CREATE TABLE "Channels" (
 "NRec" bigserial PRIMARY KEY,            -- Binding key
 "BIND" text NULL,
 "Channel" text NOT NULL,
 "Description" text NULL                  -- Optional comments
);

-- Routing table for outgoing calls
CREATE TABLE "Route" (
 "NRec" bigserial PRIMARY KEY,               -- Binding key
 "SourceLow" bigint DEFAULT 0 NOT NULL,      -- Source extensions interval
 "SourceHigh" bigint DEFAULT 9223372036854775807 NOT NULL,
 "DestinationLow" bigint DEFAULT 0 NOT NULL, -- Destination extensions interval
 "DestinationHigh" bigint DEFAULT 9223372036854775807 NOT NULL,
 "Channel" bigint NOT NULL REFERENCES "Channels" ON UPDATE CASCADE ON DELETE CASCADE,
 "Level" smallint DEFAULT 0 NOT NULL,        -- Preferred route for this call level
 "Order" smallint DEFAULT 0 NOT NULL,        -- Searching order
 "NoMore" boolean DEFAULT False NOT NULL,    -- Stop searching
 "SrcACL" bigint NULL REFERENCES "ACL" ON UPDATE CASCADE ON DELETE SET NULL,    -- Complex filtering by ACL
 "DstACL" bigint NULL REFERENCES "ACL" ON UPDATE CASCADE ON DELETE SET NULL,
 "Mangle" bigint NULL REFERENCES "Mangle" ON UPDATE CASCADE ON DELETE SET NULL, -- Mangle rule for additional perversions
 "SetCID" text NULL,                                                            -- Set this CID when passing through this route
 -- Ignore "CID" table entry absense for originator, so be completelly insecure
 "Insecure" boolean DEFAULT False NOT NULL,
 "TAG" text NULL,                            -- Special filter by the TAG (source-routing etc.)
 "BIND" text NULL,                           -- Lock available routes to those with the same binding or NULL-binded ones
 "NAT" interval NULL,                        -- Store the rule for reverse call along this time
 "RingTime" smallint NULL,                   -- Overwrite system constant
 "CallTime" bigint NULL,                     -- Drop call after this time, seconds
 "HangupDelay" smallint NULL,                -- Delayed hangup
 "CallLimit" smallint DEFAULT 0 NOT NULL,    -- Limit maximum calls through this route
 "CallGroup" text NULL,                      -- Group routes with common limit etc.
 "Ingress" boolean DEFAULT False NOT NULL,   -- Ingress route, accountcode must be set from DNID (leg B)
 "RDNIS" text NULL,                          -- Set RDNIS exactly to this value, if present
 "Description" text NULL                     -- Optional comments
);

-- Pull of numbers with the same external CID
CREATE TABLE "CID" (
 "NRec" bigserial PRIMARY KEY,                -- Binding key
 "BIND" text NULL,                            -- Lock available CID to those with the same binding or NULL-binded ones
 "FromExten" bigint NOT NULL,
 "ToExten" bigint NOT NULL,
 "CID" text NOT NULL,                         -- CID to set
 "Description" text NULL                      -- Optional comments
);

-- Menu tree structure
CREATE TABLE "Menu" (
 "NRec" bigserial PRIMARY KEY,                -- Binding key
 "BIND" text NULL,
 "Parent" bigint NULL REFERENCES "Menu" ON UPDATE CASCADE ON DELETE SET NULL,
 "Hello" text NULL,                   -- Take voice hello from here (play once)
 "Prompt" text NOT NULL,              -- Take voice prompt from here (loop)
 "0" text NULL,
 "1" text NULL,
 "2" text NULL,
 "3" text NULL,
 "4" text NULL,
 "5" text NULL,
 "6" text NULL,
 "7" text NULL,
 "8" text NULL,
 "9" text NULL,
 "*" text DEFAULT 'BACK' NULL,           -- Back to previous menu by *
 "#" text DEFAULT 'BACK:BACK:BACK' NULL, -- Back to root menu by #, m.b. although BACK:SomeFunc(Foo,bar,,)
 "Timeout" smallint DEFAULT 10 NOT NULL,
 "TimeoutAction" text NULL,
 "Repeat" smallint DEFAULT 10 NOT NULL,
 "Description" text NULL              -- Optional comments
);

-- Time zone lists for time-based switching
CREATE TABLE "TimeZones" (
 "NRec" bigserial PRIMARY KEY,                -- Binding key
 "Description" varchar(256) NULL              -- Optional comments
);

-- Prefix to time zone bindings
CREATE TABLE "TZ" (
 "TimeZones" bigint NOT NULL REFERENCES "TimeZones" ON UPDATE CASCADE ON DELETE CASCADE,
 "Prefix"varchar(16) NOT NULL,                -- Particular prefix
 "Shift" smallint DEFAULT 0 NOT NULL         -- +- minutes to local time (or, m.b., GMT?)
);

-- Calendars for time-based switching
CREATE TABLE "Calendars" (
 "NRec" bigserial PRIMARY KEY,                -- Binding key
 "Enabled" boolean DEFAULT TRUE NOT NULL,     -- Allow to disable for particular calendar
 "TimeZones" bigint NULL REFERENCES "TimeZones" ON UPDATE CASCADE ON DELETE SET NULL,
 "Description" varchar(256) NULL              -- Optional comments
);

CREATE TABLE "Calendar" (
 "Calendars" bigint NOT NULL REFERENCES "Calendars" ON UPDATE CASCADE ON DELETE CASCADE,
 "DT" date NOT NULL,                          -- This day
 "From" time DEFAULT '00:00' NOT NULL,        -- Valid from time
 "To" time DEFAULT '00:00' NOT NULL           --     ... to time
);

-- Source routing scheme
CREATE TABLE "RoutingList" (
 "NRec" bigserial PRIMARY KEY,                -- Binding key
 "Description" text NULL                      -- Optional comments
);

CREATE TABLE "Routing" (
 "NRec" bigserial PRIMARY KEY,                -- Binding key
 "RoutingList" bigint NOT NULL REFERENCES "RoutingList" ON UPDATE CASCADE ON DELETE CASCADE,
 -- do "CheckPrefix"("PrefixList" bigint, "Exten" varchar(20)) against this list
 "PrefixList5" bigint NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 "PrefixList6" bigint NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 -- do "InZoneList"("List" bigint, "DEF" smallint, "Exten" int) against this list
 "ZoneList" bigint NULL REFERENCES "PrefixList" ON UPDATE CASCADE ON DELETE CASCADE,
 "Route" text NOT NULL,                       -- Take this route spec. in case of any list matches
 "Description" text NULL                      -- Optional comments
);

-- AutoDial list
CREATE TABLE "AutoDial" (
 "Src" text NULL,                       -- Caller
 "Dst" text NULL,                       -- Callee
 "PlacedAt" timestamp NOT NULL,                -- When this record have been placed
 "LastCall" timestamp DEFAULT 'epoch' NOT NULL, -- Last attempt done at
 "NoAnswer" smallint NOT NULL DEFAULT 0,       -- 'NO ANSWER' count
 "Busy" smallint NOT NULL DEFAULT 0            -- 'BUSY' count
);

-- NAT implementation
CREATE TABLE "NAT" (
 "NRec" bigserial PRIMARY KEY,
 "Line" text NOT NULL,                      -- exten, etc.
 "BIND" text NULL,                          -- domain, if one
 "CID" text NOT NULL,                       -- Egress CallerID
 "DNID" text NOT NULL,                      -- Dialed number
 "DT" timestamp NOT NULL,                   -- Last update
 "Valid" interval default '1 day' NOT NULL  -- Optional comments
);

-- BlackList implementation
CREATE TABLE "BlackList" (
 "NRec" bigserial PRIMARY KEY,
 "Line" text,                               -- exten, etc.
 "BIND" text NULL,                          -- domain, if one
 "CID" text NULL,                           -- BlackList this one
 "DT" timestamp,                            -- Last update
 "Description" text NULL                    -- Optional comments
);

-- Dialed extensions statuses, for quick analysis
drop table "LineStatus" CASCADE;
CREATE TABLE "LineStatus" (
 "Line" text,                               -- exten, etc.
 "BIND" text NULL,                          -- domain, if one
 "Status" text NULL,                        -- Optional comments
 "DT" timestamp                             -- Last update
);

-- Send events
CREATE TABLE "Event" (
 "NRec" bigserial PRIMARY KEY,
 "Line" text,                               -- exten, etc.
 "BIND" text NULL,                          -- domain, if one
 "URL" text NULL,                           -- Notify thhis URL via http POST
 "Handler" text default 'Event' NOT NULL,   -- Macro to be called
 "Description" text NULL                    -- Optional comments
);

-- ABC+DEF
CREATE TABLE "ABC" (
 "Area" smallint NOT NULL,                    -- Area prefix, e.g. '7' for Russia (Abkhasia, Kasachstan although)
                                              -- http://ru.wikipedia.org/wiki/%D0%A2%D0%B5%D0%BB%D0%B5%D1%84%D0%BE%D0%BD%D0%BD%D1%8B%D0%B9_%D0%BA%D0%BE%D0%B4_%D1%81%D1%82%D1%80%D0%B0%D0%BD%D1%8B
 "ABC" smallint NOT NULL,                     -- ABC/DEF code, e.g. '812' or '921'
 -- Pull of numbers are placed in
 "From" int DEFAULT 0 NOT NULL,
 "To" int DEFAULT 9999999 NOT NULL,
 "Owner" text NULL,                           -- Operator
 "Region" text NOT NULL                       -- Region
);
