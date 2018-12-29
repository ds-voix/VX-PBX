#!/bin/sh
#chan=`cat /root/chan.txt  | awk '{ if ($3 ~ "^(O|I|B)$") print $2 }'`

while :
do
  date
  dev=`asterisk -rx 'devstate list' | grep -v 'NOT_INUSE' | grep 'Custom:' | grep -v '\*' | grep -v '#' | cut -d ':' -f 3 | cut -d "'" -f 1`
  chan=`asterisk -rx 'group show channels' | awk '{ if ($3 ~ "^(O|I|B|C)$") print $2 }'`
  for d in ${dev}; do
    echo "${chan}" | grep "${d}"
    if [ $? -ne 0 ]; then
      asterisk -rx "devstate change Custom:${d} NOT_INUSE"
      d=`echo -n ${d} | sed 's/+/*/'`
      asterisk -rx "devstate change Custom:${d} NOT_INUSE"
    fi
  done

  dev=`asterisk -rx 'devstate list' | grep -v 'NOT_INUSE' | grep 'Custom:' | grep 'Q\*' | cut -d ':' -f 3 | cut -d "'" -f 1`
  chan=`asterisk -rx 'group show channels' | awk '{ if ($3 ~ "^(O|I|B|C)$") print $2 }'`
  for d in ${dev}; do
    dd=`echo -n $d | cut -d '*' -f 2`
    echo "dd=${dd}"
    echo "${chan}" | grep "${dd}"
    if [ $? -ne 0 ]; then
      asterisk -rx "devstate change Custom:${d} NOT_INUSE"
    fi
  done

  for (( i=0; i<3; i++ ))
  do
    sleep 1
  done
done
