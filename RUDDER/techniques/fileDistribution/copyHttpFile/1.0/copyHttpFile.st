#####################################################################################
# Copyright 2018 Dmitry Svyatogorov
#####################################################################################
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, Version 3.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#####################################################################################

#######################################################
# Download a file from an arbitrary http/https server #
#######################################################

bundle agent download_from_http
{
  vars:
# HTTP_FILE_PATH
    &HTTP_FILE_PATH:{ fname | "get_file[&i&][name]" string => "&fname&" ;
}&
# TRACKINGKEY
    &TRACKINGKEY:{ id | "get_file[&i&][uuid]" string => "&id&";
}&
# HTTP_FILE_URL
    &HTTP_FILE_URL:{ src | "get_file[&i&][url]" string => "&src&";
}&

# HTTP_FILE_MODE
    &HTTP_FILE_MODE:{ fm | "get_file[&i&][mode]" string => "&fm&";
}&

# HTTP_FILE_USER
    &HTTP_FILE_USER:{ fu | "get_file[&i&][owner]" string => "&fu&";
}&

# HTTP_FILE_GROUP
    &HTTP_FILE_GROUP:{ fg | "get_file[&i&][group]" string => "&fg&";
}&

# HTTP_FILE_POST_HOOK_RUN
    &HTTP_FILE_POST_HOOK_RUN:{ run | "get_file[&i&][posthook_run]" string => "&run&";
}&

# HTTP_FILE_POST_HOOK_COMMAND
    &HTTP_FILE_POST_HOOK_COMMAND:{ command | "get_file[&i&][posthook_cmd]" string => "&command&";
}&

    "index" slist => getindices("get_file");


  methods:
    "any" usebundle => download_from_http_exec("${index}",
                         "${get_file[${index}][uuid]}",
                         "${get_file[${index}][name]}",
                         "${get_file[${index}][url]}",
                         "${get_file[${index}][mode]}",
                         "${get_file[${index}][owner]}",
                         "${get_file[${index}][group]}",
                         "${get_file[${index}][posthook_run]}",
                         "${get_file[${index}][posthook_cmd]}");

} # bundle agent download_from_http


body contain shell_settings
{
    useshell => "useshell";
    exec_timeout => "1800";
}

