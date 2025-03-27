#!/bin/bash



PARENT_PID=##PARENT_PID##
WORKFLOW_ID=##WORKFLOW_ID##
WORKFLOW_ACTIVITY_ID=##WORKFLOW_ACTIVITY_ID##
WORKFLOW_PATH_DATA_DIR=##WORKFLOW_PATH_DATA_DIR##


if [ -z "$PARENT_PID" ]; then
    echo "PARENT_PID is required"
    exit 0
fi


get_children() {
    local pids=("$@")
    local children=()
    
    for pid in "${pids[@]}"; do
        local new_children
        new_children=$(pgrep -P "$pid")

        if [ ! -z "$new_children" ]; then
            children+=($new_children)
            children+=($(get_children $new_children))
        fi
    done

    echo "${children[@]}"
}

etime_to_seconds() {
    local etime=$1
    local seconds=0

    if [[ $etime =~ ([0-9]+)-([0-9]{2}):([0-9]{2}):([0-9]{2}) ]]; then
        seconds=$(( (${BASH_REMATCH[1]} * 86400) + (${BASH_REMATCH[2]} * 3600) + (${BASH_REMATCH[3]} * 60) + ${BASH_REMATCH[4]} ))
    elif [[ $etime =~ ([0-9]{2}):([0-9]{2}):([0-9]{2}) ]]; then
        seconds=$(( (${BASH_REMATCH[1]} * 3600) + (${BASH_REMATCH[2]} * 60) + ${BASH_REMATCH[3]} ))
    elif [[ $etime =~ ([0-9]{2}):([0-9]{2}) ]]; then
        seconds=$(( (${BASH_REMATCH[1]} * 60) + ${BASH_REMATCH[2]} ))
    elif [[ $etime =~ ([0-9]+) ]]; then
        seconds=${BASH_REMATCH[1]}
    fi

    echo $seconds
}

CHILDREN_PIDS=$(get_children $PARENT_PID)

if [ -z "$CHILDREN_PIDS" ]; then
    echo "Anyone child process found for PID=($PARENT_PID). #NO_PROCESS_FOUND"
    exit 0
fi

CHILDREN_PIDS=$(echo "$CHILDREN_PIDS" | tr '\n' ' ')
CHILDENS_COUNT=$(echo "$CHILDREN_PIDS" | wc -w)
CHILDENS_PIDS_IMPLODED=$(echo "$CHILDREN_PIDS" | tr ' ' ',')

total_cpu=0
total_mem=0


while read -r pid user cpu mem command etime; do
    if [[ "$pid" == "PID" ]]; then
        continue
    fi

    seconds=$(etime_to_seconds "$etime")

    cpu=$(echo "$cpu" | tr ',' '.')
    mem=$(echo "$mem" | tr ',' '.')

    if [[ "$cpu" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
        total_cpu=$(echo "$total_cpu + $cpu" | bc -l)
    fi

    if [[ "$mem" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
        total_mem=$(echo "$total_mem + $mem" | bc -l)
    fi

    echo "PID=($pid); USER=($user); CPU=($cpu); MEM=($mem); ETIME=($etime); SECONDS=($seconds); COMMAND=($command)"

done < <(echo "$CHILDREN_PIDS" | xargs ps -o pid,user,%cpu,%mem,comm,etime --no-headers -p)


total_cpu=$(echo "$total_cpu" | awk '{printf "%.2f", ($0+0)}')
total_mem=$(echo "$total_mem" | awk '{printf "%.2f", ($0+0)}')

echo "---------------------------"
echo "TOTAL_CPU=(${total_cpu}%)"
echo "TOTAL_MEM=(${total_mem}%)"
echo "PIDS=($CHILDENS_PIDS_IMPLODED)"
echo "PIDS_COUNT=($CHILDENS_COUNT)"

echo "---------------------------"


echo "##START_LOG_ERROR##"
tail -n 20 $WORKFLOW_PATH_DATA_DIR/akoflow_err$WORKFLOW_ID\_$WORKFLOW_ACTIVITY_ID.err
echo "##END_LOG_ERROR##"

echo "##START_LOG_OUTPUT##"
tail -n 20 $WORKFLOW_PATH_DATA_DIR/akoflow_out$WORKFLOW_ID\_$WORKFLOW_ACTIVITY_ID.out
echo "##END_LOG_OUTPUT##"