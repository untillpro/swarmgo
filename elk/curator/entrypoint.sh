#!/bin/sh

CRON=${CRON:-"*/15 * * * *"}
KEEP_DAYS=${KEEP_DAYS:-"15"}
INDEX_PREFIX=${INDEX_PREFIX:-"filebeat-"}
HOST=${HOST:-"elasticsearch"}
CHRONO_UNIT=${CHRONO_UNIT:-"days"}

echo "$CRON /usr/bin/curator_cli --host $HOST delete_indices --ignore_empty_list --filter_list '[{\"filtertype\":\"age\",\"source\":\"name\",\"direction\":\"older\",\"unit\":\"${CHRONO_UNIT}\",\"unit_count\":${KEEP_DAYS},\"timestring\":\"%Y.%m.%d\"},{\"filtertype\":\"pattern\",\"kind\":\"prefix\",\"value\":\"${INDEX_PREFIX}\"}]'" >> /var/spool/cron/crontabs/root

exec crond -f
