!#bin/bash

echo " ______     ______     __     __  __     ______     ______   __         ______     __     __    ";
echo "/\  ___\   /\  ___\   /\ \   /\ \/ /    /\  ___\   /\  ___\ /\ \       /\  __ \   /\ \  _ \ \   ";
echo "\ \___  \  \ \ \____  \ \ \  \ \  _\"-.  \ \___  \  \ \  __\ \ \ \____  \ \ \/\ \  \ \ \/ \".\ \  ";
echo " \/\_____\  \ \_____\  \ \_\  \ \_\ \_\  \/\_____\  \ \_\    \ \_____\  \ \_____\  \ \__/\".~\_\ ";
echo "  \/_____/   \/_____/   \/_/   \/_/\/_/   \/_____/   \/_/     \/_____/   \/_____/   \/_/   \/_/ ";
echo "                                                                                                ";
echo ' ______     ______     ______     ______     ______     ______     ______     ______     ______'
echo 'scik8sflow - a workflow tools for run activities in k8s cluster'
echo '> scik8sflow - preactivity'
echo ' ______     ______     ______     ______     ______     ______     ______     ______     ______'

echo 'ENVIRONMENT VARIABLES'
echo "SCIK8SFLOW_API_URL=$SCIK8SFLOW_API_URL"
echo "ACTIVITY_ID=$ACTIVITY_ID"
echo "OUTPUT_DIR=$OUTPUT_DIR"
echo "WORKFLOW_ID=$WORKFLOW_ID"
echo "MOUNT_PATH=$MOUNT_PATH"

#list all mounted volumes
echo ">> scik8sflow - preactivity - list all mounted volumes"

echo "> scik8sflow - PREACTIVITY - #ACTIVITY_ID=$ACTIVITY_ID" > preactivity.log

mount | grep "^/dev" | awk '{print $3}' | grep $MOUNT_PATH | while read line; do

  if [ "$line" = "$OUTPUT_DIR" ]; then
    echo ">> scik8sflow - preactivity (OUTPUT_DIR) -  $line"
  fi

  if [ "$line" != "$OUTPUT_DIR" ]; then
      echo ">> scik8sflow - preactivity -  $line"
  fi

  # list all files with name, size, date and checksum
  echo ">> scik8sflow - preactivity - list all files with name, size, date and checksum"

  echo ">> scik8sflow - preactivity -  $line" >> preactivity.log
  echo "" >> preactivity.log


  find $line -type f -exec ls -lah {} \; -exec md5sum {} \; >> preactivity.log

  rsync --progress -avzcr $line $OUTPUT_DIR

done

curl -X POST -H "Content-Type: application/txt" -d @preactivity.log $SCIK8SFLOW_API_URL/activities/$ACTIVITY_ID/preactivity || true


echo ">> scik8sflow - preactivity - done"



