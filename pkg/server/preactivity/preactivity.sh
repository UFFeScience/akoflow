!#bin/bash

echo " █████╗ ██╗  ██╗ ██████╗ ███████╗██╗      ██████╗ ██╗    ██╗";
echo "██╔══██╗██║ ██╔╝██╔═══██╗██╔════╝██║     ██╔═══██╗██║    ██║";
echo "███████║█████╔╝ ██║   ██║█████╗  ██║     ██║   ██║██║ █╗ ██║";
echo "██╔══██║██╔═██╗ ██║   ██║██╔══╝  ██║     ██║   ██║██║███╗██║";
echo "██║  ██║██║  ██╗╚██████╔╝██║     ███████╗╚██████╔╝╚███╔███╔╝";
echo "╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚══════╝ ╚═════╝  ╚══╝╚══╝ ";
echo ' ______     ______     ______     ______     ______     ______     ______     ______     ______'
echo 'akoflow - a workflow tools for run activities in k8s cluster'
echo '> akoflow - preactivity'
echo ' ______     ______     ______     ______     ______     ______     ______     ______     ______'

echo 'ENVIRONMENT VARIABLES'
echo "AKOFLOW_API_URL=$AKOFLOW_API_URL"
echo "ACTIVITY_ID=$ACTIVITY_ID"
echo "OUTPUT_DIR=$OUTPUT_DIR"
echo "WORKFLOW_ID=$WORKFLOW_ID"
echo "MOUNT_PATH=$MOUNT_PATH"

#list all mounted volumes
echo ">> akoflow - preactivity - list all mounted volumes"

echo "> akoflow - PREACTIVITY - #ACTIVITY_ID=$ACTIVITY_ID" > preactivity.log

mount | grep "^/dev" | awk '{print $3}' | grep $MOUNT_PATH | while read line; do

  if [ "$line" = "$OUTPUT_DIR" ]; then
    echo ">> akoflow - preactivity (OUTPUT_DIR) -  $line"
  fi

  if [ "$line" != "$OUTPUT_DIR" ]; then
      echo ">> akoflow - preactivity -  $line"
  fi

  # list all files with name, size, date and checksum
  echo ">> akoflow - preactivity - list all files with name, size, date and checksum"

  echo ">> akoflow - preactivity -  $line" >> $line/preactivity.log
  echo "" >> $line/preactivity.log


  find $line -type f -exec ls -lah {} \; -exec md5sum {} \; >> $line/preactivity.log

  rsync --progress -avr $line/ $OUTPUT_DIR

done

curl -X POST -H "Content-Type: application/txt" -d @$OUTPUT_DIR/preactivity.log $akoflow_API_URL/activities/$ACTIVITY_ID/preactivity || true

cat $OUTPUT_DIR/preactivity.log || true

echo ">> akoflow - preactivity - done"



