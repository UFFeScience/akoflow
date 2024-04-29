FROM alpine:3.12

WORKDIR /preactivity
RUN apk add --no-cache rsync

COPY preactivity.sh .

RUN chmod +x ./preactivity.sh

ENTRYPOINT ["sh", "./preactivity.sh"]