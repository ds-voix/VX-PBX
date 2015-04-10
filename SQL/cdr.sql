-- ASTERISK CDR --
CREATE TABLE cdr (
 NRec bigserial NOT NULL primary key,
 calldate timestamp with time zone DEFAULT now() NOT NULL,
 clid text DEFAULT '' NOT NULL,
 src text DEFAULT '' NOT NULL,
 dst text DEFAULT '' NOT NULL,
 dcontext text DEFAULT '' NOT NULL,
 channel text DEFAULT '' NOT NULL,
 dstchannel text DEFAULT '' NOT NULL,
 lastapp text DEFAULT '' NOT NULL,
 lastdata text DEFAULT '' NOT NULL,
 duration bigint DEFAULT 0::bigint NOT NULL,
 billsec bigint DEFAULT 0::bigint NOT NULL,
 disposition text DEFAULT '' NOT NULL,
 amaflags bigint DEFAULT 0::bigint NOT NULL,
 accountcode text DEFAULT '' NOT NULL,
 uniqueid text DEFAULT '' NOT NULL,
 linkedid text DEFAULT '' NOT NULL,
 userfield text DEFAULT '' NOT NULL,
 -- Additional fields
 "x-tag" text NULL,               -- Device tag, if one
 "x-cid" text NULL,               -- Dialed cgpn
 "x-did" text NULL,               -- Initial cdpn
 "x-dialed" text NULL,            -- Really dialed
 "x-spec" text NULL,              -- Dialed channel specification
 "x-insecure" boolean NULL,       -- Bypass "CID" lookup
 "x-result" text NULL,            -- Error code
 "x-record" text NULL,            -- File containing call record, if one
 "x-domain" text NULL,            -- BIND
 "x-data" text ARRAY NULL         -- Call trace
);

-- app "Queue" log format
CREATE TABLE "queue_log" (
 "NRec" bigserial PRIMARY KEY,
 "DT" timestamp DEFAULT now() NOT NULL,
 "time" varchar(32) DEFAULT '' NOT NULL,
 "callid" varchar(32) DEFAULT '' NOT NULL,
 "queuename" varchar(128) DEFAULT '' NOT NULL,
 "agent" varchar(64) DEFAULT '' NOT NULL,
 "event" varchar(64) DEFAULT '' NOT NULL,
-- "data" varchar(128) DEFAULT '' NOT NULL
 "data1" varchar(128) DEFAULT '' NOT NULL,
 "data2" varchar(128) DEFAULT '' NOT NULL,
 "data3" varchar(128) DEFAULT '' NOT NULL,
 "data4" varchar(128) DEFAULT '' NOT NULL,
 "data5" varchar(128) DEFAULT '' NOT NULL
);
