#FROM centurylink/ca-certs
FROM ubuntu:14.04
MAINTAINER CenturyLink Labs <innovationslab@ctl.io>
LABEL "com.centurylinklabs.watchtower"="true"

COPY watchtower /
ENTRYPOINT ["/watchtower"]
