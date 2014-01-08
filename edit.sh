#!/bin/bash
set -e
set -x
URL=$1
ATTRIB=$2
[ -z "$EDITOR" ] && EDITOR=vim
if [ -z "$ATTRIB" ]
then
	echo "$0 <collins url w/ user+pass to asset> <attribute>"
	exit 1
fi

collins() {
  CTMP=`mktemp`
	curl -s -o "$CTMP" -H 'Accept: text/x-shellscript' "$@"
	. "$CTMP"
	# rm "$CTMP"
}

collins $URL
if [ "$status" == "error" ]
then
	echo "Error getting asset: $data_message"
	exit 1
fi

VAR="ATTRIBS_0_$ATTRIB"
TMP=`mktemp`
TMP_NEW=`mktemp`
echo "${!VAR}" > "$TMP"
sed -i 's/;/\;/g;s/\\/\\\\/' "$TMP"

cp "$TMP" "$TMP_NEW"
$EDITOR "$TMP_NEW"
if ! cmp -s "$TMP" "$TMP_NEW"
then
	echo "Updating attribute"
	collins --data-urlencode "attribute=$ATTRIB;`cat $TMP_NEW`" $URL
	if [ "$status" == "error" ]
	then
		echo "Error updating attribute: $data_message"
		ERNO=1
	fi
else
	echo "Not changed"
fi
# rm "$TMP" "$TMP_NEW"
exit $ERNO