#                                    Theese incoming vars are only local ones. All the rest are GLOBAL, so use "_${index}" suffix everywhere!!!
bundle agent download_from_http_exec(index, uuid, file_name, url, file_mode, file_owner, file_group, posthook_run, posthook_cmd)
{
  vars:
    "path_to_metadata" string => '/var/rudder/cfengine-community/state/previous_state/';

    # Reporting messages (message, hook_message)
    # !!! error+ok may co-exist, so messages are overriden!!!
    "!(file_perms_up_to_date_${index}|file_perms_reparied_${index}|file_perms_error_${index})"::
      "message_${index}" string => '';
    "file_perms_up_to_date_${index}"::
      "message_${index}" string => '. Permissions ok';
    "file_perms_reparied_${index}"::
      "message_${index}" string => '. Permissions repaired';
    "file_perms_error_${index}"::
      "message_${index}" string => '. Error in permissions: ${file_mode}, ${file_owner}:${file_group}';

    "!(posthook_ok_${index}|posthook_error_${index})"::
      "hook_message_${index}" string => '';
    "posthook_ok_${index}"::
      "hook_message_${index}" string => 'Hook executed';
    "posthook_error_${index}"::
      "hook_message_${index}" string => 'Error in hook execution';

  classes:
    "posthook_run_enabled_${index}" expression => strcmp("${posthook_run}", "true");
    "file_exists_${index}" expression => fileexists("${file_name}");

    any::
      "pass3_${index}" expression => "pass2_${index}";
      "pass2_${index}" expression => "pass1_${index}";
      "pass1_${index}" expression => "any";

    # Set 3 report classes to be mutually exclusive. Error extrudes all other reports, and repaired takes precedence over ok.
    "pass3_${index}"::
      "file_error_${index}" expression    => "(url_error_${index}|file_perms_error_${index})";
      "file_repaired_${index}" expression => "!file_error_${index}.(file_reparied_${index}|file_perms_reparied_${index})";
      "file_ok_${index}" expression => "!(file_error_${index}|file_repaired_${index}).(file_up_to_date_${index}.file_perms_up_to_date_${index})";

  commands:
# curl --limit-rate <speed>(k|M|G) ### bytes/second
# ^context=xyz sets the module context to xyz instead of the default for any following definitions
      "PATH=\"/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/var/rudder/cfengine-community/bin\"
echo '^context=curl_${index}'
state_dir=`dirname '${path_to_metadata}/copyHttpFile/${file_name}'`
state_file='${path_to_metadata}/copyHttpFile/${file_name}'

if ! type curl; then
  echo '+url_error_${index}'
  echo '=message_${index}=\"curl\" utility not found'
  exit
fi

dt=`curl -s --head '${url}' | grep -i -- 'Last-Modified:'`
if [ $? -ne 0 ]; then
  echo '+url_error_${index}'
  echo '=message_${index}=Error retrieving URL'
  exit
fi

if [ ! -f '${file_name}' ]; then
  rm -f \"$state_file\"
fi
dt1=`cat \"$state_file\" 2>/dev/null`

if [ \"${dt}\" != \"${dt1}\" ]; then
  if ! touch '${file_name}.${uuid}'; then
    echo '+url_error_${index}'
    echo '=message_${index}=Unable to write destination'
    exit
  fi

  if curl --limit-rate &CURL_SPEED_MB&M -L -s -C - '${url}' > '${file_name}.${uuid}'; then
    chmod ${file_mode} '${file_name}.${uuid}'
    chown ${file_owner}:${file_group} '${file_name}.${uuid}'
    mv -f '${file_name}.${uuid}' '${file_name}'
    mkdir -p \"$state_dir\"
    echo \"${dt}\" > \"$state_file\"
    echo '+file_reparied_${index}'
    echo '=message_${index}=File downloaded'
  else
    rm -f '${file_name}.${uuid}'
    echo '+url_error_${index}'
    echo '=message_${index}=Error retrieving file from URL'
  fi
else
  echo '+file_up_to_date_${index}'
  echo '=message_${index}=File is up-to-date'
fi
"
      contain    => shell_settings,
      module => "true";

    "posthook_run_enabled_${index}"::
      "${posthook_cmd}"
        contain    => shell_settings,
        classes    => if_else("posthook_ok_${index}", "posthook_error_${index}"),
        ifvarclass => "file_reparied_${index}",
        comment    => "Execute the posthook command if a file was changed";


  files:
    "file_exists_${index}"::
      "${file_name}"
        pathtype => "literal", 	# force literal interpretation
        perms => mog(${file_mode}, ${file_owner}, ${file_group}),
        # !!! perms and owner sets classes simultaneously!!!  M.b."kept+fail" at the same time!
        classes => kept_if_else("file_perms_up_to_date_${index}", "file_perms_reparied_${index}", "file_perms_error_${index}");


  methods:
  ### REPORTING ###
  # <TML name="copyHttpFile">
  # Section #1 "Download file from HTTP":
    "file_error_${index}"::
      # Error:
      "any" usebundle => rudder_common_report_index("copyHttpFile", "result_error", "${uuid}", "Download file from HTTP", "${file_name}", "${curl_${index}.message_${index}}${message_${index}}", "${index}");
    "file_repaired_${index}"::
      # Repaired:
      "any" usebundle => rudder_common_report_index("copyHttpFile", "result_repaired", "${uuid}", "Download file from HTTP", "${file_name}", "${curl_${index}.message_${index}}${message_${index}}", "${index}");
    "file_ok_${index}"::
      # OK:
      "any" usebundle => rudder_common_report_index("copyHttpFile", "result_success", "${uuid}", "Download file from HTTP", "${file_name}", "${curl_${index}.message_${index}}${message_${index}}", "${index}");

  # Section #2 "Post-modification hook":
    "pass3_${index}.posthook_error_${index}"::
      # Error:
      "any" usebundle => rudder_common_report_index("copyHttpFile", "result_error", "${uuid}", "Post-modification hook", "${file_name}", "${hook_message_${index}}", "${index}");
    "pass3_${index}.posthook_ok_${index}"::
      # Repaired:
      "any" usebundle => rudder_common_report_index("copyHttpFile", "result_repaired", "${uuid}", "Post-modification hook", "${file_name}", "${hook_message_${index}}", "${index}");
      # OK (Hook execution skeeped)
    "pass3_${index}.!(posthook_ok_${index}|posthook_error_${index})"::
      "any" usebundle => rudder_common_report_index("copyHttpFile", "result_success", "${uuid}", "Post-modification hook", "${file_name}", "File is ok, hook was skipped", "${index}");


} # bundle agent download_from_http_exec
