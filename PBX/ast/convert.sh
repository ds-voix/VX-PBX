#!/bin/sh
# Convert incoming images to the acceptable for SendFax() TIFF format
umask 077

result='OK'
code=0

dir=`echo ${1} | /bin/sed -r 's/(^.+\/)(.+$)/\1/'`

echo ${dir} | /bin/grep '/' > /dev/null 2>&1
if [ $? -eq 0 ]; then
 dir=`echo "${dir}" | /bin/sed 's/\/$//'`
 file=`echo "${1}" | /bin/sed -r 's/(^.+\/)(.+$)/\2/'`
else
 dir="/var/spool/asterisk/fax"
 file="$1"
fi

function pdf () {
 # Convert PDF attachment to TIFF in a right format through GhostScript
 err=""
 name="`echo -n ${1} | /bin/sed -r 's/\.pdf$//i'`"

 /bin/cat "${dir}/${1}" | /usr/bin/gs -q -sDEVICE=tiffg3 -sPAPERSIZE=a4 -r204x196 -dNOPAUSE -sOutputFile="${dir}/${name}.tif" - > /dev/null 2>&1
# /bin/cat "${dir}/${file}" | /usr/bin/gs -q -sDEVICE=tiffg3 -r600  -dNOPAUSE -sOutputFile="${dir}/${tif}" -
 code=$?
 [ ${code} -ne 0 ] && err="ERROR:PDF"
# Using convert leads to over-memory crash on big documents !!!
# /usr/bin/convert -define quantum:polarity=min-is-white -rotate "90>" -density 204x196 -compress Group4 -type bilevel -monochrome "${dir}/${tif}" "${dir}/${tif}"
# [ $? -ne 0 ] && exit 1
 if [ -z "${err}" ]; then
  echo "${dir}/${name}.tif"
 else
  echo "${err}"
 fi
 # Finally, remove prototype unconditionally!
 /bin/rm -f "${dir}/${1}" > /dev/null 2>&1
}

function htm () {
 # Convert HTML attachment to PDF through WebKit
 err=""
 name="`echo -n ${1} | /bin/sed -r 's/\.html?$//i'`"
 /usr/local/sbin/wkhtmltopdf "${dir}/${1}" "${dir}/${name}.pdf" > /dev/null 2>&1
 code=$?
 [ ${code} -ne 0 ] && err="ERROR:HTM"
 if [ -z "${err}" ]; then
  pdf ${name}.pdf
 else
  echo "${err}"
 fi
 # Finally, remove prototype unconditionally!
 /bin/rm -f "${dir}/${1}" > /dev/null 2>&1
 /bin/rm -Rf "${dir}/${name}_files" > /dev/null 2>&1
}

function img () {
 # Convert graphic attachment to HTML wrap through WebKit
 err=""
 name="`echo -n ${1} | /bin/sed -r 's/\.[a-zA-z0-9]+$//i'`"
 echo "<html><img src="'"'"${dir}/${1}"'"'"></html>" > ${dir}/${name}.htm
# /usr/bin/convert "${dir}/${1}" "${dir}/${name}.pdf"
 htm ${name}.htm
 [ ${code} -ne 0 ] && err="ERROR:IMG"

 # Finally, remove prototype unconditionally!
 /bin/rm -f "${dir}/${1}" > /dev/null 2>&1
}

function txt () {
 # Convert text attachment to HTML wrap through WebKit
 err=""
 name="`echo -n ${1} | /bin/sed -r 's/\.(txt|log)$//i'`"
 echo "<html><head>" > ${dir}/${name}.htm
 echo '<meta http-equiv="content-type" content="text/html; charset='`/usr/bin/enca -m "${dir}/${1}"`'"></head>' >> ${dir}/${name}.htm
 echo '<body><font face="Courier New, Courier, monospace"><pre>' >> ${dir}/${name}.htm
 /bin/cat "${dir}/${1}" >> ${dir}/${name}.htm
 #| /bin/sed 's/$/<br>/g' | /bin/sed 's/[ ]/\&nbsp;/g' | /bin/sed 's/\t/\&nbsp;\&nbsp;\&nbsp;\&nbsp;/g'
 echo "</pre></font></body></html>" >> ${dir}/${name}.htm
 htm ${name}.htm
 [ ${code} -ne 0 ] && err="ERROR:TXT"

 # Finally, remove prototype unconditionally!
 /bin/rm -f "${dir}/${1}" > /dev/null 2>&1
}

function eml () {
 # Parse EMail MIME to find & process the attachment to be sent as fax
 parts=""
 old_IFS=$IFS      # save the field separator
 IFS=$'\n'     # new field separator, the end of line
 for p in `/usr/bin/munpack -t -C "${dir}" $1`
 do
  m=`echo "${p}" | /bin/sed -r 's/(.*\()(.+)(\))/\2/'`
  f=`echo "${p}" | /bin/cut -d ' ' -f 1`
  m1=`/usr/bin/file -i ${dir}/${f} | /bin/cut -d ' ' -f 2 | /bin/cut -d ';' -f 1`
  parts="${f}${IFS}${parts}"
#  echo ${m1} ${m}
  case "${m}" in
   application/pdf) xpdf=${f};;
   text/html)
    if [ "${m1}" == "${m}" ]; then
     xhtm=${f}
    else
     xtxt=${f}
    fi
    ;;
   text/x-news*)
    err=ERROR:NESTED_MSG
    ;;
   text*) xtxt=${f};;
   image*) ximg=${f};;
   *)
    err=ERROR:UNSUPPORTED_ENCLOSURE
    ;;
  esac
 done
 IFS=$old_IFS     # restore default field separator

 if [ -z "${err}" ]; then
  # Try to send the most meaningfull part as FAX
  if [ "${xpdf}" != "" ]; then
   pdf ${xpdf}
  else
   if [ "${ximg}" != "" ]; then
    img ${ximg}
   else
    if [ "${xhtm}" != "" ]; then
     htm ${xhtm}
    else
     if [ "${xtxt}" != "" ]; then
      txt ${xtxt}
     fi
    fi
   fi
  fi
 else
  ${code}=255
  echo ${err}
 fi

 IFS=$'\n'     # new field separator, the end of line
 for p in ${parts}
 do
#  echo "${dir}/${p}"
  /bin/rm -f "${dir}/${p}" > /dev/null 2>&1
 done
 IFS=$old_IFS     # restore default field separator
 # Finally, remove prototype unconditionally!
 /bin/rm -f "${dir}/${1}" > /dev/null 2>&1
}

if [ ! -f "${dir}/${file}" ]; then
 echo ERROR:NOT_FOUND
 exit 1
fi

case `echo ${file} | /bin/sed -r 's/(.*\.)(.+$)/\2/'` in
 pdf) pdf ${file};;
 htm) htm ${file};;
 txt) txt ${file};;
 mbox|mbx|eml) eml ${file};;
 *)
  mime=`/usr/bin/file -i ${dir}/${file} | /bin/cut -d ' ' -f 2`
  case "${mime}" in
   application/pdf) pdf ${file};;
   text/html) htm ${file};;
   text/x-news*) eml ${file};;
   text*) txt ${file};;
   image*) img ${file};;
   *)
    echo ERROR:INVALID_TYPE
    exit 1
   ;;
  esac
 ;;
esac

exit ${code}
