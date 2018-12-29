#!/bin/bash
umask 177

f=`find /pbx/conf/NULL/ -mindepth 1 -maxdepth 1 -type f | head -n 1`
[ "${f}" != "" ] && card "${f}" ! !

find /pbx/conf/ -mindepth 1 -maxdepth 1 -type f -exec card "{}" ! ! \;

dir=(`find /pbx/conf/ -mindepth 1 -maxdepth 1 -type d -not -name 'NULL'`)

for d in "${dir[@]}"; do
  f=`find "${d}" -mindepth 1 -maxdepth 1 -type f | head -n 1`
  [ "${f}" != "" ] && card "${f}" ! !
done

find /pbx/conf/REDIRECT/ -mindepth 1 -maxdepth 1 -type f -exec card "{}" ! ! \;
