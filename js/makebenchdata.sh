#!/bin/sh

n="$1"
out="/$TMPDIR/makebenchdataouttmpfile"

rm -f $out

i=0
while [ $i -le $n ]; do
  echo "foo${i} /${i}foo" >> $out
  i=$(( i + 1 ))
done

cd ..
go run main.go -input $out
