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
echo mount | grep "^/dev" | awk '{print $3}' | grep $MOUNT_PATH
echo ' ______     ______     ______     ______     ______     ______     ______     ______     ______'

echo ">> akoflow - preactivity - copy files from mounted volumes to OUTPUT_DIR"

mount | grep "^/dev" | awk '{print $3}' | grep $MOUNT_PATH | while read line; do

  if [ "$line" = "$OUTPUT_DIR" ]; then
    echo ">> akoflow - preactivity (OUTPUT_DIR) -  $line"
  fi

  if [ "$line" != "$OUTPUT_DIR" ]; then
      echo ">> akoflow - preactivity -  $line"
  fi

  rsync --progress -avr $line/ $OUTPUT_DIR

done

echo ">> akoflow - preactivity - done"