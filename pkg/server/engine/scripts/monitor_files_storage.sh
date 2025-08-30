ls -lR $ACTIVITY_MOUNT_PATH > /tmp/du_output.txt;
echo "Preparing to start request";
body=$(cat /tmp/du_output.txt);
body_length=$(printf %s "$body" | wc -c);

echo "Start request";
{ 
    echo -ne "POST /akoflow-server/internal/storage/` + path + `/?activityId=$ACTIVITY_ID HTTP/1.1\r\n";
    echo -ne "Host: $AKOFLOW_SERVER_SERVICE_SERVICE_HOST\r\n";
    echo -ne "Content-Type: text/plain\r\n";
    echo -ne "Content-Length: $body_length\r\n";
    echo -ne "Connection: close\r\n";
    echo -ne "\r\n";
    echo -ne "$body";
} | nc $AKOFLOW_SERVER_SERVICE_SERVICE_HOST ` + port + `;

echo "End request";
