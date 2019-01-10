FROM docker.elastic.co/beats/filebeat-oss:6.5.4

COPY filebeat.yml /usr/share/filebeat/filebeat.yml

USER root

RUN mkdir -p /var/lib/docker/containers
RUN chown -R root /var/lib/docker/containers/
RUN chmod -R go-w /var/lib/docker/containers/

ENTRYPOINT  [ "/usr/local/bin/docker-entrypoint" ]
CMD ["-e"]