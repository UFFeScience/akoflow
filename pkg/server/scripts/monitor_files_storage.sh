PATH_PARAM="#PATH_PARAM#"
PORT="#PORT#"

ls -lR $ACTIVITY_MOUNT_PATH > /tmp/du_output.txt
echo "Preparing to start request"
body=$(cat /tmp/du_output.txt)
body_length=$(printf %s "$body" | wc -c)
echo "Start request"

if command -v nc >/dev/null 2>&1; then
    { echo -ne "POST /akoflow-server/internal/storage/$PATH_PARAM/?activityId=$ACTIVITY_ID HTTP/1.1\r\n"; echo -ne "Host: $AKOFLOW_SERVER_SERVICE_SERVICE_HOST\r\n"; echo -ne "Content-Type: text/plain\r\n"; echo -ne "Content-Length: $body_length\r\n"; echo -ne "Connection: close\r\n"; echo -ne "\r\n"; echo -ne "$body"; } | nc $AKOFLOW_SERVER_SERVICE_SERVICE_HOST $PORT
elif command -v curl >/dev/null 2>&1; then
    curl -X POST "http://$AKOFLOW_SERVER_SERVICE_SERVICE_HOST:$PORT/akoflow-server/internal/storage/$PATH_PARAM/?activityId=$ACTIVITY_ID" -H "Content-Type: text/plain" --data "$body"
elif command -v wget >/dev/null 2>&1; then
    wget --post-data="$body" --header="Content-Type: text/plain" "http://$AKOFLOW_SERVER_SERVICE_SERVICE_HOST:$PORT/akoflow-server/internal/storage/$PATH_PARAM/?activityId=$ACTIVITY_ID" -O /dev/null
elif [ "$BASH_VERSION" ] && [ -e /dev/tcp ]; then
    exec 3<>/dev/tcp/$AKOFLOW_SERVER_SERVICE_SERVICE_HOST/$PORT
    echo -ne "POST /akoflow-server/internal/storage/$PATH_PARAM/?activityId=$ACTIVITY_ID HTTP/1.1\r\nHost: $AKOFLOW_SERVER_SERVICE_SERVICE_HOST\r\nContent-Type: text/plain\r\nContent-Length: $body_length\r\nConnection: close\r\n\r\n$body" >&3
else
    echo "No HTTP client available (nc, curl, wget or bash)" >&2
fi

echo -e "\nEnd request"