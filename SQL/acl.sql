/*
 VX-PBX routing ACL @ postgresql
 Copyright (C) 2012-2015 Dmitry Svyatogorov ds@vo-ix.ru

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

-- Mangle rules
CREATE TABLE "Mangle" (
 "NRec" bigserial PRIMARY KEY,             -- Binding key
 "BIND" text NULL,
 "SrcCutPref" smallint DEFAULT 0 NOT NULL, -- Strip first|last chars
 "DstCutPref" smallint DEFAULT 0 NOT NULL,
 "SrcCutSuff" smallint DEFAULT 0 NOT NULL,
 "DstCutSuff" smallint DEFAULT 0 NOT NULL,
 "SrcAddPref" text DEFAULT '' NOT NULL,    -- Add prefix|suffix
 "DstAddPref" text DEFAULT '' NOT NULL,
 "SrcAddSuff" text DEFAULT '' NOT NULL,
 "DstAddSuff" text DEFAULT '' NOT NULL,
 -- http://www.postgresql.org/docs/8.4/static/functions-matching.html#POSIX-EMBEDDED-OPTIONS-TABLE
 -- regexp_replace(source, pattern, replacement [, flags ])
 -- SELECT regexp_replace('+a0b1cc098*', '[^0-9]', '' , 'g'); >> '01098'
 -- SELECT regexp_replace('88129583253', '(^.*)?([0-9]{7}$)', '7812\\2' , ''); >> '78129583253'
 "SrcRE" text DEFAULT '' NOT NULL,         -- Mangle src, according to this regular expression
 "DstRE" text DEFAULT '' NOT NULL,         -- Mangle dst, according to this regular expression

 "Description" text NULL                   -- Optional comments
);

-- ACL implementation
CREATE TABLE "ACL" (
 "NRec" bigserial PRIMARY KEY,
 "BIND" text NULL,                         -- Binding key
 "Default" boolean NOT NULL DEFAULT False, -- Default policy. False to deny by default.
 "Description" text NULL                   -- Optional comments
);

-- ACL Lines
CREATE TABLE "ACLines" (
 "NRec" bigserial PRIMARY KEY,             -- Binding key
 "ACL" bigint NOT NULL REFERENCES "ACL" ON UPDATE CASCADE ON DELETE CASCADE,
 "Order" smallint DEFAULT 0 NOT NULL,      -- Processing order
 "Allow" boolean NOT NULL DEFAULT True,    -- Allow|Deny
 "FromExten" bigint DEFAULT 0 NOT NULL,    -- Corresponding numeric interval from..to
 "ToExten" bigint DEFAULT 9223372036854775807 NOT NULL,
 "Description" text NULL                   -- Optional comments
);

-- ACL Lines
CREATE TABLE "ACRegex" (
 "NRec" bigserial PRIMARY KEY,             -- Binding key
 "ACL" bigint NOT NULL REFERENCES "ACL" ON UPDATE CASCADE ON DELETE CASCADE,
 "Order" smallint DEFAULT 0 NOT NULL,      -- Processing order
 "Allow" boolean NOT NULL DEFAULT True,    -- Allow|Deny
 "NOT" boolean NOT NULL DEFAULT False,     -- Invert regexp
 "RE" text NULL DEFAULT '.*',              -- Regular expression to match
 "Description" text NULL                   -- Optional comments
);
